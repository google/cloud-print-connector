/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Package gcp is the Google Cloud Print API client.
package gcp

import (
	"cups-connector/lib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
)

const (
	// XMPP connections fail. Attempt to reconnect a few times before giving up.
	restartXMPPMaxRetries = 4

	// Stop retrying when two failurees occur in succession in this quantity of seconds.
	minimumTimeBetweenXMPPFailures = 3

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
	xmppClient  *gcpXMPP
	robotClient *http.Client
	userClient  *http.Client
	proxyName   string
	xmppServer  string
	xmppPort    uint16
}

// NewGoogleCloudPrint establishes a connection with GCP, returns a new GoogleCloudPrint object.
func NewGoogleCloudPrint(baseURL, xmppJID, robotRefreshToken, userRefreshToken, proxyName, oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, xmppServer string, xmppPort uint16) (*GoogleCloudPrint, error) {
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

	gcp := &GoogleCloudPrint{baseURL, xmppJID, nil, robotClient, userClient, proxyName, xmppServer, xmppPort}

	return gcp, nil
}

type localSettingsInner struct {
	XMPPTimeoutValue uint32 `json:"xmpp_timeout_value"`
	// Other values can be set here; omit until needed.
}

type localSettingsOuter struct {
	// "current" name must be lower-case, but can't annotate inner struct
	// without separate type declaration.
	Current localSettingsInner `json:"current"`
}

// Turn arguments into a JSON-encoded GCP LocalSettings object.
func marshalLocalSettings(xmppTimeout uint32) (string, error) {
	var ls localSettingsOuter
	ls.Current.XMPPTimeoutValue = xmppTimeout
	lss, err := json.Marshal(ls)
	if err != nil {
		return "", err
	}
	return string(lss), nil
}

func newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, refreshToken string, scopes ...string) (*http.Client, error) {
	config := &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthAuthURL,
			TokenURL: oauthTokenURL,
		},
		RedirectURL: RedirectURL,
		Scopes:      scopes,
	}

	token := &oauth2.Token{RefreshToken: refreshToken}
	client := config.Client(oauth2.NoContext, token)

	return client, nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (gcp *GoogleCloudPrint) Quit() {
	gcp.xmppClient.quit()
}

// CanShare answers the question "can we share printers when they are registered?"
func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userClient != nil
}

// RestartXMPP tries to start an XMPP conversation.
// Tries multiple times before returning an error.
func (gcp *GoogleCloudPrint) RestartXMPP() error {
	if gcp.xmppClient != nil {
		go gcp.xmppClient.quit()
	}

	var err error
	for i := 0; i < restartXMPPMaxRetries; i++ {
		// The current access token is the XMPP password.
		var token *oauth2.Token
		token, err = gcp.robotClient.Transport.(*oauth2.Transport).Source.Token()

		if err == nil {
			var xmpp *gcpXMPP
			xmpp, err = newXMPP(gcp.xmppJID, token.AccessToken, gcp.proxyName, gcp.xmppServer, gcp.xmppPort)

			if err == nil {
				gcp.xmppClient = xmpp
				return nil
			}
		}

		// Sleep for 1, 2, 4, 8 seconds.
		time.Sleep(time.Duration((i+1)*2) * time.Second)
	}

	return fmt.Errorf("Failed to (re-)start XMPP conversation: %s", err)
}

