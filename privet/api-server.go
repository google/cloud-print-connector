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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"
)

var (
	missingPrivetToken = []byte("Missing X-Privet-Token header")
	closed             = errors.New("closed")
	supportedAPIs      = []string{
		"/privet/accesstoken",
		"/privet/capabilities",
		"/privet/printer/createjob",
		"/privet/printer/submitdoc",
		"/privet/printer/jobstate",
	}
)

// TODO: return proper errors per GCP Privet docs
type privetAPI struct {
	gcpID      string
	gcpBaseURL string
	xsrf       xsrfSecret
	jc         *jobCache
	jobs       chan<- *lib.Job

	getPrinter        func() (lib.Printer, bool)
	getProximityToken func(string) (*cdd.ProximityToken, error)
	createTempFile    func() (*os.File, error)

	listener  *quittableListener
	startTime time.Time
}

func newPrivetAPI(gcpID, gcpBaseURL string, xsrf xsrfSecret, jc *jobCache, jobs chan<- *lib.Job, getPrinter func() (lib.Printer, bool), getProximityToken func(string) (*cdd.ProximityToken, error), createTempFile func() (*os.File, error)) (*privetAPI, error) {
	l, err := newQuittableListener()
	if err != nil {
		return nil, err
	}
	api := &privetAPI{
		gcpID:      gcpID,
		gcpBaseURL: gcpBaseURL,
		xsrf:       xsrf,
		jc:         jc,
		jobs:       jobs,

		getPrinter:        getPrinter,
		getProximityToken: getProximityToken,
		createTempFile:    createTempFile,

		listener:  l,
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
	sm.HandleFunc("/privet/accesstoken", api.accesstoken)
	sm.HandleFunc("/privet/capabilities", api.capabilities)
	sm.HandleFunc("/privet/printer/createjob", api.createjob)
	sm.HandleFunc("/privet/printer/submitdoc", api.submitdoc)
	sm.HandleFunc("/privet/printer/jobstate", api.jobstate)

	err := http.Serve(api.listener, sm)
	if err != nil && err != closed {
		glog.Errorf("Privet API HTTP server failed: %s", err)
	}
}

type infoResponse struct {
	Version         string               `json:"version"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	URL             string               `json:"url"`
	Type            []string             `json:"type"`
	ID              string               `json:"id"`
	DeviceState     string               `json:"device_state"`
	ConnectionState string               `json:"connection_state"`
	Manufacturer    string               `json:"manufacturer"`
	Model           string               `json:"model"`
	SerialNumber    string               `json:"serial_number"`
	Firmware        string               `json:"firmware"`
	Uptime          uint                 `json:"uptime"`
	SetupURL        string               `json:"setup_url"`
	SupportURL      string               `json:"support_url"`
	UpdateURL       string               `json:"update_url"`
	XPrivetToken    string               `json:"x-privet-token"`
	API             []string             `json:"api"`
	SemanticState   cdd.CloudDeviceState `json:"semantic_state"`
}

func (api *privetAPI) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, exists := r.Header["X-Privet-Token"]; !exists {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(missingPrivetToken)
		return
	}

	printer, exists := api.getPrinter()
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s := cdd.CloudConnectionStateOnline
	state := cdd.CloudDeviceState{
		Version:              "1.0",
		CloudConnectionState: &s,
		Printer:              printer.State,
	}

	response := infoResponse{
		Version:         "1.0",
		Name:            printer.Name,
		URL:             api.gcpBaseURL,
		Type:            []string{"printer"},
		ID:              printer.GCPID,
		DeviceState:     strings.ToLower(string(printer.State.State)),
		ConnectionState: "online",
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
		glog.Errorf("Failed to marshal Privet info: %s", err)
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
		w.Write(missingPrivetToken)
		return false
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func (api *privetAPI) accesstoken(w http.ResponseWriter, r *http.Request) {
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	user := r.Form.Get("user")
	proximityToken, err := api.getProximityToken(user)
	if err != nil {
		glog.Errorf("Failed to get proximity token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	j, err := json.MarshalIndent(proximityToken, "", "  ")
	if err != nil {
		glog.Errorf("Failed to marshal proximity token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) capabilities(w http.ResponseWriter, r *http.Request) {
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	printer, exists := api.getPrinter()
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	capabilities := cdd.CloudDeviceDescription{
		Version: "1.0",
		Printer: printer.Description,
	}
	j, err := json.MarshalIndent(capabilities, "", "  ")
	if err != nil {
		glog.Errorf("Failed to marshal capabilities response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) createjob(w http.ResponseWriter, r *http.Request) {
	if ok := api.checkRequest(w, r, "POST"); !ok {
		return
	}

	jobID, expiresIn := api.jc.createJob(api.gcpID, nil)
	var response struct {
		JobID     string `json:"job_id"`
		ExpiresIn int32  `json:"expires_in"`
	}
	response.JobID = jobID
	response.ExpiresIn = expiresIn
	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		glog.Errorf("Failed to createJob response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) submitdoc(w http.ResponseWriter, r *http.Request) {
	if ok := api.checkRequest(w, r, "POST"); !ok {
		return
	}

	file, err := api.createTempFile()
	if err != nil {
		glog.Errorf("Failed to create file for new Privet job: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jobSize, err := io.Copy(file, r.Body)
	if err != nil {
		glog.Errorf("Failed to copy new print job file: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		file.Close()
		os.Remove(file.Name())
		return
	}
	if length, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64); err != nil || length != jobSize {
		w.WriteHeader(http.StatusBadRequest)
		file.Close()
		os.Remove(file.Name())
		return
	}

	jobType := r.Header.Get("Content-Type")
	if jobType == "" {
		w.WriteHeader(http.StatusBadRequest)
		file.Close()
		os.Remove(file.Name())
		return
	}

	jobName := r.Form.Get("job_name")
	userName := r.Form.Get("user_name")
	jobID := r.Form.Get("job_id")
	var expiresIn int32
	var ticket *cdd.CloudJobTicket
	if jobID == "" {
		jobID, expiresIn = api.jc.createJob(api.gcpID, nil)
	} else {
		var ok bool
		if expiresIn, ticket, ok = api.jc.getJobExpiresIn(jobID); !ok {
			w.WriteHeader(http.StatusBadRequest)
			file.Close()
			os.Remove(file.Name())
			return
		}
	}

	api.jobs <- &lib.Job{
		GCPPrinterID: api.gcpID,
		Filename:     file.Name(),
		Title:        jobName,
		User:         userName,
		JobID:        jobID,
		Ticket:       ticket,
		UpdateJob:    api.jc.updateJob,
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
		glog.Errorf("Failed to marshal submitdoc response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func (api *privetAPI) jobstate(w http.ResponseWriter, r *http.Request) {
	if ok := api.checkRequest(w, r, "GET"); !ok {
		return
	}

	jobID := r.Form.Get("job_id")
	jobState, exists := api.jc.jobState(jobID)
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Write(jobState)
}

type quittableListener struct {
	*net.TCPListener
	// When q is closed, the listener is quitting.
	q chan struct{}
}

func newQuittableListener() (*quittableListener, error) {
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to start Privet API listener: %s", err)
	}
	return &quittableListener{l, make(chan struct{}, 0)}, nil
}

func (l *quittableListener) Accept() (net.Conn, error) {
	conn, err := l.AcceptTCP()

	select {
	case <-l.q:
		if err == nil {
			conn.Close()
		}
		// The listener was closed on purpose.
		// Returning an error that is not a net.Error causes net.Server.Serve() to return.
		return nil, closed
	default:
	}

	// Clean up zombie connections.
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Minute)

	return conn, err
}

func (l *quittableListener) quit() {
	close(l.q)
	l.Close()
}
