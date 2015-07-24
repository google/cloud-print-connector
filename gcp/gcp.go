/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Package gcp is the Google Cloud Print API client.
package gcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"
)

const (
	// This prefix tickles a magic spell in GCP so that, for example,
	// the GCP UI shows location as the string found in the
	// printer-location CUPS attribute.
	gcpTagPrefix = "__cp__"

	// OAuth constants.
	RedirectURL     = "oob"
	ScopeCloudPrint = "https://www.googleapis.com/auth/cloudprint"
	ScopeGoogleTalk = "https://www.googleapis.com/auth/googletalk"
	AccessType      = "offline"
)

// GoogleCloudPrint is the interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	baseURL                 string
	robotClient             *http.Client
	userClient              *http.Client
	proxyName               string
	xmppPingIntervalDefault time.Duration
}

// NewGoogleCloudPrint establishes a connection with GCP, returns a new GoogleCloudPrint object.
func NewGoogleCloudPrint(baseURL, robotRefreshToken, userRefreshToken, proxyName, oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL string, xmppPingIntervalDefault time.Duration) (*GoogleCloudPrint, error) {
	robotClient, err := newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, robotRefreshToken, ScopeCloudPrint, ScopeGoogleTalk)
	if err != nil {
		return nil, err
	}

	var userClient *http.Client
	if userRefreshToken != "" {
		userClient, err = newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, userRefreshToken, ScopeCloudPrint)
		if err != nil {
			return nil, err
		}
	}

	gcp := &GoogleCloudPrint{
		baseURL:                 baseURL,
		robotClient:             robotClient,
		userClient:              userClient,
		proxyName:               proxyName,
		xmppPingIntervalDefault: xmppPingIntervalDefault,
	}

	return gcp, nil
}

func (gcp *GoogleCloudPrint) GetRobotAccessToken() (string, error) {
	token, err := gcp.robotClient.Transport.(*oauth2.Transport).Source.Token()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// CanShare answers the question "can we share printers when they are registered?"
func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userClient != nil
}

// Control calls google.com/cloudprint/control to set the state of a
// GCP print job.
func (gcp *GoogleCloudPrint) Control(jobID string, state cdd.PrintJobStateDiff) error {
	semanticState, err := json.Marshal(state)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("semantic_state_diff", string(semanticState))

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"control", form); err != nil {
		return err
	}

	return nil
}

// Delete calls google.com/cloudprint/delete to delete a printer from GCP.
func (gcp *GoogleCloudPrint) Delete(gcpID string) error {
	form := url.Values{}
	form.Set("printerid", gcpID)

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"delete", form); err != nil {
		return err
	}

	return nil
}

// Fetch calls google.com/cloudprint/fetch to get the outstanding print jobs for
// a GCP printer.
func (gcp *GoogleCloudPrint) Fetch(gcpID string) ([]Job, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, errorCode, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"fetch", form)
	if err != nil {
		if errorCode == 413 {
			// 413 means "Zero print jobs returned", which isn't really an error.
			return []Job{}, nil
		}
		return nil, err
	}

	var jobsData struct {
		Jobs []struct {
			ID      string
			Title   string
			FileURL string
			OwnerID string
		}
	}
	if err = json.Unmarshal(responseBody, &jobsData); err != nil {
		return nil, err
	}

	jobs := make([]Job, len(jobsData.Jobs))

	for i, jobData := range jobsData.Jobs {
		jobs[i] = Job{
			GCPPrinterID: gcpID,
			GCPJobID:     jobData.ID,
			FileURL:      jobData.FileURL,
			OwnerID:      jobData.OwnerID,
			Title:        jobData.Title,
		}
	}

	return jobs, nil
}

// List calls google.com/cloudprint/list to get all GCP printers assigned
// to this connector.
//
// Returns map of GCPID => printer name. GCPID is unique to GCP; printer name
// should be unique to CUPS. Use Printer to get details about each printer.
func (gcp *GoogleCloudPrint) List() (map[string]string, error) {
	form := url.Values{}
	form.Set("proxy", gcp.proxyName)
	form.Set("extra_fields", "-tags")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"list", form)
	if err != nil {
		return nil, err
	}

	var listData struct {
		Printers []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
	}
	if err = json.Unmarshal(responseBody, &listData); err != nil {
		return nil, err
	}

	printers := make(map[string]string, len(listData.Printers))
	for _, p := range listData.Printers {
		printers[p.ID] = p.Name
	}

	return printers, nil
}

