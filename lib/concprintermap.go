/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import "sync"

// ConcurrentPrinterMap is a map-like data structure that is also
// thread-safe. Printers are keyed by Printer.Name and Printer.GCPID.
type ConcurrentPrinterMap struct {
	byNativeName map[string]Printer
	byGCPID      map[string]Printer
	mutex        sync.RWMutex
}

// NewConcurrentPrinterMap initializes an empty ConcurrentPrinterMap.
func NewConcurrentPrinterMap(printers []Printer) *ConcurrentPrinterMap {
	cpm := ConcurrentPrinterMap{}
	// TODO will this fail on nil?
	cpm.Refresh(printers)
	return &cpm
}

// Refresh replaces the internal (non-concurrent) map with newPrinters.
func (cpm *ConcurrentPrinterMap) Refresh(newPrinters []Printer) {
	c := make(map[string]Printer, len(newPrinters))
	for _, printer := range newPrinters {
		c[printer.Name] = printer
	}

	g := make(map[string]Printer, len(newPrinters))
	for _, printer := range newPrinters {
		if len(printer.GCPID) > 0 {
			g[printer.GCPID] = printer
		}
	}

	cpm.mutex.Lock()
	defer cpm.mutex.Unlock()

	cpm.byNativeName = c
	cpm.byGCPID = g
}

// GetByNativeName gets gets a printer, using the native name as key.
//
// The second return value is true if the entry exists.
func (cpm *ConcurrentPrinterMap) GetByNativeName(name string) (Printer, bool) {
	cpm.mutex.RLock()
	defer cpm.mutex.RUnlock()

	if p, exists := cpm.byNativeName[name]; exists {
		return p, true
	}
	return Printer{}, false
}

// GetByGCPID gets gets a printer, using the GCP ID as key.
//
// The second return value is true if the entry exists.
func (cpm *ConcurrentPrinterMap) GetByGCPID(gcpID string) (Printer, bool) {
	cpm.mutex.RLock()
	defer cpm.mutex.RUnlock()

	if p, exists := cpm.byGCPID[gcpID]; exists {
		return p, true
	}
	return Printer{}, false
}

// GetAll returns a slice of all printers.
func (cpm *ConcurrentPrinterMap) GetAll() []Printer {
	cpm.mutex.RLock()
	defer cpm.mutex.RUnlock()

	printers := make([]Printer, len(cpm.byNativeName))
	i := 0
	for _, printer := range cpm.byNativeName {
		printers[i] = printer
		i++
	}

	return printers
}
