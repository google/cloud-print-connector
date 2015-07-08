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
	"net"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/google/cups-connector/cdd"
)

var missingPrivetToken = []byte("Missing X-Privet-Token header")

var closed error = errors.New("closed")

type PrivetAPI struct {
	listener  *QuittableListener
	xsrf      xsrfSecret
	startTime time.Time
}

func NewPrivetAPI(x xsrfSecret) (*PrivetAPI, error) {
	l, err := NewQuittableListener()
	if err != nil {
		return nil, err
	}
	api := &PrivetAPI{
		listener:  l,
		xsrf:      x,
		startTime: time.Now(),
	}
	go api.serve()

	return api, nil
}

func (api *PrivetAPI) Port() uint16 {
	return uint16(api.listener.Addr().(*net.TCPAddr).Port)
}

func (api *PrivetAPI) Quit() {
	api.listener.Quit()
}

func (api *PrivetAPI) serve() {
	sm := http.NewServeMux()
	sm.HandleFunc("/privet/info", api.info)

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

func (api *PrivetAPI) info(w http.ResponseWriter, r *http.Request) {
	if _, exists := r.Header["X-Privet-Token"]; !exists {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(missingPrivetToken)
		return
	}

	response := infoResponse{
		Version:      "1.0",
		Uptime:       uint(time.Since(api.startTime).Seconds()),
		XPrivetToken: api.xsrf.newToken(),
	}

	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		glog.Errorf("Failed to marshal Privet info: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

type QuittableListener struct {
	*net.TCPListener
	// When q is closed, the listener is quitting.
	q chan struct{}
}

func NewQuittableListener() (*QuittableListener, error) {
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to start Privet API listener: %s", err)
	}
	return &QuittableListener{l, make(chan struct{}, 0)}, nil
}

func (l *QuittableListener) Accept() (net.Conn, error) {
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

func (l *QuittableListener) Quit() {
	close(l.q)
	l.Close()
}
