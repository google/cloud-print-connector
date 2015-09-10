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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/golang/glog"
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

	jobs              chan<- *lib.Job
	downloadSemaphore *lib.Semaphore
}

// NewGoogleCloudPrint establishes a connection with GCP, returns a new GoogleCloudPrint object.
func NewGoogleCloudPrint(baseURL, robotRefreshToken, userRefreshToken, proxyName, oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL string, xmppPingIntervalDefault time.Duration, maxConcurrentDownload uint, jobs chan<- *lib.Job) (*GoogleCloudPrint, error) {
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
		jobs:              jobs,
		downloadSemaphore: lib.NewSemaphore(maxConcurrentDownload),
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
	if httpStatusCode != http.StatusOK {
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
// Returns byte array of raw JSON to preserve any/all returned fields
// and returned HTTP status code.
func (gcp *GoogleCloudPrint) ProximityToken(gcpID, user string) ([]byte, int, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("user", user)

	responseBody, _, httpStatus, err := postWithRetry(gcp.robotClient, gcp.baseURL+"proximitytoken", form)
	return responseBody, httpStatus, err
}

// ListPrinters calls gcp.List, then calls gcp.Printer, one goroutine per
// printer. This is a fast way to fetch all printers with corresponding CDD
// info, which the List API does not provide.
//
// The second return value is a map of GCPID -> queued print job quantity.
func (gcp *GoogleCloudPrint) ListPrinters() ([]lib.Printer, map[string]uint, error) {
	ids, err := gcp.List()
	if err != nil {
		return nil, nil, err
	}

	type response struct {
		printer         *lib.Printer
		queuedJobsCount uint
		err             error
	}
	ch := make(chan response)
	for id := range ids {
		go func(id string) {
			printer, queuedJobsCount, err := gcp.Printer(id)
			ch <- response{printer, queuedJobsCount, err}
		}(id)
	}

	errs := make([]error, 0)
	printers := make([]lib.Printer, 0, len(ids))
	queuedJobsCount := make(map[string]uint)
	for _ = range ids {
		r := <-ch
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		printers = append(printers, *r.printer)
		if r.queuedJobsCount > 0 {
			queuedJobsCount[r.printer.GCPID] = r.queuedJobsCount
		}
	}

	if len(errs) == 0 {
		return printers, queuedJobsCount, nil
	} else if len(errs) == 1 {
		return nil, nil, errs[0]
	} else {
		// Return an error that is somewhat human-readable.
		b := bytes.NewBufferString(fmt.Sprintf("%d errors: ", len(errs)))
		for i, err := range errs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(err.Error())
		}
		return nil, nil, errors.New(b.String())
	}
}

// HandleJobs gets and processes jobs waiting on a printer.
func (gcp *GoogleCloudPrint) HandleJobs(printer *lib.Printer, reportJobFailed func()) {
	jobs, err := gcp.Fetch(printer.GCPID)
	if err != nil {
		glog.Errorf("Failed to fetch jobs for GCP printer %s: %s", printer.GCPID, err)
	} else {
		for i := range jobs {
			go gcp.processJob(&jobs[i], printer, reportJobFailed)
		}
	}
}

// processJob performs these steps:
//
// 1) Assembles the job resources (printer, ticket, data)
// 2) Creates a new job in CUPS.
// 3) Follows up with the job state until done or error.
// 4) Deletes temporary file.
//
// Nothing is returned; intended for use as goroutine.
func (gcp *GoogleCloudPrint) processJob(job *Job, printer *lib.Printer, reportJobFailed func()) {
	glog.Infof("Received GCP job %s", job.GCPJobID)

	ticket, filename, message, state := gcp.assembleJob(job)
	if message != "" {
		reportJobFailed()
		glog.Error(message)
		if err := gcp.Control(job.GCPJobID, state); err != nil {
			glog.Error(err)
		}
		return
	}

	jobTitle := fmt.Sprintf("gcp:%s %s", job.GCPJobID, job.Title)

	gcp.jobs <- &lib.Job{
		CUPSPrinterName: printer.Name,
		Filename:        filename,
		Title:           jobTitle,
		User:            job.OwnerID,
		JobID:           job.GCPJobID,
		Ticket:          ticket,
		UpdateJob:       gcp.Control,
	}
}

// assembleJob prepares for printing a job by fetching the job's ticket and payload.
//
// The caller is responsible to remove the returned file.
//
// Errors are returned as a string (last return value), for reporting
// to GCP and local logging.
func (gcp *GoogleCloudPrint) assembleJob(job *Job) (*cdd.CloudJobTicket, string, string, cdd.PrintJobStateDiff) {
	ticket, err := gcp.Ticket(job.GCPJobID)
	if err != nil {
		return nil, "",
			fmt.Sprintf("Failed to get a ticket for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: &cdd.JobState{
					Type:              cdd.JobStateAborted,
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCauseInvalidTicket},
				},
			}
	}

	file, err := ioutil.TempFile("", "cups-connector-gcp-")
	if err != nil {
		return nil, "",
			fmt.Sprintf("Failed to create a temporary file for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: &cdd.JobState{
					Type:              cdd.JobStateAborted,
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCauseOther},
				},
			}
	}

	gcp.downloadSemaphore.Acquire()
	t := time.Now()
	// Do not check err until semaphore is released and timer is stopped.
	err = gcp.Download(file, job.FileURL)
	dt := time.Since(t)
	gcp.downloadSemaphore.Release()
	if err != nil {
		// Clean up this temporary file so the caller doesn't need extra logic.
		os.Remove(file.Name())
		return nil, "",
			fmt.Sprintf("Failed to download data for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: &cdd.JobState{
					Type:              cdd.JobStateAborted,
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCauseDownloadFailure},
				},
			}
	}

	glog.Infof("Downloaded job %s in %s", job.GCPJobID, dt.String())
	defer file.Close()

	return ticket, file.Name(), "", cdd.PrintJobStateDiff{}
}
