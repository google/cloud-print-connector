/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import (
	"reflect"
	"regexp"

	"github.com/google/cloud-print-connector/cdd"
)

type PrinterState uint8

// DuplexVendorMap maps a DuplexType to a CUPS key:value option string for a given printer.
type DuplexVendorMap map[cdd.DuplexType]string

// CUPS: cups_dest_t; GCP: /register and /update interfaces
type Printer struct {
	GCPID              string                         //                                    GCP: printerid (GCP key)
	Name               string                         // CUPS: cups_dest_t.name (CUPS key); GCP: name field
	DefaultDisplayName string                         // CUPS: printer-info;                GCP: default_display_name field
	UUID               string                         // CUPS: printer-uuid;                GCP: uuid field
	Manufacturer       string                         // CUPS: PPD;                         GCP: manufacturer field
	Model              string                         // CUPS: PPD;                         GCP: model field
	GCPVersion         string                         //                                    GCP: gcpVersion field
	SetupURL           string                         //                                    GCP: setup_url field
	SupportURL         string                         //                                    GCP: support_url field
	UpdateURL          string                         //                                    GCP: update_url field
	ConnectorVersion   string                         //                                    GCP: firmware field
	State              *cdd.PrinterStateSection       // CUPS: various;                     GCP: semantic_state field
	Description        *cdd.PrinterDescriptionSection // CUPS: translated PPD;              GCP: capabilities field
	CapsHash           string                         // CUPS: hash of PPD;                 GCP: capsHash field
	Tags               map[string]string              // CUPS: all printer attributes;      GCP: repeated tag field
	DuplexMap          DuplexVendorMap                // CUPS: PPD;
	NativeJobSemaphore *Semaphore
	QuotaEnabled       bool
	DailyQuota         int
}

var rDeviceURIHostname *regexp.Regexp = regexp.MustCompile(
	"(?i)^(?:socket|http|https|ipp|ipps|lpd)://([a-z][a-z0-9.-]*)")

// GetHostname gets the network hostname, parsed from Printer.Tags["device-uri"].
func (p *Printer) GetHostname() (string, bool) {
	deviceURI, ok := p.Tags["device-uri"]
	if !ok {
		return "", false
	}

	parts := rDeviceURIHostname.FindStringSubmatch(deviceURI)
	if len(parts) == 2 {
		return parts[1], true
	}

	return "", false
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
	ManufacturerChanged       bool
	ModelChanged              bool
	GCPVersionChanged         bool
	SetupURLChanged           bool
	SupportURLChanged         bool
	UpdateURLChanged          bool
	ConnectorVersionChanged   bool
	StateChanged              bool
	DescriptionChanged        bool
	CapsHashChanged           bool
	TagsChanged               bool
	DuplexMapChanged          bool
	QuotaEnabledChanged       bool
	DailyQuotaChanged         bool
}

func printerSliceToMapByName(s []Printer) map[string]Printer {
	m := make(map[string]Printer, len(s))
	for i := range s {
		m[s[i].Name] = s[i]
	}
	return m
}

// DiffPrinters returns the diff between old (GCP) and new (native) printers.
// Returns nil if zero printers or if all diffs are NoChangeToPrinter operation.
func DiffPrinters(nativePrinters, gcpPrinters []Printer) []PrinterDiff {
	// So far, no changes.
	dirty := false

	diffs := make([]PrinterDiff, 0, 1)
	printersConsidered := make(map[string]struct{}, len(nativePrinters))
	nativePrintersByName := printerSliceToMapByName(nativePrinters)

	for i := range gcpPrinters {
		if _, exists := printersConsidered[gcpPrinters[i].Name]; exists {
			// GCP can have multiple printers with one name. Remove dupes.
			diffs = append(diffs, PrinterDiff{Operation: DeletePrinter, Printer: gcpPrinters[i]})
			dirty = true

		} else {
			printersConsidered[gcpPrinters[i].Name] = struct{}{}

			if nativePrinter, exists := nativePrintersByName[gcpPrinters[i].Name]; exists {
				// Native printer doesn't know about GCPID yet.
				nativePrinter.GCPID = gcpPrinters[i].GCPID
				// Don't lose track of this semaphore.
				nativePrinter.NativeJobSemaphore = gcpPrinters[i].NativeJobSemaphore

				diff := diffPrinter(&nativePrinter, &gcpPrinters[i])
				diffs = append(diffs, diff)

				if diff.Operation != NoChangeToPrinter {
					dirty = true
				}

			} else {
				diffs = append(diffs, PrinterDiff{Operation: DeletePrinter, Printer: gcpPrinters[i]})
				dirty = true
			}
		}
	}

	for i := range nativePrinters {
		if _, exists := printersConsidered[nativePrinters[i].Name]; !exists {
			diffs = append(diffs, PrinterDiff{Operation: RegisterPrinter, Printer: nativePrinters[i]})
			dirty = true
		}
	}

	if dirty {
		return diffs
	} else {
		return nil
	}
}