// Register calls google.com/cloudprint/register to register a GCP printer.
//
// Sets the GCPID field in the printer arg.
func (gcp *GoogleCloudPrint) Register(printer *lib.Printer) error {
	capabilities, err := marshalCapabilities(printer.Description)
	if err != nil {
		return err
	}

	semanticState, err := json.Marshal(cdd.CloudDeviceState{Printer: printer.State})
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("name", printer.Name)
	form.Set("default_display_name", printer.DefaultDisplayName)
	form.Set("proxy", gcp.proxyName)
	form.Set("uuid", printer.UUID)
	form.Set("manufacturer", printer.Manufacturer)
	form.Set("model", printer.Model)
	form.Set("gcp_version", printer.GCPVersion)
	form.Set("setup_url", printer.SetupURL)
	form.Set("support_url", printer.SupportURL)
	form.Set("update_url", printer.UpdateURL)
	form.Set("firmware", printer.ConnectorVersion)
	form.Set("semantic_state", string(semanticState))
	form.Set("use_cdd", "true")
	form.Set("capabilities", capabilities)
	form.Set("capsHash", printer.CapsHash)

	sortedKeys := make([]string, 0, len(printer.Tags))
	for key := range printer.Tags {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		form.Add("tag", fmt.Sprintf("%s%s=%s", gcpTagPrefix, key, printer.Tags[key]))
	}

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"register", form)
	if err != nil {
		return err
	}

	var registerData struct {
		Printers []struct {
			ID string
		}
	}
	if err = json.Unmarshal(responseBody, &registerData); err != nil {
		return err
	}

	printer.GCPID = registerData.Printers[0].ID

	return nil
}

// Update calls google.com/cloudprint/update to update a GCP printer.
func (gcp *GoogleCloudPrint) Update(diff *lib.PrinterDiff) error {
	// Ignores Name field because it never changes.

	form := url.Values{}
	form.Set("printerid", diff.Printer.GCPID)
	form.Set("proxy", gcp.proxyName)

	if diff.DefaultDisplayNameChanged {
		form.Set("default_display_name", diff.Printer.DefaultDisplayName)
	}
	if diff.ManufacturerChanged {
		form.Set("manufacturer", diff.Printer.Manufacturer)
	}
	if diff.ModelChanged {
		form.Set("model", diff.Printer.Model)
	}
	if diff.GCPVersionChanged {
		form.Set("gcp_version", diff.Printer.GCPVersion)
	}
	if diff.SetupURLChanged {
		form.Set("setup_url", diff.Printer.SetupURL)
	}
	if diff.SupportURLChanged {
		form.Set("support_url", diff.Printer.SupportURL)
	}
	if diff.UpdateURLChanged {
		form.Set("update_url", diff.Printer.UpdateURL)
	}
	if diff.ConnectorVersionChanged {
		form.Set("firmware", diff.Printer.ConnectorVersion)
	}

	if diff.StateChanged || diff.DescriptionChanged || diff.GCPVersionChanged {
		semanticState, err := json.Marshal(cdd.CloudDeviceState{Printer: diff.Printer.State})
		if err != nil {
			return err
		}
		form.Set("semantic_state", string(semanticState))
	}

	if diff.CapsHashChanged || diff.DescriptionChanged || diff.GCPVersionChanged {
		capabilities, err := marshalCapabilities(diff.Printer.Description)
		if err != nil {
			return err
		}

		form.Set("use_cdd", "true")
		form.Set("capabilities", capabilities)
		form.Set("capsHash", diff.Printer.CapsHash)
	}

	if diff.TagsChanged {
		sortedKeys := make([]string, 0, len(diff.Printer.Tags))
		for key := range diff.Printer.Tags {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)
		for _, key := range sortedKeys {
			form.Add("tag", fmt.Sprintf("%s%s=%s", gcpTagPrefix, key, diff.Printer.Tags[key]))
		}

		form.Set("remove_tag", gcpTagPrefix+".*")
	}

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"update", form); err != nil {
		return err
	}

	return nil
}

