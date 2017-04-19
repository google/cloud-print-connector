/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/gcp-cups-driver/backend-common"
)

type privetInfo struct {
	Version         string                `json:"version"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	URL             string                `json:"url"`
	Type            []string              `json:"type"`
	ID              string                `json:"id"`
	DeviceState     string                `json:"device_state"`
	ConnectionState string                `json:"connection_state"`
	Manufacturer    string                `json:"manufacturer"`
	Model           string                `json:"model"`
	SerialNumber    string                `json:"serial_number"`
	Firmware        string                `json:"firmware"`
	Uptime          int                   `json:"uptime"`
	SetupURL        string                `json:"setup_url"`
	SupportURL      string                `json:"support_url"`
	UpdateURL       string                `json:"update_url"`
	XPrivetToken    string                `json:"x-privet-token"`
	API             []string              `json:"api"`
	SemanticState   *cdd.CloudDeviceState `json:"semantic_state"`
}

func stringSliceContains(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func (pi *privetInfo) deviceMakeAndModel() string {
	if pi.Manufacturer == "" {
		if pi.Model == "" {
			return ""
		} else {
			return pi.Model
		}
	} else {
		if pi.Model == "" {
			return pi.Manufacturer
		} else {
			return fmt.Sprintf("%s %s", pi.Manufacturer, pi.Model)
		}
	}
}

// privetClient is the interface with a Privet printer.
type privetClient struct {
	host string
	// info is useful for reference internally, so we keep a reference to the most
	// recent copy.
	latestInfo *privetInfo
}

func newPrivetClient(hostname string, port uint16) *privetClient {
	return &privetClient{
		host: fmt.Sprintf("%s:%d", hostname, port),
	}
}

type privetError struct {
	Error          *string `json:"error,omitempty"`
	Description    *string `json:"description,omitempty"`
	ServerAPI      *string `json:"server_api,omitempty"`
	ServerCode     *int32  `json:"server_code,omitempty"`
	ServerHTTPCode *int32  `json:"server_http_code,omitempty"`
	Timeout        *int32  `json:"timeout,omitempty"`
}

func (pe *privetError) string() string {
	if pe.Error == nil {
		return ""
	}
	var s string
	s += *pe.Error
	if pe.Description != nil {
		s += " description: " + *pe.Description
	}
	if pe.ServerAPI != nil {
		s += " server API: " + *pe.ServerAPI
	}
	if pe.ServerCode != nil {
		s += " server code: " + string(*pe.ServerCode)
	}
	if pe.ServerHTTPCode != nil {
		s += " server HTTP code: " + string(*pe.ServerHTTPCode)
	}
	if pe.Timeout != nil {
		s += " timeout: " + string(*pe.Timeout)
	}
	return s
}

// doHTTPRequest calls path arg with method arg. Returns a privetError if the response contains an error.
func (pc *privetClient) doHTTPRequest(method, path string, form url.Values, body *os.File, contentType string) ([]byte, *privetError, error) {
	var token string
	if pc.latestInfo != nil {
		token = pc.latestInfo.XPrivetToken
	}

	req := http.Request{
		Method: method,
		URL: &url.URL{
			Scheme: "http",
			Host:   pc.host,
			Path:   path,
		},
		Header: map[string][]string{"X-Privet-Token": {token}},
		Form:   form,
	}

	if body != nil {
		fi, err := body.Stat()
		if err != nil {
			return []byte{}, nil, fmt.Errorf("Failed to stat print job file: %s", err)
		}

		req.Header["Content-Type"] = []string{contentType}
		req.ContentLength = fi.Size()
		req.Body = body
		req.Close = false
	}

	res, err := http.DefaultClient.Do(&req)
	if err != nil {
		return []byte{}, nil, fmt.Errorf("HTTP request failed: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return []byte{}, nil, fmt.Errorf("HTTP response status code %d", res.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte{}, nil, fmt.Errorf("HTTP response body could not be read: %s", err)
	}

	var pe privetError
	if err = json.Unmarshal(responseBody, &pe); err != nil {
		return []byte{}, nil, errors.New("Failed to unmarshal JSON")
	}
	if pe.Error != nil {
		return responseBody, &pe, nil
	}

	return responseBody, nil, nil
}

// info calls /privet/info and updates the internal X-Privet-Token value implicitly.
// Returns nil on failure.
func (pc *privetClient) info() *privetInfo {
	body, pe, err := pc.doHTTPRequest("GET", "/privet/info", nil, nil, "")
	if err != nil {
		common.Error("Failed to get printer info: %s", err)
		return nil
	}
	if pe != nil {
		common.Error("Printer returned an error: %s", pe.string())
		return nil
	}

	var info privetInfo
	if err := json.Unmarshal(body, &info); err != nil {
		common.Error("Failed to unmarshal JSON: %s", err)
		return nil
	}

	pc.latestInfo = &info

	return &info
}

// capabilities returns the capabilities of the printer, and true on success.
// The capabilities API is optional, so (nil, true) is returned if the printer
// doesn't support it.
func (pc *privetClient) capabilities() (*CloudDeviceDescription, bool) {
	if !pc.supportsCapabilities() {
		return nil, true
	}

	body, pe, err := pc.doHTTPRequest("GET", "/privet/capabilities", nil, nil, "")
	if err != nil {
		common.Error("Failed to get printer capabilities: %s", err)
		return nil, false
	}
	if pe != nil {
		common.Error("Printer returned an error: %s", pe.string())
		return nil, false
	}

	var capabilities CloudDeviceDescription
	if err = json.Unmarshal(body, &capabilities); err != nil {
		common.Error("Failed to unmarshal JSON: %s", err)
		return nil, false
	}
	return &capabilities, true
}

func (pc *privetClient) supportsCapabilities() bool {
	if pc.latestInfo == nil {
		common.Error("Cannot acquire printer capabilities without info() call first")
		return false
	}
	return stringSliceContains("/privet/capabilities", pc.latestInfo.API)
}

type CloudDeviceDescription struct {
	*cdd.CloudDeviceDescription
}

func (c *CloudDeviceDescription) supportsPDF() bool {
	if c != nil && c.Printer != nil && c.Printer.SupportedContentType != nil {
		for _, sct := range *c.Printer.SupportedContentType {
			if sct.ContentType == "application/pdf" || sct.ContentType == "*/*" {
				return true
			}
		}
	}
	return false
}

// createJob passes the ticket arg to the printer's createjob API.
// Returns the new job ID and true on success.
// Returns "" and true if the printer doesn't support the createjob API.
// Returns "" and false on failure.
func (pc *privetClient) createJob(ticket *cdd.CloudJobTicket) (string, bool) {
	if !pc.supportsCreateJob() {
		return "", true
	}

	body, pe, err := pc.doHTTPRequest("POST", "/privet/printer/createjob", nil, nil, "")
	if err != nil {
		common.Error("Failed to create new print job: %s", err)
		return "", false
	}
	if pe != nil {
		switch *pe.Error {
		case "printer_busy":
			// TODO: signal retry to CUPS
		case "printer_error":
			// TODO: signal state to CUPS
		default:
			common.Error("Printer returned an error: %s", pe.string())
		}
		return "", false
	}

	var newJob struct {
		JobID     string `json:"job_id"`
		ExpiresIn int32  `json:"expires_in"`
	}
	if err = json.Unmarshal(body, &newJob); err != nil {
		common.Error("Failed to unmarshal JSON: %s", err)
		return "", false
	}

	return newJob.JobID, true
}

func (pc *privetClient) supportsCreateJob() bool {
	if pc.latestInfo == nil {
		common.Error("Cannot call createjob without info() call first")
		return false
	}
	return stringSliceContains("/privet/printer/createjob", pc.latestInfo.API)
}

// submitDoc passes a file to the printer's submitdoc API.
// The jobID arg is optional; see "Simple printing" in GCP dev docs.
// Returns the (new) job ID and true on success.
// Returns "" and false on failure.
func (pc *privetClient) submitDoc(jobID, userName, jobName string, body *os.File, contentType string) (string, bool) {
	// TODO: Do this in a streaming way and update CUPS as progress is made.
	/*
		r, w := io.Pipe()
		go func() {
		}()
	*/
	form := url.Values{}
	form.Add("job_id", jobID)
	form.Add("user_name", userName)
	form.Add("client_name", "TODO CUPS virtual driver")
	form.Add("job_name", jobName)

	responseBody, pe, err := pc.doHTTPRequest("POST", "/privet/printer/submitdoc", form, body, contentType)
	if err != nil {
		common.Error("Failed to submit new print job: %s", err)
		return "", false
	}
	if pe != nil {
		switch *pe.Error {
		case "invalid_print_job", "printer_busy":
			// TODO: signal retry to CUPS
		default:
			common.Error("Printer returned an error: %s", pe.string())
		}
		return "", false
	}

	var newJob struct {
		JobID     string `json:"job_id"`
		ExpiresIn int32  `json:"expires_in"`
		JobType   string `json:"job_type"`
		JobSize   int64  `json:"job_size"`
		JobName   string `json:"job_name"`
	}
	if err := json.Unmarshal(responseBody, &newJob); err != nil {
		common.Error("Failed to unmarshal JSON: %s", err)
		return "", false
	}

	return newJob.JobID, true
}