// nextWaitingPrinter calls xmppClient.nextWaitingPrinter(), with retries on
// failure. If a call to this function fails, then there's probably no reason to
// go on living.
//
// Returns a base64-encoded string representation of the ID of the printer
// that has waiting job(s).
func (gcp *GoogleCloudPrint) nextWaitingPrinterWithRetries() (string, error) {
	lastFailure := time.Unix(0, 0)
	for {
		printerIDb64, err := gcp.xmppClient.nextWaitingPrinter()
		if err == nil {
			// Success!
			return printerIDb64, nil
		}

		if err == ErrClosed {
			// The connection is closed.
			return "", err
		}

		if strings.HasPrefix(err.Error(), "Unexpected element") {
			// Not really an error, but interesting, so log and try again.
			glog.Warningf("While waiting for print jobs: %s", err)
			continue
		}

		if time.Since(lastFailure) < time.Duration(minimumTimeBetweenXMPPFailures)*time.Second {
			// There was a failure very recently; a third try is likely to fail.
			return "", fmt.Errorf("While (retrying) waiting for print jobs: %s", err)
		}
		lastFailure = time.Now()

		glog.Warningf("Restarting XMPP conversation because: %s", err)
		if err = gcp.RestartXMPP(); err != nil {
			// Restarting XMPP failed, so what's the point?
			return "", err
		}
		glog.Info("Started XMPP successfully")
	}
	panic("unreachable")
}

// NextJobBatch gets the next batch of print jobs from GCP. Blocks on XMPP until
// batch notification arrives. Calls google.com/cloudprint/fetch to get the jobs.
//
// Panics when XMPP error is too serious to retry.
func (gcp *GoogleCloudPrint) NextJobBatch() ([]lib.Job, error) {
	printerIDb64, err := gcp.nextWaitingPrinterWithRetries()
	if err != nil {
		if err == ErrClosed {
			return nil, err
		}
		glog.Fatalf("Fatal error while waiting for next printer: %s", err)
	}
	printerIDbytes, err := base64.StdEncoding.DecodeString(printerIDb64)
	if err != nil {
		return nil, err
	}
	printerID := string(printerIDbytes)
	return gcp.Fetch(printerID)
}

// Control calls google.com/cloudprint/control to set the status of a
// GCP print job.
func (gcp *GoogleCloudPrint) Control(jobID string, status lib.GCPJobStatus, code, message string) error {
	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("status", string(status))
	form.Set("code", code)
	form.Set("message", message)

	if _, _, _, err := gcp.postWithRetry(gcp.robotClient, "control", form); err != nil {
		return err
	}

	return nil
}

// Delete calls google.com/cloudprint/delete to delete a printer from GCP.
func (gcp *GoogleCloudPrint) Delete(gcpID string) error {
	form := url.Values{}
	form.Set("printerid", gcpID)

	if _, _, _, err := gcp.postWithRetry(gcp.robotClient, "delete", form); err != nil {
		return err
	}

	return nil
}

