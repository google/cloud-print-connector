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

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
)

const (
	// This prefix tickles a magic spell in GCP so that, for example,
	// the GCP UI shows location as the string found in the
	// printer-location CUPS attribute.
	gcpTagPrefix = "__cp__"

	// XMPP connections fail. Attempt to reconnect a few times before giving up.
	restartXMPPMaxRetries = 4

	// Stop retrying when two failures occur in a short period of time.
	minTimeBetweenXMPPFailures = time.Second * 3

	// OAuth constants.
	RedirectURL     = "oob"
	ScopeCloudPrint = "https://www.googleapis.com/auth/cloudprint"
	ScopeGoogleTalk = "https://www.googleapis.com/auth/googletalk"
	AccessType      = "offline"
)

// GoogleCloudPrint is the interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	baseURL     string
	xmppJID     string
	xmpp        *XMPP
	robotClient *http.Client
	userClient  *http.Client
	proxyName   string

	xmppServer              string
	xmppPort                uint16
	xmppPingTimeout         time.Duration
	xmppPingIntervalDefault time.Duration

	xmppPrintersJobs        chan string
	xmppPrintersUpdates     chan string
	xmppPingIntervalUpdates chan time.Duration
	xmppDead                chan struct{}

	quit chan struct{}
}

// NewGoogleCloudPrint establishes a connection with GCP, returns a new GoogleCloudPrint object.
func NewGoogleCloudPrint(baseURL, xmppJID, robotRefreshToken, userRefreshToken, proxyName, oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, xmppServer string, xmppPort uint16, xmppPingTimeout, xmppPingIntervalDefault time.Duration) (*GoogleCloudPrint, error) {
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
		xmppJID:                 xmppJID,
		xmpp:                    nil,
		robotClient:             robotClient,
		userClient:              userClient,
		proxyName:               proxyName,
		xmppServer:              xmppServer,
		xmppPort:                xmppPort,
		xmppPingTimeout:         xmppPingTimeout,
		xmppPingIntervalDefault: xmppPingIntervalDefault,
		xmppPrintersJobs:        make(chan string, 10),
		xmppPrintersUpdates:     make(chan string, 10),
		xmppPingIntervalUpdates: make(chan time.Duration, 1),
		xmppDead:                make(chan struct{}),
		quit:                    make(chan struct{}),
	}

	return gcp, nil
}

type localSettingsSettings struct {
	XMPPPingInterval uint32 `json:"xmpp_timeout_value"`
	// Other values can be set here; omit until needed.
}

// localSettingsPush is for Register and Update.
// Using the pending field is not permitted from device accounts.
type localSettingsPush struct {
	Current localSettingsSettings `json:"current"`
}

// localSettingsPull is for List.
type localSettingsPull struct {
	Current localSettingsSettings `json:"current"`
	Pending localSettingsSettings `json:"pending"`
}

// Turn arguments into a JSON-encoded GCP LocalSettings message.
func marshalLocalSettings(xmppInterval time.Duration) (string, error) {
	var ls localSettingsPush
	ls.Current.XMPPPingInterval = uint32(xmppInterval.Seconds())
	lss, err := json.Marshal(ls)
	if err != nil {
		return "", err
	}
	return string(lss), nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (gcp *GoogleCloudPrint) Quit() {
	if gcp.xmpp != nil {
		// Signal to KeepXMPPAlive.
		gcp.quit <- struct{}{}
		select {
		case <-gcp.xmppDead:
			// Wait for XMPP to die.
		case <-time.After(5 * time.Second):
			// But not too long.
			glog.Error("XMPP taking a while to close, so giving up")
		}
	}
}

// CanShare answers the question "can we share printers when they are registered?"
func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userClient != nil
}

// StartXMPP tries to start an XMPP conversation.
// Tries multiple times before returning an error.
func (gcp *GoogleCloudPrint) StartXMPP() error {
	if gcp.xmpp != nil {
		go gcp.xmpp.Quit()
	}

	var err error
	for i := 0; i < restartXMPPMaxRetries; i++ {
		// The current access token is the XMPP password.
		var token *oauth2.Token
		token, err = gcp.robotClient.Transport.(*oauth2.Transport).Source.Token()
		if err == nil {
			var xmpp *XMPP
			xmpp, err = NewXMPP(gcp.xmppJID, token.AccessToken, gcp.proxyName,
				gcp.xmppServer, gcp.xmppPort, gcp.xmppPingTimeout, gcp.xmppPingIntervalDefault,
				gcp.xmppPrintersJobs, gcp.xmppPrintersUpdates, gcp.xmppPingIntervalUpdates, gcp.xmppDead)

			if err == nil {
				// Success!
				gcp.xmpp = xmpp
				// Don't give up.
				go gcp.keepXMPPAlive()
				return nil
			}
		}

		// Sleep for 1, 2, 4, 8 seconds.
		time.Sleep(time.Duration((i+1)*2) * time.Second)
	}

	return fmt.Errorf("Failed to start XMPP conversation: %s", err)
}

// KeepXMPPAlive restarts XMPP when it fails.
func (gcp *GoogleCloudPrint) keepXMPPAlive() {
	for {
		select {
		case <-gcp.xmppDead:
			glog.Error("XMPP conversation died; restarting")
			if err := gcp.StartXMPP(); err != nil {
				glog.Fatalf("Failed to keep XMPP conversation alive: %s", err)
			}
		case <-gcp.quit:
			// Close XMPP.
			gcp.xmpp.Quit()
			return
		}
	}
}

