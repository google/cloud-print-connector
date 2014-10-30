/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package gcp

// TODO(jacobmarble): Implement Quit() function that causes XMPP to stop receiving jobs.

import (
	"cups-connector/gcp/xmpp"
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

	"github.com/golang/oauth2"
)

const baseURL = "https://www.google.com/cloudprint/"

// Interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	xmppClient     *xmpp.XMPP
	robotTransport *oauth2.Transport
	userTransport  *oauth2.Transport
	shareScope     string
	proxyName      string
}

func NewGoogleCloudPrint(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName string) (*GoogleCloudPrint, error) {
	robotTransport, err := newTransport(robotRefreshToken, lib.ScopeCloudPrint, lib.ScopeGoogleTalk)
	if err != nil {
		return nil, err
	}

	var userTransport *oauth2.Transport
	if userRefreshToken != "" && shareScope != "" {
		userTransport, err = newTransport(userRefreshToken, lib.ScopeCloudPrint)
		if err != nil {
			return nil, err
		}
	}

	xmppClient, err := xmpp.NewXMPP(xmppJID, robotTransport.Token().AccessToken)
	if err != nil {
		return nil, err
	}

	return &GoogleCloudPrint{xmppClient, robotTransport, userTransport, shareScope, proxyName}, nil
}

func newTransport(refreshToken string, scopes ...string) (*oauth2.Transport, error) {
	options := &oauth2.Options{
		ClientID:     lib.ClientID,
		ClientSecret: lib.ClientSecret,
		RedirectURL:  lib.RedirectURL,
		Scopes:       scopes,
	}
	oauthConfig, err := oauth2.NewConfig(options, lib.AuthURL, lib.TokenURL)
	if err != nil {
		return nil, err
	}

	transport := oauthConfig.NewTransport()
	transport.SetToken(&oauth2.Token{RefreshToken: refreshToken})
	// Get first access token to be sure we can.
	if err = transport.RefreshToken(); err != nil {
		return nil, err
	}

	return transport, nil
}

func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userTransport != nil
}

// Waits for the next batch of jobs from GCP. Blocks until batch arrives.
//
// Calls google.com/cloudprint/fetch.
func (gcp *GoogleCloudPrint) NextJobBatch() ([]lib.Job, error) {
	printerIDb64, err := gcp.xmppClient.NextWaitingPrinter()
	if err != nil {
		return nil, err
	}

	printerIDbyte, err := base64.StdEncoding.DecodeString(printerIDb64)
	if err != nil {
		return nil, err
	}

	return gcp.Fetch(string(printerIDbyte))
}

func (gcp *GoogleCloudPrint) GetAccessToken() string {
	if gcp.robotTransport.Token().Expired() {
		gcp.robotTransport.RefreshToken()
	}
	return gcp.robotTransport.Token().AccessToken
}

// Calls google.com/cloudprint/control.
func (gcp *GoogleCloudPrint) Control(jobID string, status lib.GCPJobStatus, code, message string) error {
	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("status", string(status))
	form.Set("code", code)
	form.Set("message", message)

	if _, _, err := post(gcp.robotTransport, "control", form); err != nil {
		return err
	}

	return nil
}

// Calls google.com/cloudprint/delete.
func (gcp *GoogleCloudPrint) Delete(gcpID string) error {
	form := url.Values{}
	form.Set("printerid", gcpID)

	if _, _, err := post(gcp.robotTransport, "delete", form); err != nil {
		return err
	}

	return nil
}

