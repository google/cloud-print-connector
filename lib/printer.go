/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

import (
	"reflect"
	"strings"
)

type PrinterStatus string

func PrinterStatusFromString(ss string) PrinterStatus {
	switch strings.ToLower(ss) {
	case "3", "idle":
		return PrinterIdle
	case "4", "processing":
		return PrinterProcessing
	case "5", "stopped":
		return PrinterStopped
	default:
		return PrinterIdle
	}
}

// CUPS: ipp_pstate_t; GCP: CloudDeviceState.StateType
const (
	PrinterIdle       PrinterStatus = "3" // CUPS: IPP_PSTATE_IDLE;       GCP: StateType.IDLE
	PrinterProcessing PrinterStatus = "4" // CUPS: IPP_PSTATE_PROCESSING; GCP: StateType.PROCESSING
	PrinterStopped    PrinterStatus = "5" // CUPS: IPP_PSTATE_STOPPED;    GCP: StateType.STOPPED
)

// CUPS: cups_dest_t; GCP: /register and /update interfaces
type Printer struct {
	GCPID              string            // CUPS: custom field;                GCP: printerid (GCP key)
	Name               string            // CUPS: cups_dest_t.name (CUPS key); GCP: name field
	DefaultDisplayName string            // CUPS: printer-info option;         GCP: default_display_name field
	Description        string            // CUPS: printer-make-and-model;      GCP: description field
	Status             PrinterStatus     // CUPS: printer-state;               GCP: status field
	CapsHash           string            // CUPS: hash of PPD;                 GCP: capsHash field
	Tags               map[string]string // CUPS: all printer attributes;      GCP: repeated tag field
	XMPPTimeout        uint32            //                                    GCP: local_settings/xmpp_timeout_value field
	CUPSJobSemaphore   *Semaphore
}

type PrinterDiffOperation int8

const (
	RegisterPrinter PrinterDiffOperation = iota
	UpdatePrinter
	DeletePrinter
	NoChangeToPrinter
)

// Describes changes to be pushed to a GCP printer.
type PrinterDiff struct {
	Operation PrinterDiffOperation
	Printer   Printer

	DefaultDisplayNameChanged bool
	DescriptionChanged        bool
	StatusChanged             bool
	CapsHashChanged           bool
	XMPPTimeoutChanged        bool
	TagsChanged               bool
}

func printerSliceToMapByName(s []Printer) map[string]Printer {
	m := make(map[string]Printer, len(s))
	for i := range s {
		m[s[i].Name] = s[i]
	}
	return m
}

// DiffPrinters returns the diff between old (GCP) and new (CUPS) printers.
// Returns nil if zero printers or if all diffs are NoChangeToPrinter operation.
func DiffPrinters(cupsPrinters, gcpPrinters []Printer) []PrinterDiff {
	// So far, no changes.
	dirty := false

	diffs := make([]PrinterDiff, 0, 1)
	printersConsidered := make(map[string]bool, len(cupsPrinters))
	cupsPrintersByName := printerSliceToMapByName(cupsPrinters)

	for i := range gcpPrinters {
		if printersConsidered[gcpPrinters[i].Name] {
			// GCP can have multiple printers with one name. Remove dupes.
			diffs = append(diffs, PrinterDiff{Operation: DeletePrinter, Printer: gcpPrinters[i]})
			dirty = true

		} else {
			printersConsidered[gcpPrinters[i].Name] = true

			if cupsPrinter, exists := cupsPrintersByName[gcpPrinters[i].Name]; exists {
				cupsPrinter.GCPID = gcpPrinters[i].GCPID
				cupsPrinter.CUPSJobSemaphore = gcpPrinters[i].CUPSJobSemaphore

				if reflect.DeepEqual(cupsPrinter, gcpPrinters[i]) {
					diffs = append(diffs, PrinterDiff{Operation: NoChangeToPrinter, Printer: gcpPrinters[i]})

				} else {
					diffs = append(diffs, diffPrinter(&cupsPrinter, &gcpPrinters[i]))
					dirty = true
				}

			} else {
				diffs = append(diffs, PrinterDiff{Operation: DeletePrinter, Printer: gcpPrinters[i]})
				dirty = true
			}
		}
	}

	for i := range cupsPrinters {
		if !printersConsidered[cupsPrinters[i].Name] {
			diffs = append(diffs, PrinterDiff{Operation: RegisterPrinter, Printer: cupsPrinters[i]})
			dirty = true
		}
	}

	if dirty {
		return diffs
	} else {
		return nil
	}
}

// diffPrinter finds the difference between a CUPS printer and the corresponding GCP printer.
//
// pc: printer-CUPS; the thing that is correct
//
// pg: printer-GCP; the thing that will be updated
func diffPrinter(pc, pg *Printer) PrinterDiff {
	d := PrinterDiff{
		Operation: UpdatePrinter,
		Printer:   *pc,
	}

	// Do not lose track of this semaphore.
	d.Printer.CUPSJobSemaphore = pg.CUPSJobSemaphore

	if pg.DefaultDisplayName != pc.DefaultDisplayName {
		d.DefaultDisplayNameChanged = true
	}
	if pg.Description != pc.Description {
		d.DescriptionChanged = true
	}
	if pg.Status != pc.Status {
		d.StatusChanged = true
	}
	if pg.CapsHash != pc.CapsHash {
		d.CapsHashChanged = true
	}
	if pg.XMPPTimeout != pc.XMPPTimeout {
		d.XMPPTimeoutChanged = true
	}
	if !reflect.DeepEqual(pg.Tags, pc.Tags) {
		d.TagsChanged = true
	}

	return d
}

// FilterRawPrinters splits a slice of printers into non-raw and raw.
func FilterRawPrinters(printers []Printer) ([]Printer, []Printer) {
	notRaw, raw := make([]Printer, 0, len(printers)), make([]Printer, 0, 0)
	for i := range printers {
		if PrinterIsRaw(printers[i]) {
			raw = append(raw, printers[i])
		} else {
			notRaw = append(notRaw, printers[i])
		}
	}
	return notRaw, raw
}

func PrinterIsRaw(printer Printer) bool {
	if printer.Description == "Local Raw Printer" {
		return true
	}
	return false
}