// NextJobBatch gets the next batch of print jobs from GCP. Blocks on XMPP until
// batch notification arrives. Calls Fetch to get the jobs.
func (gcp *GoogleCloudPrint) NextJobBatch() ([]lib.Job, error) {
	gcpID := <-gcp.xmppPrintersJobs
	return gcp.Fetch(gcpID)
}

// NextPrinterWithUpdates gets the GCPID of the next printer with updates
// in the pending state.
func (gcp *GoogleCloudPrint) NextPrinterWithUpdates() string {
	return <-gcp.xmppPrintersUpdates
}

// printJobStateDiff represents a CJS PrintJobStateDiff message.
type printJobStateDiff struct {
	State        jobState `json:"state"`
	PagesPrinted uint32   `json:"pages_printed"`
}

// jobState represents a CJS JobState message.
type jobState struct {
	Type              string             `json:"type"`
	UserActionCause   *userActionCause   `json:"user_action_cause,omitempty"`
	DeviceActionCause *deviceActionCause `json:"device_action_cause,omitempty"`
}

// userActionCause represents a CJS JobState.UserActionCause message.
type userActionCause struct {
	ActionCode string `json:"action_code"`
}

// deviceActionCause represents a CJS JobState.DeviceActionCause message.
type deviceActionCause struct {
	ErrorCode string `json:"error_code"`
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
func (gcp *GoogleCloudPrint) Fetch(gcpID string) ([]lib.Job, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, errorCode, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"fetch", form)
	if err != nil {
		if errorCode == 413 {
			// 413 means "Zero print jobs returned", which isn't really an error.
			return []lib.Job{}, nil
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

	jobs := make([]lib.Job, len(jobsData.Jobs))

	for i, jobData := range jobsData.Jobs {
		jobs[i] = lib.Job{
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

	localSettings, err := marshalLocalSettings(gcp.xmppPingIntervalDefault)
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
	form.Set("setup_url", lib.ConnectorHomeURL)
	form.Set("support_url", lib.ConnectorHomeURL)
	form.Set("update_url", lib.ConnectorHomeURL)
	form.Set("firmware", printer.ConnectorVersion)
	form.Set("local_settings", localSettings)
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

	if diff.XMPPPingIntervalChanged {
		localSettings, err := marshalLocalSettings(diff.Printer.XMPPPingInterval)
		if err != nil {
			return err
		}

		form.Set("local_settings", localSettings)
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

// SetConnectorXMPPPingInterval sets the interval at which this connector will
// ping the XMPP server.
func (gcp *GoogleCloudPrint) SetConnectorXMPPPingInterval(interval time.Duration) {
	gcp.xmppPingIntervalUpdates <- interval
}

// SetPrinterXMPPPingInterval checks GCP for a pending XMPP interval change,
// and applies the change, to one printer.
func (gcp *GoogleCloudPrint) SetPrinterXMPPPingInterval(printer lib.Printer) error {
	diff := lib.PrinterDiff{
		Operation:               lib.UpdatePrinter,
		Printer:                 printer,
		XMPPPingIntervalChanged: true,
	}

	if err := gcp.Update(&diff); err != nil {
		return err
	}

	return nil
}

// Printer gets the printer identified by it's GCPID.
//
// The second return value is queued print job quantity.
// The third return value is pending XMPP ping interval change, or zero if no change is pending.
func (gcp *GoogleCloudPrint) Printer(gcpID string) (*lib.Printer, uint, time.Duration, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("use_cdd", "true")
	form.Set("extra_fields", "queuedJobsCount,semanticState")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"printer", form)
	if err != nil {
		return nil, 0, 0, err
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
			LocalSettings      localSettingsPull          `json:"local_settings"`
			Tags               []string                   `json:"tags"`
			QueuedJobsCount    uint                       `json:"queuedJobsCount"`
			SemanticState      cdd.CloudDeviceState       `json:"semanticState"`
		}
	}
	if err = json.Unmarshal(responseBody, &printersData); err != nil {
		return nil, 0, 0, err
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

	var xmppPingInterval time.Duration
	var xmppPingIntervalPending time.Duration
	if p.LocalSettings.Pending.XMPPPingInterval > 0 {
		xmppPingIntervalSeconds := p.LocalSettings.Pending.XMPPPingInterval
		xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
		xmppPingIntervalPending = xmppPingInterval
	} else if p.LocalSettings.Current.XMPPPingInterval > 0 {
		xmppPingIntervalSeconds := p.LocalSettings.Current.XMPPPingInterval
		xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
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
		XMPPPingInterval:   xmppPingInterval,
	}

	return printer, p.QueuedJobsCount, xmppPingIntervalPending, err
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

// Download downloads a URL (a print job PDF) directly to a Writer.
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
func (gcp *GoogleCloudPrint) Ticket(gcpJobID string) (cdd.CloudJobTicket, error) {
	form := url.Values{}
	form.Set("jobid", gcpJobID)
	form.Set("use_cjt", "true")

	responseBody, _, httpStatusCode, err := postWithRetry(gcp.robotClient, gcp.baseURL+"ticket", form)
	// The /ticket API is different than others, because it only returns the
	// standard GCP error information on success=false.
	if httpStatusCode != 200 {
		return cdd.CloudJobTicket{}, err
	}

	d := json.NewDecoder(bytes.NewReader(responseBody))
	d.UseNumber() // Force large numbers not to be formatted with scientific notation.

	var ticket cdd.CloudJobTicket
	err = d.Decode(&ticket)
	if err != nil {
		return cdd.CloudJobTicket{}, fmt.Errorf("Failed to unmarshal ticket: %s", err)
	}

	return ticket, nil
}