// diffPrinter finds the difference between a native printer and the corresponding GCP printer.
//
// pn: printer-native; the thing that is correct
//
// pg: printer-GCP; the thing that will be updated
func diffPrinter(pn, pg *Printer) PrinterDiff {
	d := PrinterDiff{
		Operation: UpdatePrinter,
		Printer:   *pn,
	}

	if pg.DefaultDisplayName != pn.DefaultDisplayName {
		d.DefaultDisplayNameChanged = true
	}
	if pg.Manufacturer != pn.Manufacturer {
		d.ManufacturerChanged = true
	}
	if pg.Model != pn.Model {
		d.ModelChanged = true
	}
	if pg.GCPVersion != pn.GCPVersion {
		if pg.GCPVersion > pn.GCPVersion {
			panic("GCP version cannot be downgraded; delete GCP printers")
		}
		d.GCPVersionChanged = true
	}
	if pg.SetupURL != pn.SetupURL {
		d.SetupURLChanged = true
	}
	if pg.SupportURL != pn.SupportURL {
		d.SupportURLChanged = true
	}
	if pg.UpdateURL != pn.UpdateURL {
		d.UpdateURLChanged = true
	}
	if pg.ConnectorVersion != pn.ConnectorVersion {
		d.ConnectorVersionChanged = true
	}
	if !reflect.DeepEqual(pg.State, pn.State) {
		d.StateChanged = true
	}
	if !reflect.DeepEqual(pg.Description, pn.Description) {
		d.DescriptionChanged = true
	}
	if pg.CapsHash != pn.CapsHash {
		d.CapsHashChanged = true
	}

	gcpTagshash, gcpHasTagshash := pg.Tags["tagshash"]
	nativeTagshash, nativeHasTagshash := pn.Tags["tagshash"]
	if !gcpHasTagshash || !nativeHasTagshash || gcpTagshash != nativeTagshash {
		d.TagsChanged = true
	}

	if !reflect.DeepEqual(pg.DuplexMap, pn.DuplexMap) {
		d.DuplexMapChanged = true
	}

	if pg.QuotaEnabled != pn.QuotaEnabled {
		d.QuotaEnabledChanged = true
	}

	if pg.DailyQuota != pn.DailyQuota {
		d.DailyQuotaChanged = true
	}

	if d.DefaultDisplayNameChanged || d.ManufacturerChanged || d.ModelChanged ||
		d.GCPVersionChanged || d.SetupURLChanged || d.SupportURLChanged ||
		d.UpdateURLChanged || d.ConnectorVersionChanged || d.StateChanged ||
		d.DescriptionChanged || d.CapsHashChanged || d.TagsChanged ||
		d.DuplexMapChanged || d.QuotaEnabledChanged || d.DailyQuotaChanged {
		return d
	}

	return PrinterDiff{
		Operation: NoChangeToPrinter,
		Printer:   *pg,
	}
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
	if printer.Tags["printer-make-and-model"] == "Local Raw Printer" {
		return true
	}
	return false
}

func PrinterIsClass(printer Printer) bool {
	if printer.Tags["printer-make-and-model"] == "Local Printer Class" {
		return true
	}
	return false
}
