package lib

import "strings"

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
	GCPID              string        // CUPS: custom field;                GCP: printerid (GCP key)
	Name               string        // CUPS: cups_dest_t.name (CUPS key); GCP: name field
	DefaultDisplayName string        // CUPS: printer-info option;         GCP: default_display_name field
	Description        string        // CUPS: printer-make-and-model;      GCP: description field
	Status             PrinterStatus // CUPS: printer-state;               GCP: status field
	CapsHash           string        // CUPS: hash of PPD;                 GCP: capsHash field
	Location           string        // CUPS: printer-location option;     GCP: custom tag
}

type PrinterDiffOperation int8

const (
	RegisterPrinter PrinterDiffOperation = iota
	UpdatePrinter
	DeletePrinter
	LeavePrinter
)

// Describes changes to be pushed to a GCP printer.
type PrinterDiff struct {
	Operation PrinterDiffOperation
	Printer   Printer // Only GCPID, Name, and changed fields are filled.
}

func printerSliceToMapByName(s []Printer) map[string]Printer {
	m := make(map[string]Printer, len(s))
	for _, p := range s {
		m[p.Name] = p
	}
	return m
}

// Returns the diff between old (GCP) and new (CUPS) printers.
// Returns nil if zero printers or if all diffs are LeavePrinter operation.
func DiffPrinters(cupsPrinters, gcpPrinters []Printer) []PrinterDiff {
	// So far, no changes.
	dirty := false

	diffs := make([]PrinterDiff, 0, 1)
	printersConsidered := make(map[string]bool, len(cupsPrinters))
	cupsPrintersByName := printerSliceToMapByName(cupsPrinters)

	for _, gcpPrinter := range gcpPrinters {
		if printersConsidered[gcpPrinter.Name] {
			// GCP can have multiple printers with one name. Remove dupes.
			diffs = append(diffs, PrinterDiff{DeletePrinter, gcpPrinter})
			dirty = true

		} else {
			printersConsidered[gcpPrinter.Name] = true

			if cupsPrinter, exists := cupsPrintersByName[gcpPrinter.Name]; exists {
				cupsPrinter.GCPID = gcpPrinter.GCPID
				if cupsPrinter == gcpPrinter {
					diffs = append(diffs, PrinterDiff{LeavePrinter, gcpPrinter})

				} else {
					diffs = append(diffs, diffPrinter(&cupsPrinter, &gcpPrinter))
					dirty = true
				}

			} else {
				diffs = append(diffs, PrinterDiff{DeletePrinter, gcpPrinter})
				dirty = true
			}
		}
	}

	for _, cupsPrinter := range cupsPrinters {
		if !printersConsidered[cupsPrinter.Name] {
			diffs = append(diffs, PrinterDiff{RegisterPrinter, cupsPrinter})
			dirty = true
		}
	}

	if dirty {
		return diffs
	} else {
		return nil
	}
}

// Find the difference between a CUPS printer and the corresponding GCP printer.
// pc: printer-CUPS; the thing that is correct
// pg: printer-GCP; the thing that will be updated
func diffPrinter(pc, pg *Printer) PrinterDiff {
	// pd: printer-diff; the thing that fixes pg.
	// Name and GCPID are immutable keys, so don't diff them.
	pd := Printer{
		GCPID: pg.GCPID,
		Name:  pg.Name,
	}
	if pg.DefaultDisplayName != pc.DefaultDisplayName {
		pd.DefaultDisplayName = pc.DefaultDisplayName
	}
	if pg.Description != pc.Description {
		pd.Description = pc.Description
	}
	if pg.Status != pc.Status {
		pd.Status = pc.Status
	}
	if pg.CapsHash != pc.CapsHash {
		pd.CapsHash = pc.CapsHash
	}
	if pg.Location != pc.Location {
		pd.Location = pc.Location
	}

	return PrinterDiff{
		Operation: UpdatePrinter,
		Printer:   pd,
	}
}
