/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

import "sync"

// ConcurrentPrinterMap is a map-like data structure that is also
// thread-safe. Printers are keyed by Printer.GCPID.
type ConcurrentPrinterMap struct {
	printers map[string]Printer
	mutex    sync.RWMutex
}

// NewConcurrentPrinterMap initializes an empty ConcurrentPrinterMap.
func NewConcurrentPrinterMap(printers []Printer) *ConcurrentPrinterMap {
	cpm := ConcurrentPrinterMap{}
	cpm.Refresh(printers)
	return &cpm
}

// Refresh replaces the internal (non-concurrent) map with newPrinters.
func (cpm *ConcurrentPrinterMap) Refresh(newPrinters []Printer) {
	m := make(map[string]Printer, len(newPrinters))
	for _, printer := range newPrinters {
		m[printer.GCPID] = printer
	}

	cpm.mutex.Lock()
	defer cpm.mutex.Unlock()

	cpm.printers = m
}

// Get gets a printer from the map.
//
// The second return value is true if the entry exists.
func (cpm *ConcurrentPrinterMap) Get(gcpID string) (Printer, bool) {
	cpm.mutex.RLock()
	defer cpm.mutex.RUnlock()

	if p, exists := cpm.printers[gcpID]; exists {
		return p, true
	}
	return Printer{}, false
}

// GetAll returns a slice of all printers.
func (cpm *ConcurrentPrinterMap) GetAll() []Printer {
	cpm.mutex.RLock()
	defer cpm.mutex.RUnlock()

	printers := make([]Printer, len(cpm.printers))
	i := 0
	for _, printer := range cpm.printers {
		printers[i] = printer
		i++
	}

	return printers
}