// Printer gets the printer identified by it's GCPID.
//
// The second return value is queued print job quantity.
func (gcp *GoogleCloudPrint) Printer(gcpID string) (*lib.Printer, uint, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("use_cdd", "true")
	form.Set("extra_fields", "queuedJobsCount,semanticState")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"printer", form)
	if err != nil {
		return nil, 0, err
	}

	var printersData struct {
		Printers []struct {
			ID                 string                     `json:"id"`
			Name               string                     `json:"name"`
			DefaultDisplayName string                     `json:"defaultDisplayName"`
			UUID               string                     `json:"uuid"`
			Manufacturer       string                     `json:"manufacturer"`
			Model              string                     `json:"model"`
			GCPVersion         string                     `json:"gcpVersion"`
			SetupURL           string                     `json:"setupUrl"`
			SupportURL         string                     `json:"supportUrl"`
			UpdateURL          string                     `json:"updateUrl"`
			Firmware           string                     `json:"firmware"`
			Capabilities       cdd.CloudDeviceDescription `json:"capabilities"`
			CapsHash           string                     `json:"capsHash"`
			Tags               []string                   `json:"tags"`
			QueuedJobsCount    uint                       `json:"queuedJobsCount"`
			SemanticState      cdd.CloudDeviceState       `json:"semanticState"`
		}
	}
	if err = json.Unmarshal(responseBody, &printersData); err != nil {
		return nil, 0, err
	}

	p := printersData.Printers[0] // If the slice were empty, postWithRetry would have returned an error.

	tags := make(map[string]string)
	for _, tag := range p.Tags {
		s := strings.SplitN(tag[len(gcpTagPrefix):], "=", 2)
		key := s[0]
		var value string
		if len(s) > 1 {
			value = s[1]
		}
		tags[key] = value
	}

	printer := &lib.Printer{
		GCPID:              p.ID,
		Name:               p.Name,
		DefaultDisplayName: p.DefaultDisplayName,
		UUID:               p.UUID,
		Manufacturer:       p.Manufacturer,
		Model:              p.Model,
		GCPVersion:         p.GCPVersion,
		SetupURL:           p.SetupURL,
		SupportURL:         p.SupportURL,
		UpdateURL:          p.UpdateURL,
		ConnectorVersion:   p.Firmware,
		State:              p.SemanticState.Printer,
		Description:        p.Capabilities.Printer,
		CapsHash:           p.CapsHash,
		Tags:               tags,
	}

	return printer, p.QueuedJobsCount, err
}

// Translate calls google.com/cloudprint/tools/cdd/translate to translate
// a PPD string to cdd.PrinterDescriptionSection.
func (gcp *GoogleCloudPrint) Translate(ppd string) (*cdd.PrinterDescriptionSection, error) {
	form := url.Values{}
	form.Set("capabilities", ppd)

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"tools/cdd/translate", form)
	if err != nil {
		return nil, err
	}

	d := json.NewDecoder(bytes.NewReader(responseBody))
	d.UseNumber() // Force large numbers not to be formatted with scientific notation.

	var response struct {
		CDD cdd.CloudDeviceDescription `json:"cdd"`
	}
	if err = d.Decode(&response); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal translated CDD: %s", err)
	}

	return response.CDD.Printer, nil
}

func marshalCapabilities(description *cdd.PrinterDescriptionSection) (string, error) {
	capabilities := cdd.CloudDeviceDescription{
		Version: "1.0",
		Printer: description,
	}

	cdd, err := json.Marshal(capabilities)
	if err != nil {
		return "", fmt.Errorf("Failed to remarshal translated CDD: %s", err)
	}

	return string(cdd), nil
}

// Share calls google.com/cloudprint/share to share a registered GCP printer.
func (gcp *GoogleCloudPrint) Share(gcpID, shareScope string) error {
	if gcp.userClient == nil {
		return errors.New("Cannot share because user OAuth credentials not provided.")
	}

	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("scope", shareScope)
	form.Set("role", "USER")
	form.Set("skip_notification", "true")

	if _, _, _, err := postWithRetry(gcp.userClient, gcp.baseURL+"share", form); err != nil {
		return err
	}

	return nil
}

// Download downloads a URL (a print job data file) directly to a Writer.
func (gcp *GoogleCloudPrint) Download(dst io.Writer, url string) error {
	response, err := getWithRetry(gcp.robotClient, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	_, err = io.Copy(dst, response.Body)
	if err != nil {
		return err
	}

	return nil
}

// Ticket gets a ticket, aka print job options.
func (gcp *GoogleCloudPrint) Ticket(gcpJobID string) (*cdd.CloudJobTicket, error) {
	form := url.Values{}
	form.Set("jobid", gcpJobID)
	form.Set("use_cjt", "true")

	responseBody, _, httpStatusCode, err := postWithRetry(gcp.robotClient, gcp.baseURL+"ticket", form)
	// The /ticket API is different than others, because it only returns the
	// standard GCP error information on success=false.
	if httpStatusCode != 200 {
		return nil, err
	}

	d := json.NewDecoder(bytes.NewReader(responseBody))
	d.UseNumber() // Force large numbers not to be formatted with scientific notation.

	var ticket cdd.CloudJobTicket
	err = d.Decode(&ticket)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal ticket: %s", err)
	}

	return &ticket, nil
}

// ProximityToken gets a proximity token for Privet users to access a printer
// through the cloud.
//
// Returned byte array is marshalled JSON to preserve any/all returned fields.
func (gcp *GoogleCloudPrint) ProximityToken(gcpID, user string) ([]byte, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("user", user)

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"proximitytoken", form)
	if err != nil {
		return nil, err
	}

	var response struct {
		ProximityToken map[string]interface{} `json:"proximity_token"`
	}
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	token, err := json.MarshalIndent(response.ProximityToken, "", "  ")
	if err != nil {
		return nil, err
	}

	return token, nil
}