// Gets the outstanding print jobs for a printer.
//
// Calls google.com/cloudprint/fetch.
func (gcp *GoogleCloudPrint) Fetch(gcpID string) ([]lib.Job, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, errorCode, err := post(gcp.robotTransport, "fetch", form)
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
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Gets all GCP printers assigned to the configured proxy.
//
// Calls google.com/cloudprint/list.
func (gcp *GoogleCloudPrint) List() ([]lib.Printer, error) {
	form := url.Values{}
	form.Set("proxy", gcp.proxyName)

	responseBody, _, err := post(gcp.robotTransport, "list", form)
	if err != nil {
		return nil, err
	}

	var listData struct {
		Printers []struct {
			Id                 string
			Name               string
			DefaultDisplayName string
			Description        string
			Status             string
			CapsHash           string
			Tags               []string
		}
	}
	if err = json.Unmarshal(responseBody, &listData); err != nil {
		return nil, err
	}

	printers := make([]lib.Printer, 0, len(listData.Printers))
	for _, p := range listData.Printers {
		tags := make(map[string]string)
		for _, tag := range p.Tags {
			if !strings.HasPrefix(tag, "cups-") {
				continue
			}
			s := strings.SplitN(tag, "=", 2)
			key := s[0][5:]
			var value string
			if len(s) > 1 {
				value = s[1]
			}
			tags[key] = value
		}

		printer := lib.Printer{
			GCPID:              p.Id,
			Name:               p.Name,
			DefaultDisplayName: p.DefaultDisplayName,
			Description:        p.Description,
			Status:             lib.PrinterStatusFromString(p.Status),
			CapsHash:           p.CapsHash,
			Tags:               tags,
		}
		printers = append(printers, printer)
	}

	return printers, nil
}

// Registers a Google Cloud Print Printer. Sets the GCPID field in the printer arg.
//
// Calls google.com/cloudprint/register.
func (gcp *GoogleCloudPrint) Register(printer *lib.Printer, ppd string) error {
	if len(ppd) <= 0 {
		return errors.New("GCP requires a non-empty PPD")
	}

	form := url.Values{}
	form.Set("name", printer.Name)
	form.Set("default_display_name", printer.DefaultDisplayName)
	form.Set("proxy", gcp.proxyName)
	form.Set("capabilities", string(ppd))
	form.Set("description", printer.Description)
	form.Set("status", string(printer.Status))
	form.Set("capsHash", printer.CapsHash)
	form.Set("content_types", "application/pdf")
	for key, value := range printer.Tags {
		form.Add("tag", fmt.Sprintf("cups-%s=%s", key, value))
	}

	responseBody, _, err := post(gcp.robotTransport, "register", form)
	if err != nil {
		return err
	}

	var registerData struct {
		Printers []struct {
			Id string
		}
	}
	if err = json.Unmarshal(responseBody, &registerData); err != nil {
		return err
	}

	printer.GCPID = registerData.Printers[0].Id

	return nil
}

// Updates a Google Cloud Print Printer.
//
// Calls google.com/cloudprint/update.
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
		form.Set("capsHash", diff.Printer.CapsHash)
		form.Set("capabilities", ppd)
	}

	if diff.TagsChanged {
		for key, value := range diff.Printer.Tags {
			form.Add("tag", fmt.Sprintf("cups-%s=%s", key, value))
		}
		form.Set("remove_tag", "^cups-.*")
	}

	if _, _, err := post(gcp.robotTransport, "update", form); err != nil {
		return err
	}

	return nil
}

// Shares a GCP printer.
//
// Calls google.com/cloudprint/share.
func (gcp *GoogleCloudPrint) Share(gcpID string) error {
	if gcp.userTransport == nil {
		return errors.New("Cannot share because user OAuth credentials not provided.")
	}

	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("scope", gcp.shareScope)
	form.Set("role", "USER")
	form.Set("skip_notification", "true")

	if _, _, err := post(gcp.userTransport, "share", form); err != nil {
		return err
	}

	return nil
}

// Downloads a url (print job) to a Writer.
func (gcp *GoogleCloudPrint) Download(dst io.Writer, url string) error {
	response, err := get(gcp.robotTransport, url)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, response.Body)
	if err != nil {
		return err
	}

	return nil
}

// Gets a ticket (job options), returns it as a map.
func (gcp *GoogleCloudPrint) Ticket(ticketURL string) (map[string]string, error) {
	response, err := get(gcp.robotTransport, ticketURL)
	if err != nil {
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	err = json.Unmarshal(responseBody, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// GETs to a URL. Returns the response object, in case the body is very large.
func get(t *oauth2.Transport, url string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-CloudPrint-Proxy", "cups-cloudprint-"+runtime.GOOS)

	response, err := t.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET failed: %s %s", url, response.Status)
	}

	return response, nil
}

// POSTs to a GCP method. Returns the body of the response.
//
// On error, the last two return values are non-zero values.
func post(t *oauth2.Transport, method string, form url.Values) ([]byte, uint, error) {
	requestBody := strings.NewReader(form.Encode())
	request, err := http.NewRequest("POST", baseURL+method, requestBody)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CloudPrint-Proxy", "cups-cloudprint-"+runtime.GOOS)

	response, err := t.RoundTrip(request)
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode != 200 {
		return nil, 0, fmt.Errorf("/%s call failed: %s", method, response.Status)
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, 0, err
	}

	var responseStatus struct {
		Success   bool
		Message   string
		ErrorCode uint
	}
	if err = json.Unmarshal(responseBody, &responseStatus); err != nil {
		return nil, 0, err
	}
	if !responseStatus.Success {
		return nil, responseStatus.ErrorCode, fmt.Errorf(
			"/%s call failed: %s", method, responseStatus.Message)
	}

	return responseBody, 0, nil
}
