/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package snmp

/*
#cgo LDFLAGS: -lnetsnmp
#include <net-snmp/net-snmp-config.h>
#include <net-snmp/net-snmp-includes.h>
#include "snmp.h"
*/
import "C"
import (
	"errors"
	"reflect"
	"sync"
	"unsafe"

	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/snmp/oid"
)

type SNMPManager struct {
	inUse          *lib.Semaphore
	community      *C.char
	maxConnections uint
}

// NewSNMPManager creates a new SNMP manager.
func NewSNMPManager(community string, maxConnections uint) (*SNMPManager, error) {
	C.initialize()
	s := SNMPManager{
		inUse:          lib.NewSemaphore(1),
		community:      C.CString(community),
		maxConnections: maxConnections,
	}
	return &s, nil
}

func (s *SNMPManager) Quit() {
	s.inUse.Acquire()
	C.free(unsafe.Pointer(s.community))
}

func intArrayToOID(cOID *C.oid, cLength C.size_t) oid.OID {
	length := int(cLength)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cOID)),
		Len:  length,
		Cap:  length,
	}

	o := make([]uint, length)
	for i, digit := range *(*[]C.oid)(unsafe.Pointer(&hdr)) {
		o[i] = uint(digit)
	}

	return o
}

func charArrayToSlice(cArr **C.char, cLength C.size_t) []*C.char {
	length := int(cLength)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cArr)),
		Len:  length,
		Cap:  length,
	}
	return *(*[]*C.char)(unsafe.Pointer(&hdr))
}

// getPrinters gets all printer SNMP information for each hostname, concurrently.
func (s *SNMPManager) getPrinters(hostnames []string) (map[string]*oid.VariableSet, error) {
	if !s.inUse.TryAcquire() {
		return nil, errors.New("Tried to query printers via SNMP twice")
	}
	defer s.inUse.Release()

	wg := sync.WaitGroup{}
	wg.Add(len(hostnames))

	results := make(map[string]*oid.VariableSet, len(hostnames))
	for _, hostname := range hostnames {
		results[hostname] = &oid.VariableSet{}
	}

	semaphore := lib.NewSemaphore(s.maxConnections)

	for _, hostname := range hostnames {
		go func(hostname string) {
			r := results[hostname]
			h := C.CString(hostname)
			defer C.free(unsafe.Pointer(h))

			semaphore.Acquire()
			defer semaphore.Release()

			response := C.bulkwalk(h, s.community)
			for o := response.ov_root; o != nil; o = o.next {
				r.AddVariable(intArrayToOID((*o).name, (*o).name_length), C.GoString((*o).value))
				defer C.free(unsafe.Pointer((*o).name))
				defer C.free(unsafe.Pointer((*o).value))
				defer C.free(unsafe.Pointer(o))
			}
			if response.errors_len > 0 {
				for _, err := range charArrayToSlice(response.errors, response.errors_len) {
					// Ignore errors. Not all printers support SNMP, so this is best effort.
					C.free(unsafe.Pointer(err))
				}
				C.free(unsafe.Pointer(response.errors))
			}
			wg.Done()
		}(hostname)
	}

	wg.Wait()

	return results, nil
}

// AugmentPrinters queries every printer's SNMP agent, adds anything it
// finds back to the printer object.
func (s *SNMPManager) AugmentPrinters(printers []lib.Printer) error {
	hostnames := make([]string, 0, len(printers))
	for _, printer := range printers {
		if hostname, exists := printer.GetHostname(); exists {
			hostnames = append(hostnames, hostname)
		}
	}

	varsByHostname, err := s.getPrinters(hostnames)
	if err != nil {
		return err
	}

	for i := range printers {
		hostname, ok := printers[i].GetHostname()
		if !ok {
			continue
		}
		vars, ok := varsByHostname[hostname]
		if !ok {
			continue
		}
		if serialNumber, ok := vars.GetSerialNumber(); ok {
			printers[i].UUID = serialNumber
		}
		if covers, coverState, exists := vars.GetCovers(); exists {
			printers[i].State.CoverState = coverState
			printers[i].Description.Cover = covers
		}
		if inputTrayUnits, inputTrayState, exists := vars.GetInputTrays(); exists {
			printers[i].State.InputTrayState = inputTrayState
			printers[i].Description.InputTrayUnit = inputTrayUnits
		}
		if outputBinUnits, outputBinState, exists := vars.GetOutputBins(); exists {
			printers[i].State.OutputBinState = outputBinState
			printers[i].Description.OutputBinUnit = outputBinUnits
		}
		if markers, markerState, vendorState, exists := vars.GetMarkers(); exists {
			if len(*markers) > 0 && len(markerState.Item) > 0 {
				printers[i].State.MarkerState = markerState
				printers[i].Description.Marker = markers
			}
			if len(vendorState.Item) > 0 {
				printers[i].State.VendorState = vendorState
			}
		}
	}

	return nil
}