// Fetch calls google.com/cloudprint/fetch to get the outstanding print jobs for
// a GCP printer.
func (gcp *GoogleCloudPrint) Fetch(gcpID string) ([]lib.Job, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, errorCode, _, err := gcp.postWithRetry(gcp.robotClient, "fetch", form)
	if err != nil {
		if errorCode == 413 {
			// 413 means "Zero print jobs returned", which isn't really an error.
			return []lib.Job{}, nil
		}
		return nil, err
	}

	var jobsData struct {
		Jobs []struct {
			ID        string
			Title     string
			FileURL   string
			TicketURL string
			OwnerID   string
		}
	}
	if err = json.Unmarshal(responseBody, &jobsData); err != nil {
		return nil, err
	}

	jobs := make([]lib.Job, 0, len(jobsData.Jobs))

	for _, jobData := range jobsData.Jobs {
		job := lib.Job{
			GCPPrinterID: gcpID,
			GCPJobID:     jobData.ID,
			FileURL:      jobData.FileURL,
			TicketURL:    jobData.TicketURL,
			OwnerID:      jobData.OwnerID,
			Title:        jobData.Title,
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// List calls google.com/cloudprint/list to get all GCP printers assigned
// to this connector.
//
// The second return value is a map of GCPID:queuedPrintJobs.
func (gcp *GoogleCloudPrint) List() ([]lib.Printer, map[string]uint, error) {
	form := url.Values{}
	form.Set("proxy", gcp.proxyName)
	form.Set("extra_fields", "queuedJobsCount")

	responseBody, _, _, err := gcp.postWithRetry(gcp.robotClient, "list", form)
	if err != nil {
		return nil, nil, err
	}

	var listData struct {
		Printers []struct {
			ID                 string
			Name               string
			DefaultDisplayName string
			Description        string
			Status             string
			CapsHash           string
			LocalSettings      localSettingsOuter `json:"local_settings"`
			Tags               []string
			QueuedJobsCount    uint
		}
	}
	if err = json.Unmarshal(responseBody, &listData); err != nil {
		return nil, nil, err
	}

	queuedJobsCount := make(map[string]uint)

	printers := make([]lib.Printer, 0, len(listData.Printers))
	for _, p := range listData.Printers {
		tags := make(map[string]string)
		for _, tag := range p.Tags {
			s := strings.SplitN(tag, "=", 2)
			key := s[0][5:]
			var value string
			if len(s) > 1 {
				value = s[1]
			}
			tags[key] = value
		}

		var xmppTimeout uint32
		if p.LocalSettings.Current.XMPPTimeoutValue > 0 {
			xmppTimeout = p.LocalSettings.Current.XMPPTimeoutValue
		}

		printer := lib.Printer{
			GCPID:              p.ID,
			Name:               p.Name,
			DefaultDisplayName: p.DefaultDisplayName,
			Description:        p.Description,
			Status:             lib.PrinterStatusFromString(p.Status),
			CapsHash:           p.CapsHash,
			XMPPTimeout:        xmppTimeout,
			Tags:               tags,
		}
		printers = append(printers, printer)

		if p.QueuedJobsCount > 0 {
			queuedJobsCount[p.ID] = p.QueuedJobsCount
		}
	}

	return printers, queuedJobsCount, nil
}

// Register calls google.com/cloudprint/register to register a GCP printer.
//
// Sets the GCPID field in the printer arg.
func (gcp *GoogleCloudPrint) Register(printer *lib.Printer, ppd string) error {
	if len(ppd) <= 0 {
		return errors.New("GCP requires a non-empty PPD")
	}

	cdd, err := gcp.Translate(ppd)
	if err != nil {
		return err
	}

	localSettings, err := marshalLocalSettings(printer.XMPPTimeout)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("name", printer.Name)
	form.Set("default_display_name", printer.DefaultDisplayName)
	form.Set("proxy", gcp.proxyName)
	form.Set("local_settings", localSettings)
	form.Set("use_cdd", "true")
	form.Set("capabilities", cdd)
	form.Set("description", printer.Description)
	form.Set("status", string(printer.Status))
	form.Set("capsHash", printer.CapsHash)
	form.Set("content_types", "application/pdf")
	for key, value := range printer.Tags {
		form.Add("tag", fmt.Sprintf("%s=%s", key, value))
	}

	responseBody, _, _, err := gcp.postWithRetry(gcp.robotClient, "register", form)
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
func (gcp *GoogleCloudPrint) Update(diff *lib.PrinterDiff, ppd string) error {
	form := url.Values{}
	form.Set("printerid", diff.Printer.GCPID)
	form.Set("proxy", gcp.proxyName)

	// Ignore Name field because it never changes.
	if diff.DefaultDisplayNameChanged {
		form.Set("default_display_name", diff.Printer.DefaultDisplayName)
	}

	if diff.DescriptionChanged {
		form.Set("description", diff.Printer.Description)
	}

	if diff.StatusChanged {
		form.Set("status", string(diff.Printer.Status))
	}

	if diff.CapsHashChanged {
		cdd, err := gcp.Translate(ppd)
		if err != nil {
			return err
		}

		form.Set("use_cdd", "true")
		form.Set("capsHash", diff.Printer.CapsHash)
		form.Set("capabilities", cdd)
	}

	if diff.XMPPTimeoutChanged {
		localSettings, err := marshalLocalSettings(diff.Printer.XMPPTimeout)
		if err != nil {
			return err
		}

		form.Set("local_settings", localSettings)
	}

	if diff.TagsChanged {
		for key, value := range diff.Printer.Tags {
			form.Add("tag", fmt.Sprintf("%s=%s", key, value))
		}
		form.Set("remove_tag", ".*")
	}

	if _, _, _, err := gcp.postWithRetry(gcp.robotClient, "update", form); err != nil {
		return err
	}

	return nil
}

// Translate calls google.com/cloudprint/tools/cdd/translate to translate a PPD to CDD.
func (gcp *GoogleCloudPrint) Translate(ppd string) (string, error) {
	form := url.Values{}
	form.Set("capabilities", ppd)

	responseBody, _, _, err := gcp.postWithRetry(gcp.robotClient, "tools/cdd/translate", form)
	if err != nil {
		return "", err
	}

	var cddInterface interface{}
	if err = json.Unmarshal(responseBody, &cddInterface); err != nil {
		return "", fmt.Errorf("Failed to unmarshal translated CDD: %s", err)
	}

	cdd, ok := cddInterface.(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}
	cdd, ok = cdd["cdd"].(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}
	p, ok := cdd["printer"].(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}

	p["copies"] = map[string]int{
		"max":     100,
		"default": 1,
	}
	p["collate"] = map[string]bool{
		"default": true,
	}

	cddString, err := json.Marshal(cdd)
	if err != nil {
		return "", fmt.Errorf("Failed to remarshal translated CDD: %s", err)
	}

	return string(cddString), nil
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

	if _, _, _, err := gcp.postWithRetry(gcp.userClient, "share", form); err != nil {
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

// getWithRetry calls get() and retries once on HTTP failure
// (response code != 200).
func getWithRetry(hc *http.Client, url string) (*http.Response, error) {
	response, err := get(hc, url)
	if response != nil && response.StatusCode == 200 {
		return response, err
	}

	return get(hc, url)
}

// get GETs a URL. Returns the response object (not body), in case the body
// is very large.
//
// The caller must close the returned Response.Body object if err == nil.
func get(hc *http.Client, url string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-CloudPrint-Proxy", "cups-cloudprint-"+runtime.GOOS)

	response, err := hc.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET failure: %s", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET HTTP-level failure: %s %s", url, response.Status)
	}

	return response, nil
}

// postWithRetry calls post() and retries once on HTTP failure
// (response code != 200).
func (gcp *GoogleCloudPrint) postWithRetry(hc *http.Client, method string, form url.Values) ([]byte, uint, int, error) {
	responseBody, gcpErrorCode, httpStatusCode, err := gcp.post(hc, method, form)
	if responseBody != nil && httpStatusCode == 200 {
		return responseBody, gcpErrorCode, httpStatusCode, err
	}

	return gcp.post(hc, method, form)
}

// post POSTs to a GCP method. Returns the body of the response.
//
// Returns the response body, GCP error code, HTTP status, and error.
// On success, only the response body is guaranteed to be non-zero.
func (gcp *GoogleCloudPrint) post(hc *http.Client, method string, form url.Values) ([]byte, uint, int, error) {
	requestBody := strings.NewReader(form.Encode())
	request, err := http.NewRequest("POST", gcp.baseURL+method, requestBody)
	if err != nil {
		return nil, 0, 0, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CloudPrint-Proxy", "cups-cloudprint-"+runtime.GOOS)

	response, err := hc.Do(request)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("/%s POST failure: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, 0, response.StatusCode, fmt.Errorf("/%s POST HTTP-level failure: %s", method, response.Status)
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, 0, response.StatusCode, err
	}

	var responseStatus struct {
		Success   bool
		Message   string
		ErrorCode uint
	}
	if err = json.Unmarshal(responseBody, &responseStatus); err != nil {
		return nil, 0, response.StatusCode, err
	}
	if !responseStatus.Success {
		return nil, responseStatus.ErrorCode, response.StatusCode, fmt.Errorf(
			"/%s call failed: %s", method, responseStatus.Message)
	}

	return responseBody, 0, response.StatusCode, nil
}
