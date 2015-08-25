/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"os"
	"sync"

	"github.com/google/cups-connector/lib"
)

// Privet managers local discovery and printing.
type Privet struct {
	xsrf      xsrfSecret
	apis      map[string]*privetAPI
	apisMutex sync.RWMutex // Protects apis
	zc        *zeroconf

	jobs chan *lib.Job
	jc   jobCache

	gcpBaseURL        string
	getProximityToken func(string, string) ([]byte, int, error)
	createTempFile    func() (*os.File, error)
}

// NewPrivet constructs a new Privet object.
//
// getProximityToken should be GoogleCloudPrint.ProximityToken()
// createTempFile should be cups.CreateTempFile()
func NewPrivet(gcpBaseURL string, getProximityToken func(string, string) ([]byte, int, error), createTempFile func() (*os.File, error)) (*Privet, error) {
	zc, err := newZeroconf()
	if err != nil {
		return nil, err
	}

	p := Privet{
		xsrf: newXSRFSecret(),
		apis: make(map[string]*privetAPI),
		zc:   zc,

		jobs: make(chan *lib.Job, 10),
		jc:   *newJobCache(),

		gcpBaseURL:        gcpBaseURL,
		getProximityToken: getProximityToken,
		createTempFile:    createTempFile,
	}

	return &p, nil
}

// AddPrinter makes a printer available locally.
func (p *Privet) AddPrinter(printer lib.Printer, getPrinter func() (lib.Printer, bool)) error {
	getProximityToken := func(user string) ([]byte, int, error) { return p.getProximityToken(printer.GCPID, user) }
	api, err := newPrivetAPI(printer.GCPID, p.gcpBaseURL, p.xsrf, &p.jc, p.jobs, getPrinter, getProximityToken, p.createTempFile)
	if err != nil {
		return err
	}

	online := false
	if printer.GCPID != "" {
		online = true
	}

	var LocalDefaultDisplayName = printer.DefaultDisplayName
	# TODO once we add local-only support we should hide the append behind an if ! local-only
	LocalDefaultDisplayName = fmt.Sprint(LocalDefaultDisplayName, " (local)")
	err = p.zc.addPrinter(printer.GCPID, printer.Name, api.port(), LocalDefaultDisplayName, p.gcpBaseURL, printer.GCPID, online)
	if err != nil {
		api.quit()
		return err
	}

	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	p.apis[printer.GCPID] = api

	return nil
}

// UpdatePrinter updates a printer's TXT mDNS record.
func (p *Privet) UpdatePrinter(diff *lib.PrinterDiff) error {
	// API never needs to be updated

	online := false
	if diff.Printer.GCPID != "" {
		online = true
	}
	return p.zc.updatePrinterTXT(diff.Printer.GCPID, diff.Printer.DefaultDisplayName, p.gcpBaseURL, diff.Printer.GCPID, online)
}

// DeletePrinter removes a printer from Privet.
func (p *Privet) DeletePrinter(gcpID string) error {
	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	err := p.zc.removePrinter(gcpID)
	if api, ok := p.apis[gcpID]; ok {
		api.quit()
		delete(p.apis, gcpID)
	}
	return err
}

// Jobs returns a channel that emits new print jobs.
func (p *Privet) Jobs() <-chan *lib.Job {
	return p.jobs
}

func (p *Privet) Quit() {
	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	p.zc.quit()
	for gcpID, api := range p.apis {
		api.quit()
		delete(p.apis, gcpID)
	}
}
