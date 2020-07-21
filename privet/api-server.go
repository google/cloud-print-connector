/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/cloud-print-connector/cdd"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
)

var (
	closed              = errors.New("closed")
	supportedAPIsOnline = []string{
		"/privet/accesstoken",
		"/privet/capabilities",
		"/privet/printer/createjob",
		"/privet/printer/submitdoc",
		"/privet/printer/jobstate",
	}
	supportedAPIsOffline = []string{
		"/privet/capabilities",
		"/privet/printer/createjob",
		"/privet/printer/submitdoc",
		"/privet/printer/jobstate",
	}
)

type privetError struct {
	Error          string `json:"error"`
	Description    string `json:"description,omitempty"`
	ServerAPI      string `json:"server_api,omitempty"`
	ServerCode     int    `json:"server_code,omitempty"`
	ServerHTTPCode int    `json:"server_http_code,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
}

func writeError(w http.ResponseWriter, e, description string) {
	pe := privetError{
		Error:       e,
		Description: description,
	}.json()
	w.Write(pe)
}

func (e privetError) json() []byte {
	marshalled, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal Privet Error: %s", err)
	}
	return marshalled
}

type privetAPI struct {
	gcpID string
	name  string

	gcpBaseURL string
	xsrf       xsrfSecret
	online     bool
	jc         *jobCache
	jobs       chan<- *lib.Job

	getPrinter        func(string) (lib.Printer, bool)
	getProximityToken func(string, string) ([]byte, int, error)

	listener  *quittableListener
	startTime time.Time
}

func newPrivetAPI(gcpID, name, gcpBaseURL string, xsrf xsrfSecret, online bool, jc *jobCache, jobs chan<- *lib.Job, getPrinter func(string) (lib.Printer, bool), getProximityToken func(string, string) ([]byte, int, error), listener *quittableListener) (*privetAPI, error) {
	api := &privetAPI{
		gcpID:      gcpID,
		name:       name,
		gcpBaseURL: gcpBaseURL,
		xsrf:       xsrf,
		online:     online,
		jc:         jc,
		jobs:       jobs,

		getPrinter:        getPrinter,
		getProximityToken: getProximityToken,

		listener:  listener,
		startTime: time.Now(),
	}
	go api.serve()

	return api, nil
}

func (api *privetAPI) port() uint16 {
	return uint16(api.listener.Addr().(*net.TCPAddr).Port)
}

func (api *privetAPI) quit() {
	api.listener.quit()
}

func (api *privetAPI) serve() {
	sm := http.NewServeMux()
	sm.HandleFunc("/privet/info", api.info)
	if api.online {
		sm.HandleFunc("/privet/accesstoken", api.accesstoken)
	}
	sm.HandleFunc("/privet/capabilities", api.capabilities)
	sm.HandleFunc("/privet/printer/createjob", api.createjob)
	sm.HandleFunc("/privet/printer/submitdoc", api.submitdoc)
	sm.HandleFunc("/privet/printer/jobstate", api.jobstate)

	err := http.Serve(api.listener, sm)
	if err != nil && err != closed {
		log.Errorf("Privet API HTTP server failed: %s", err)
	}
}

type infoResponse struct {
	Version         string               `json:"version"`
	Name            string               `json:"name"`
	Description     string               `json:"description,omitempty"`
	URL             string               `json:"url"`
	Type            []string             `json:"type"`
	ID              string               `json:"id"`
	DeviceState     string               `json:"device_state"`
	ConnectionState string               `json:"connection_state"`
	Manufacturer    string               `json:"manufacturer"`
	Model           string               `json:"model"`
	SerialNumber    string               `json:"serial_number,omitempty"`
	Firmware        string               `json:"firmware"`
	Uptime          uint                 `json:"uptime"`
	SetupURL        string               `json:"setup_url,omitempty"`
	SupportURL      string               `json:"support_url,omitempty"`
	UpdateURL       string               `json:"update_url,omitempty"`
	XPrivetToken    string               `json:"x-privet-token"`
	API             []string             `json:"api"`
	SemanticState   cdd.CloudDeviceState `json:"semantic_state,omitempty"`
}

func (api *privetAPI) info(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /info request: %+v", r)
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, exists := r.Header["X-Privet-Token"]; !exists {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "invalid_x_privet_token",
			"X-Privet-Token request header is missing or invalid")
		return
	}

	printer, exists := api.getPrinter(api.name)
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var s cdd.CloudConnectionStateType
	if api.online {
		s = cdd.CloudConnectionStateOnline
	} else {
		s = cdd.CloudConnectionStateOffline
	}
	state := cdd.CloudDeviceState{
		Version:              "1.0",
		CloudConnectionState: &s,
		Printer:              printer.State,
	}

	var connectionState string
	var supportedAPIs []string
	if api.online {
		connectionState = "online"
		supportedAPIs = supportedAPIsOnline
	} else {
		connectionState = "offline"
		supportedAPIs = supportedAPIsOffline
	}

	response := infoResponse{
		Version:         "1.0",
		Name:            printer.DefaultDisplayName,
		URL:             api.gcpBaseURL,
		Type:            []string{"printer"},
		ID:              printer.GCPID,
		DeviceState:     strings.ToLower(string(printer.State.State)),
		ConnectionState: connectionState,
		Manufacturer:    printer.Manufacturer,
		Model:           printer.Model,
		SerialNumber:    printer.UUID,
		Firmware:        printer.ConnectorVersion,
		Uptime:          uint(time.Since(api.startTime).Seconds()),
		SetupURL:        printer.SetupURL,
		SupportURL:      printer.SupportURL,
		UpdateURL:       printer.UpdateURL,
		XPrivetToken:    api.xsrf.newToken(),
		API:             supportedAPIs,
		SemanticState:   state,
	}

	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal Privet info: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) checkRequest(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	if token, exists := r.Header["X-Privet-Token"]; !exists || len(token) != 1 || !api.xsrf.isTokenValid(token[0]) {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "invalid_x_privet_token",
			"X-Privet-Token request header is missing or invalid")
		return false
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func (api *privetAPI) accesstoken(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /accesstoken request: %+v", r)
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	user := r.Form.Get("user")
	if len(user) == 0 {
		writeError(w, "invalid_params", "user parameter expected")
		return
	}

	responseBody, httpStatusCode, err := api.getProximityToken(api.gcpID, user)
	if err != nil {
		log.Errorf("Failed to get proximity token: %s", err)
	}

	if responseBody == nil || len(responseBody) == 0 {
		log.Warning("Cloud returned empty response body")
		writeError(w, "server_error", "Check connector logs")
		return
	}

	var response struct {
		Success        bool                   `json:"success"`
		Message        string                 `json:"message"`
		ErrorCode      int                    `json:"errorCode"`
		ProximityToken map[string]interface{} `json:"proximity_token"`
	}
	if err = json.Unmarshal(responseBody, &response); err != nil {
		log.Errorf("Failed to unmarshal ticket from cloud: %s", err)
		writeError(w, "server_error", "Check connector logs")
		return
	}

	if response.Success {
		token, err := json.MarshalIndent(response.ProximityToken, "", "  ")
		if err != nil {
			log.Errorf("Failed to marshal something that was just unmarshalled: %s", err)
			writeError(w, "server_error", "Check connector logs")
		} else {
			w.Write(token)
		}
		return
	}

	if response.ErrorCode != 0 {
		e := privetError{
			Error:          "server_error",
			Description:    response.Message,
			ServerAPI:      "/proximitytoken",
			ServerCode:     response.ErrorCode,
			ServerHTTPCode: httpStatusCode,
		}.json()
		w.Write(e)
		return
	}

	writeError(w, "server_error", "Check connector logs")
}

func (api *privetAPI) capabilities(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /capabilities request: %+v", r)
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	printer, exists := api.getPrinter(api.name)
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	capabilities := cdd.CloudDeviceDescription{
		Version: "1.0",
		Printer: printer.Description,
	}
	j, err := json.MarshalIndent(capabilities, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal capabilities response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(j)
	}
}

func (api *privetAPI) createjob(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /createjob request: %+v", r)
	if ok := api.checkRequest(w, r, "POST"); !ok {
		return
	}

	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Warningf("Failed to read request body: %s", err)
		writeError(w, "invalid_ticket", "Check connector logs")
		return
	}

	var ticket cdd.CloudJobTicket
	if err = json.Unmarshal(requestBody, &ticket); err != nil {
		log.Warningf("Failed to read request body: %s", err)
		writeError(w, "invalid_ticket", "Check connector logs")
		return
	}

	printer, exists := api.getPrinter(api.name)
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if printer.State.State == cdd.CloudDeviceStateStopped {
		writeError(w, "printer_error", "Printer is stopped")
		return
	}

	jobID, expiresIn := api.jc.createJob(&ticket)
	var response struct {
		JobID     string `json:"job_id"`
		ExpiresIn int32  `json:"expires_in"`
	}
	response.JobID = jobID
	response.ExpiresIn = expiresIn
	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		api.jc.deleteJob(jobID)
		log.Errorf("Failed to marrshal createJob response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(j)
	}
}

func (api *privetAPI) submitdoc(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /submitdoc request: %+v", r)
	if ok := api.checkRequest(w, r, "POST"); !ok {
		return
	}

	file, err := ioutil.TempFile("", "cloud-print-connector-privet-")
	if err != nil {
		log.Errorf("Failed to create file for new Privet job: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	jobSize, err := io.Copy(file, r.Body)
	if err != nil {
		log.Errorf("Failed to copy new print job file: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		os.Remove(file.Name())
		return
	}
	if length, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64); err != nil || length != jobSize {
		writeError(w, "invalid_params", "Content-Length header doesn't match length of content")
		os.Remove(file.Name())
		return
	}

	jobType := r.Header.Get("Content-Type")
	if jobType == "" {
		writeError(w, "invalid_document_type", "Content-Type header is missing")
		os.Remove(file.Name())
		return
	}

	printer, exists := api.getPrinter(api.name)
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		os.Remove(file.Name())
		return
	}
	if printer.State.State == cdd.CloudDeviceStateStopped {
		writeError(w, "printer_error", "Printer is stopped")
		os.Remove(file.Name())
		return
	}

	jobName := r.Form.Get("job_name")
	userName := r.Form.Get("user_name")
	jobID := r.Form.Get("job_id")
	var expiresIn int32
	var ticket *cdd.CloudJobTicket
	if jobID == "" {
		jobID, expiresIn = api.jc.createJob(nil)
	} else {
		var ok bool
		if expiresIn, ticket, ok = api.jc.getJobExpiresIn(jobID); !ok {
			pe := privetError{
				Error:   "invalid_print_job",
				Timeout: 5,
			}.json()
			w.Write(pe)
			os.Remove(file.Name())
			return
		}
	}

	api.jobs <- &lib.Job{
		NativePrinterName: api.name,
		Filename:          file.Name(),
		Title:             jobName,
		User:              userName,
		JobID:             jobID,
		Ticket:            ticket,
		UpdateJob:         api.jc.updateJob,
	}

	var response struct {
		JobID     string `json:"job_id"`
		ExpiresIn int32  `json:"expires_in"`
		JobType   string `json:"job_type"`
		JobSize   int64  `json:"job_size"`
		JobName   string `json:"job_name,omitempty"`
	}

	response.JobID = jobID
	response.ExpiresIn = expiresIn
	response.JobType = jobType
	response.JobSize = jobSize
	response.JobName = jobName
	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.ErrorJobf(jobID, "Failed to marshal submitdoc response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) jobstate(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Received /jobstate request: %+v", r)
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	jobID := r.Form.Get("job_id")
	jobState, exists := api.jc.jobState(jobID)
	if !exists {
		writeError(w, "invalid_print_job", "")
		return
	}

	w.Write(jobState)
}
