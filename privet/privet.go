/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"fmt"
	"sync"

	"github.com/google/cups-connector/lib"
)

// Privet managers local discovery and printing.
type Privet struct {
	xsrf      xsrfSecret
	apis      map[string]*privetAPI
	apisMutex sync.RWMutex // Protects apis
	zc        *zeroconf

	jobs chan<- *lib.Job
	jc   jobCache

	gcpBaseURL        string
	getProximityToken func(string, string) ([]byte, int, error)
}

// NewPrivet constructs a new Privet object.
//
// getProximityToken should be GoogleCloudPrint.ProximityToken()
func NewPrivet(jobs chan<- *lib.Job, gcpBaseURL string, getProximityToken func(string, string) ([]byte, int, error)) (*Privet, error) {
	zc, err := newZeroconf()
	if err != nil {
		return nil, err
	}

	p := Privet{
		xsrf: newXSRFSecret(),
		apis: make(map[string]*privetAPI),
		zc:   zc,

		jobs: jobs,
		jc:   *newJobCache(),

		gcpBaseURL:        gcpBaseURL,
		getProximityToken: getProximityToken,
	}

	return &p, nil
}

// TODO move getPrinter to NewPrivet.
// AddPrinter makes a printer available locally.
func (p *Privet) AddPrinter(printer lib.Printer, getPrinter func(string) (lib.Printer, bool)) error {
	online := false
	if printer.GCPID != "" {
		online = true
	}

	api, err := newPrivetAPI(printer.GCPID, printer.Name, p.gcpBaseURL, p.xsrf, online, &p.jc, p.jobs, getPrinter, p.getProximityToken)
	if err != nil {
		return err
	}

	var localDefaultDisplayName = printer.DefaultDisplayName
	if online {
		localDefaultDisplayName = fmt.Sprintf("%s (local)", localDefaultDisplayName)
	}
	err = p.zc.addPrinter(printer.Name, api.port(), localDefaultDisplayName, p.gcpBaseURL, printer.GCPID, online)
	if err != nil {
		api.quit()
		return err
	}

	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	p.apis[printer.Name] = api

	return nil
}

// UpdatePrinter updates a printer's TXT mDNS record.
func (p *Privet) UpdatePrinter(diff *lib.PrinterDiff) error {
	// API never needs to be updated

	online := false
	if diff.Printer.GCPID != "" {
		online = true
	}

	var localDefaultDisplayName = diff.Printer.DefaultDisplayName
	if online {
		localDefaultDisplayName = fmt.Sprintf("%s (local)", localDefaultDisplayName)
	}

	return p.zc.updatePrinterTXT(diff.Printer.GCPID, localDefaultDisplayName, p.gcpBaseURL, diff.Printer.GCPID, online)
}

// DeletePrinter removes a printer from Privet.
func (p *Privet) DeletePrinter(cupsPrinterName string) error {
	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	err := p.zc.removePrinter(cupsPrinterName)
	if api, ok := p.apis[cupsPrinterName]; ok {
		api.quit()
		delete(p.apis, cupsPrinterName)
	}
	return err
}

func (p *Privet) Quit() {
	p.apisMutex.Lock()
	defer p.apisMutex.Unlock()

	p.zc.quit()
	for cupsPrinterName, api := range p.apis {
		api.quit()
		delete(p.apis, cupsPrinterName)
	}
}
