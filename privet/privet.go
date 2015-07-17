/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"fmt"
	"os"
	"sync"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"
)

// Privet managers local discovery and printing.
type Privet struct {
	xsrf      xsrfSecret
	apis      map[string]*privetAPI
	zeroconfs map[string]*bonjour
	azMutex   sync.RWMutex // Protects apis and zeroconfs.

	jobs chan *lib.Job
	jc   jobCache

	gcpBaseURL        string
	getProximityToken func(string, string) (*cdd.ProximityToken, error)
	createTempFile    func() (*os.File, error)
}

// NewPrivet constructs a new Privet object.
//
// getProximityToken should be GoogleCloudPrint.ProximityToken()
// createTempFile should be cups.CreateTempFile()
func NewPrivet(gcpBaseURL string, getProximityToken func(string, string) (*cdd.ProximityToken, error), createTempFile func() (*os.File, error)) (*Privet, error) {
	p := Privet{
		xsrf:      newXSRFSecret(),
		apis:      make(map[string]*privetAPI),
		zeroconfs: make(map[string]*bonjour),

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
	getProximityToken := func(user string) (*cdd.ProximityToken, error) { return p.getProximityToken(printer.GCPID, user) }
	api, err := newPrivetAPI(printer.GCPID, p.gcpBaseURL, p.xsrf, &p.jc, p.jobs, getPrinter, getProximityToken, p.createTempFile)
	if err != nil {
		return err
	}

	online := false
	if printer.GCPID != "" {
		online = true
	}
	zc, err := newZeroconf(printer.Name, "_privet._tcp", api.port(), printer.DefaultDisplayName, p.gcpBaseURL, printer.GCPID, online)
	if err != nil {
		api.quit()
		return err
	}

	p.azMutex.Lock()
	defer p.azMutex.Unlock()

	p.apis[printer.GCPID] = api
	p.zeroconfs[printer.GCPID] = zc

	return nil
}

// UpdatePrinter updates a printer's TXT mDNS record.
func (p *Privet) UpdatePrinter(diff *lib.PrinterDiff) {
	// API never needs to be updated
	// Only update zeroconf when the ty field (lib.Printer.DefaultDisplayName) changes.
	if !diff.DefaultDisplayNameChanged {
		return
	}

	p.azMutex.RLock()
	defer p.azMutex.RUnlock()

	online := false
	if diff.Printer.GCPID != "" {
		online = true
	}
	if zc, ok := p.zeroconfs[diff.Printer.GCPID]; ok {
		zc.updateTXT(diff.Printer.DefaultDisplayName, p.gcpBaseURL, diff.Printer.GCPID, online)
	}
}

// DeletePrinter removes a printer from Privet.
func (p *Privet) DeletePrinter(gcpID string) {
	fmt.Println("called DeletePrinter", gcpID)
	p.azMutex.Lock()
	defer p.azMutex.Unlock()

	if zc, ok := p.zeroconfs[gcpID]; ok {
		zc.quit()
		delete(p.zeroconfs, gcpID)
	} else {
		fmt.Println("couldn't find printer", gcpID)
	}
	if api, ok := p.apis[gcpID]; ok {
		api.quit()
		delete(p.apis, gcpID)
	}
}

// Jobs returns a channel that emits new print jobs.
func (p *Privet) Jobs() <-chan *lib.Job {
	return p.jobs
}

func (p *Privet) Quit() {
	p.azMutex.Lock()
	defer p.azMutex.Unlock()

	for gcpID, zc := range p.zeroconfs {
		zc.quit()
		delete(p.zeroconfs, gcpID)
	}
	for gcpID, api := range p.apis {
		api.quit()
		delete(p.apis, gcpID)
	}
}
