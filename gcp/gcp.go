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
	"strings"

	"github.com/golang/oauth2"
)

const baseURL = "https://www.google.com/cloudprint/"

// Interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	transport  *oauth2.Transport
	xmppClient *xmpp.XMPP
	proxyName  string
}

func NewGoogleCloudPrint(refreshToken, xmppJID, proxyName string) (*GoogleCloudPrint, error) {
	options := &oauth2.Options{
		ClientID:     lib.ClientID,
		ClientSecret: lib.ClientSecret,
		RedirectURL:  lib.RedirectURL,
		Scopes:       []string{lib.ScopeCloudPrint, lib.ScopeGoogleTalk},
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

	xmppClient, err := xmpp.NewXMPP(xmppJID, transport.Token().AccessToken)
	if err != nil {
		return nil, err
	}

	return &GoogleCloudPrint{transport, xmppClient, proxyName}, nil
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
	if gcp.transport.Token().Expired() {
		gcp.transport.RefreshToken()
	}
	return gcp.transport.Token().AccessToken
}

// Calls google.com/cloudprint/control.
func (gcp *GoogleCloudPrint) Control(jobID, status, message string) error {
	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("status", status)
	form.Set("message", message)

	if _, err := gcp.post("control", form); err != nil {
		return err
	}

	return nil
}

// Calls google.com/cloudprint/delete.
func (gcp *GoogleCloudPrint) Delete(id string) error {
	form := url.Values{}
	form.Set("printerid", id)

	if _, err := gcp.post("delete", form); err != nil {
		return err
	}

	return nil
}

// Gets the outstanding print jobs for a printer.
//
// Calls google.com/cloudprint/fetch.
func (gcp *GoogleCloudPrint) Fetch(printerID string) ([]lib.Job, error) {
	form := url.Values{}
	form.Set("printerid", printerID)

	responseBody, err := gcp.post("fetch", form)
	if err != nil {
		return nil, err
	}

	var jobsData struct {
		Jobs []struct {
			Id      string
			FileURL string
		}
	}
	if err = json.Unmarshal(responseBody, &jobsData); err != nil {
		return nil, err
	}

	jobs := make([]lib.Job, 0, len(jobsData.Jobs))

	for _, jobData := range jobsData.Jobs {
		job := lib.Job{
			GCPPrinterID: printerID,
			GCPJobID:     jobData.Id,
			FileURL:      jobData.FileURL,
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

	responseBody, err := gcp.post("list", form)
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

	responseBody, err := gcp.post("register", form)
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

	if _, err := gcp.post("update", form); err != nil {
		return err
	}

	return nil
}

// Downloads a url (print job) to a Writer.
func (gcp *GoogleCloudPrint) Download(dst io.Writer, url string) error {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	response, err := gcp.transport.RoundTrip(request)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		text := fmt.Sprintf("Download failed: %s %s", url, response.Status)
		return errors.New(text)
	}

	_, err = io.Copy(dst, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func (gcp *GoogleCloudPrint) post(method string, form url.Values) ([]byte, error) {
	requestBody := strings.NewReader(form.Encode())
	request, err := http.NewRequest("POST", baseURL+method, requestBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := gcp.transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		text := fmt.Sprintf("/%s HTTP request failed with %s", method, response.Status)
		return nil, errors.New(text)
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var responseStatus struct {
		Success bool
		Message string
	}
	if err = json.Unmarshal(responseBody, &responseStatus); err != nil {
		return nil, err
	}
	if !responseStatus.Success {
		text := fmt.Sprintf("/%s RPC call failed with %s", method, responseStatus.Message)
		return nil, errors.New(text)
	}

	return responseBody, nil
}
