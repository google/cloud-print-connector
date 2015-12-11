/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package winspool

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"
)

// Interface between Go and the Windows API.
type WinSpool struct {
	prefixJobIDToJobTitle bool
	displayNamePrefix     string
}

func NewWinSpool(prefixJobIDToJobTitle bool, displayNamePrefix string) (*WinSpool, error) {
	// TODO: Give meaning to both args.
	return &WinSpool{prefixJobIDToJobTitle, displayNamePrefix}, nil
}

func convertPrinterState(wsStatus uint32) *cdd.PrinterStateSection {
	state := cdd.PrinterStateSection{
		State:       cdd.CloudDeviceStateIdle,
		VendorState: &cdd.VendorState{},
	}

	if wsStatus&(PRINTER_STATUS_PRINTING|PRINTER_STATUS_PROCESSING) != 0 {
		state.State = cdd.CloudDeviceStateProcessing
	}

	if wsStatus&PRINTER_STATUS_PAUSED != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateWarning,
			DescriptionLocalized: cdd.NewLocalizedString("printer paused"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_ERROR != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("printer error"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_PENDING_DELETION != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("printer is being deleted"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_PAPER_JAM != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("paper jam"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_PAPER_OUT != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("paper out"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_MANUAL_FEED != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("manual feed mode"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_PAPER_PROBLEM != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("paper problem"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_OFFLINE != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("printer is offline"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_IO_ACTIVE != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("active I/O state"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_BUSY != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("busy"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_OUTPUT_BIN_FULL != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("output bin is full"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_NOT_AVAILABLE != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("printer not available"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_WAITING != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("waiting"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_INITIALIZING != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("intitializing"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_WARMING_UP != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("warming up"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_TONER_LOW != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateWarning,
			DescriptionLocalized: cdd.NewLocalizedString("toner low"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_NO_TONER != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("no toner"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_PAGE_PUNT != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("cannot print the current page"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_USER_INTERVENTION != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("user intervention required"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_OUT_OF_MEMORY != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("out of memory"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_DOOR_OPEN != 0 {
		state.State = cdd.CloudDeviceStateStopped
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("door open"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_SERVER_UNKNOWN != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateError,
			DescriptionLocalized: cdd.NewLocalizedString("printer status unknown"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}
	if wsStatus&PRINTER_STATUS_POWER_SAVE != 0 {
		vs := cdd.VendorStateItem{
			State:                cdd.VendorStateInfo,
			DescriptionLocalized: cdd.NewLocalizedString("power save mode"),
		}
		state.VendorState.Item = append(state.VendorState.Item, vs)
	}

	if len(state.VendorState.Item) == 0 {
		state.VendorState = nil
	}

	return &state
}

// GetPrinters gets all Windows printers found on this computer.
func (c *WinSpool) GetPrinters() ([]lib.Printer, error) {
	pi2s, err := EnumPrinters2()
	if err != nil {
		return nil, err
	}

	printers := make([]lib.Printer, 0, len(pi2s))
	for _, pi2 := range pi2s {
		printerName := pi2.PrinterName()
		portName := pi2.PortName()
		devMode := pi2.DevMode()

		printer := lib.Printer{
			Name:        printerName,
			State:       convertPrinterState(pi2.Status()),
			Description: &cdd.PrinterDescriptionSection{},
		}

		if def, ok := devMode.GetDuplex(); ok {
			duplex, err := DeviceCapabilitiesInt32(printerName, portName, DC_DUPLEX)
			if err != nil {
				return nil, err
			}
			if duplex == 1 {
				printer.Description.Duplex = &cdd.Duplex{
					Option: []cdd.DuplexOption{
						cdd.DuplexOption{
							Type:      cdd.DuplexNoDuplex,
							IsDefault: def == DMDUP_SIMPLEX,
						},
						cdd.DuplexOption{
							Type:      cdd.DuplexLongEdge,
							IsDefault: def == DMDUP_VERTICAL,
						},
						cdd.DuplexOption{
							Type:      cdd.DuplexShortEdge,
							IsDefault: def == DMDUP_HORIZONTAL,
						},
					},
				}
			}
		}

		if def, ok := devMode.GetOrientation(); ok {
			orientation, err := DeviceCapabilitiesInt32(printerName, portName, DC_ORIENTATION)
			if err != nil {
				return nil, err
			}
			if orientation == 90 || orientation == 270 {
				printer.Description.PageOrientation = &cdd.PageOrientation{
					Option: []cdd.PageOrientationOption{
						cdd.PageOrientationOption{
							Type:      cdd.PageOrientationPortrait,
							IsDefault: def == DMORIENT_PORTRAIT,
						},
						cdd.PageOrientationOption{
							Type:      cdd.PageOrientationLandscape,
							IsDefault: def == DMORIENT_LANDSCAPE,
						},
					},
				}
			}
		}

		if def, ok := devMode.GetCopies(); ok {
			copies, err := DeviceCapabilitiesInt32(printerName, portName, DC_COPIES)
			if err != nil {
				return nil, err
			}
			if copies > 1 {
				printer.Description.Copies = &cdd.Copies{
					Default: int32(def),
					Max:     copies,
				}
			}
		}

		printer.Description.FitToPage = &cdd.FitToPage{
			Option: []cdd.FitToPageOption{
				cdd.FitToPageOption{
					Type:      cdd.FitToPageNoFitting,
					IsDefault: true,
				},
				cdd.FitToPageOption{
					Type:      cdd.FitToPageFitToPage,
					IsDefault: false,
				},
			},
		}

		printer.Description.MediaSize, err = convertMediaSize(printerName, portName, devMode)
		if err != nil {
			return nil, err
		}

		if def, ok := devMode.GetCollate(); ok {
			collate, err := DeviceCapabilitiesInt32(printerName, portName, DC_COLLATE)
			if err != nil {
				return nil, err
			}
			if collate == 1 {
				printer.Description.Collate = &cdd.Collate{
					Default: def == DMCOLLATE_TRUE,
				}
			}
		}

		printers = append(printers, printer)
	}

	return printers, nil
}

func convertMediaSize(printerName, portName string, devMode *DevMode) (*cdd.MediaSize, error) {
	defSize, defSizeOK := devMode.GetPaperSize()
	defLength, defLengthOK := devMode.GetPaperLength()
	defWidth, defWidthOK := devMode.GetPaperWidth()

	names, err := DeviceCapabilitiesStrings(printerName, portName, DC_PAPERNAMES, 64*2)
	if err != nil {
		return nil, err
	}
	papers, err := DeviceCapabilitiesUint16Array(printerName, portName, DC_PAPERS)
	if err != nil {
		return nil, err
	}
	sizes, err := DeviceCapabilitiesInt32Pairs(printerName, portName, DC_PAPERSIZE)
	if err != nil {
		return nil, err
	}
	if len(names) != len(papers) || len(names) != len(sizes)/2 {
		return nil, nil
	}

	ms := cdd.MediaSize{
		Option: make([]cdd.MediaSizeOption, 0, len(names)),
	}

	var foundDef bool
	for i := range names {
		if names[i] == "" {
			continue
		}
		width, length := sizes[2*i], sizes[2*i+1]

		var def bool
		if !foundDef {
			if defSizeOK {
				if uint16(defSize) == papers[i] {
					def = true
					foundDef = true
				}
			} else if defLengthOK && int32(defLength) == length && defWidthOK && int32(defWidth) == width {
				def = true
				foundDef = true
			}
		}

		o := cdd.MediaSizeOption{
			Name:                       cdd.MediaSizeCustom,
			WidthMicrons:               width,
			HeightMicrons:              length,
			IsDefault:                  def,
			VendorID:                   strconv.FormatUint(uint64(papers[i]), 10),
			CustomDisplayNameLocalized: cdd.NewLocalizedString(names[i]),
		}
		ms.Option = append(ms.Option, o)
	}

	if !foundDef && len(ms.Option) > 0 {
		ms.Option[0].IsDefault = true
	}

	return &ms, nil
}

func convertJobState(wsStatus uint32) *cdd.JobState {
	var state cdd.JobState

	if wsStatus&(JOB_STATUS_SPOOLING|JOB_STATUS_PRINTING) != 0 {
		state.Type = cdd.JobStateInProgress

	} else if wsStatus&(JOB_STATUS_PRINTED|JOB_STATUS_COMPLETE) != 0 {
		state.Type = cdd.JobStateDone

	} else if wsStatus&JOB_STATUS_PAUSED != 0 {
		state.Type = cdd.JobStateStopped
		state.UserActionCause = &cdd.UserActionCause{cdd.UserActionCausePaused}

	} else if wsStatus&JOB_STATUS_ERROR != 0 {
		state.Type = cdd.JobStateAborted
		state.DeviceActionCause = &cdd.DeviceActionCause{cdd.DeviceActionCausePrintFailure}

	} else if wsStatus&(JOB_STATUS_DELETING|JOB_STATUS_DELETED) != 0 {
		state.Type = cdd.JobStateAborted
		state.UserActionCause = &cdd.UserActionCause{cdd.UserActionCauseCanceled}

	} else if wsStatus&(JOB_STATUS_OFFLINE|JOB_STATUS_PAPEROUT|JOB_STATUS_BLOCKED_DEVQ|JOB_STATUS_USER_INTERVENTION) != 0 {
		state.Type = cdd.JobStateStopped
		state.DeviceStateCause = &cdd.DeviceStateCause{cdd.DeviceStateCauseOther}

	} else {
		// Don't know what is going on. Get the job out of our queue.
		state.Type = cdd.JobStateAborted
	}

	return &state
}

// GetJobState gets the current state of the job indicated by jobID.
func (c *WinSpool) GetJobState(printerName string, jobID uint32) (*cdd.PrintJobStateDiff, error) {
	hPrinter, err := OpenPrinter(printerName)
	if err != nil {
		return nil, err
	}

	ji1, err := hPrinter.GetJob(int32(jobID))
	if err != nil {
		if err == ERROR_INVALID_PARAMETER {
			jobState := cdd.PrintJobStateDiff{
				State: &cdd.JobState{
					Type: cdd.JobStateAborted,
				},
			}
			return &jobState, nil
		}
		return nil, err
	}

	jobState := cdd.PrintJobStateDiff{
		State: convertJobState(ji1.GetStatus()),
	}
	return &jobState, nil
}

type jobContext struct {
	pDoc     PopplerDocument
	hPrinter HANDLE
	devMode  *DevMode
	hDC      HDC
	cSurface CairoSurface
	cContext CairoContext
}

func newJobContext(printerName, fileName, title string) (int32, *jobContext, error) {
	pDoc, err := PopplerDocumentNewFromFile(fileName)
	if err != nil {
		return 0, nil, err
	}
	hPrinter, err := OpenPrinter(printerName)
	if err != nil {
		pDoc.Unref()
		return 0, nil, err
	}
	devMode, err := hPrinter.DocumentPropertiesGet(printerName)
	if err != nil {
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	err = hPrinter.DocumentPropertiesSet(printerName, devMode)
	if err != nil {
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	hDC, err := CreateDC(devMode.GetDeviceName(), devMode)
	if err != nil {
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	jobID, err := hDC.StartDoc(title)
	if err != nil {
		hDC.DeleteDC()
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	cSurface, err := CairoWin32PrintingSurfaceCreate(hDC)
	if err != nil {
		hDC.EndDoc()
		hDC.DeleteDC()
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	cContext, err := CairoCreateContext(cSurface)
	if err != nil {
		cSurface.Destroy()
		hDC.EndDoc()
		hDC.DeleteDC()
		hPrinter.ClosePrinter()
		pDoc.Unref()
		return 0, nil, err
	}
	c := jobContext{pDoc, hPrinter, devMode, hDC, cSurface, cContext}
	return jobID, &c, nil
}

func (c *jobContext) free() error {
	var err error
	err = c.cContext.Destroy()
	if err != nil {
		return err
	}
	err = c.cSurface.Destroy()
	if err != nil {
		return err
	}
	err = c.hDC.EndDoc()
	if err != nil {
		return err
	}
	err = c.hDC.DeleteDC()
	if err != nil {
		return err
	}
	err = c.hPrinter.ClosePrinter()
	if err != nil {
		return err
	}
	c.pDoc.Unref()
	return nil
}

func getScaleAndOffset(wDocPoints, hDocPoints float64, wPaperPixels, hPaperPixels, xMarginPixels, yMarginPixels, wPrintablePixels, hPrintablePixels, xDPI, yDPI int32, fitToPage bool) (scale, xOffsetPoints, yOffsetPoints float64) {

	wPaperPoints, hPaperPoints := float64(wPaperPixels*72)/float64(xDPI), float64(hPaperPixels*72)/float64(yDPI)

	var wPrintablePoints, hPrintablePoints float64
	if fitToPage {
		wPrintablePoints, hPrintablePoints = float64(wPrintablePixels*72)/float64(xDPI), float64(hPrintablePixels*72)/float64(yDPI)
	} else {
		wPrintablePoints, hPrintablePoints = wPaperPoints, hPaperPoints
	}

	xScale, yScale := wPrintablePoints/wDocPoints, hPrintablePoints/hDocPoints
	if xScale < yScale {
		scale = xScale
	} else {
		scale = yScale
	}

	xOffsetPoints = (wPaperPoints - wDocPoints*scale) / 2
	yOffsetPoints = (hPaperPoints - hDocPoints*scale) / 2

	return
}

func printPage(printerName string, i int, c *jobContext, fitToPage bool) error {
	pPage := c.pDoc.GetPage(i)
	defer pPage.Unref()

	if err := c.hPrinter.DocumentPropertiesSet(printerName, c.devMode); err != nil {
		return err
	}

	if err := c.hDC.ResetDC(c.devMode); err != nil {
		return err
	}

	// Set device to zero offset, and to points scale.
	xDPI := c.hDC.GetDeviceCaps(LOGPIXELSX)
	yDPI := c.hDC.GetDeviceCaps(LOGPIXELSY)
	xMarginPixels := c.hDC.GetDeviceCaps(PHYSICALOFFSETX)
	yMarginPixels := c.hDC.GetDeviceCaps(PHYSICALOFFSETY)
	xform := NewXFORM(float32(xDPI)/72, float32(yDPI)/72, float32(-xMarginPixels), float32(-yMarginPixels))
	if err := c.hDC.SetGraphicsMode(GM_ADVANCED); err != nil {
		return err
	}
	if err := c.hDC.SetWorldTransform(xform); err != nil {
		return err
	}

	if err := c.hDC.StartPage(); err != nil {
		return err
	}
	defer c.hDC.EndPage()

	if err := c.cContext.Save(); err != nil {
		return err
	}

	wPaperPixels := c.hDC.GetDeviceCaps(PHYSICALWIDTH)
	hPaperPixels := c.hDC.GetDeviceCaps(PHYSICALHEIGHT)
	wPrintablePixels := c.hDC.GetDeviceCaps(HORZRES)
	hPrintablePixels := c.hDC.GetDeviceCaps(VERTRES)

	wDocPoints, hDocPoints, err := pPage.GetSize()
	if err != nil {
		return err
	}

	scale, xOffsetPoints, yOffsetPoints := getScaleAndOffset(wDocPoints, hDocPoints, wPaperPixels, hPaperPixels, xMarginPixels, yMarginPixels, wPrintablePixels, hPrintablePixels, xDPI, yDPI, fitToPage)

	if err := c.cContext.IdentityMatrix(); err != nil {
		return err
	}
	if err := c.cContext.Translate(xOffsetPoints, yOffsetPoints); err != nil {
		return err
	}
	if err := c.cContext.Scale(scale, scale); err != nil {
		return err
	}

	pPage.RenderForPrinting(c.cContext)

	if err := c.cContext.Restore(); err != nil {
		return err
	}
	if err := c.cSurface.ShowPage(); err != nil {
		return err
	}

	return nil
}

var (
	duplexValueByType = map[cdd.DuplexType]int16{
		cdd.DuplexNoDuplex:  DMDUP_SIMPLEX,
		cdd.DuplexLongEdge:  DMDUP_VERTICAL,
		cdd.DuplexShortEdge: DMDUP_HORIZONTAL,
	}

	pageOrientationByType = map[cdd.PageOrientationType]int16{
		cdd.PageOrientationPortrait:  DMORIENT_PORTRAIT,
		cdd.PageOrientationLandscape: DMORIENT_LANDSCAPE,
		// Ignore cdd.PageOrientationAuto for ticket parsing, in order to interpret "auto".
	}
)

// Print sends a new print job to the specified printer. The job ID
// is returned.
func (c *WinSpool) Print(printer *lib.Printer, fileName, title, user, gcpJobID string, ticket *cdd.CloudJobTicket) (uint32, error) {
	if printer == nil {
		return 0, errors.New("Print() called with nil printer")
	}
	if ticket == nil {
		return 0, errors.New("Print() called with nil ticket")
	}

	jobID, jobContext, err := newJobContext(printer.Name, fileName, title)
	if err != nil {
		return 0, err
	}
	defer jobContext.free()

	if ticket.Print.Duplex != nil && printer.Description.Duplex != nil {
		if duplex, ok := duplexValueByType[ticket.Print.Duplex.Type]; ok {
			jobContext.devMode.SetDuplex(duplex)
		}
	}

	if ticket.Print.PageOrientation != nil && printer.Description.PageOrientation != nil {
		if pageOrientation, ok := pageOrientationByType[ticket.Print.PageOrientation.Type]; ok {
			jobContext.devMode.SetOrientation(pageOrientation)
		}
	}

	if ticket.Print.Copies != nil && printer.Description.Copies != nil {
		if ticket.Print.Copies.Copies > 0 {
			jobContext.devMode.SetCopies(int16(ticket.Print.Copies.Copies))
		}
	}

	var fitToPage bool
	if ticket.Print.FitToPage != nil && printer.Description.FitToPage != nil {
		if ticket.Print.FitToPage.Type == cdd.FitToPageFitToPage {
			fitToPage = true
		}
	}

	if ticket.Print.MediaSize != nil && printer.Description.MediaSize != nil {
		if ticket.Print.MediaSize.VendorID != "" {
			v, err := strconv.ParseInt(ticket.Print.MediaSize.VendorID, 10, 16)
			if err != nil {
				fmt.Println("houston, problem", err)
				return 0, err
			}
			jobContext.devMode.SetPaperSize(int16(v))
			jobContext.devMode.ClearPaperLength()
			jobContext.devMode.ClearPaperWidth()
		} else {
			jobContext.devMode.ClearPaperSize()
			jobContext.devMode.SetPaperLength(int16(ticket.Print.MediaSize.HeightMicrons / 10))
			jobContext.devMode.SetPaperWidth(int16(ticket.Print.MediaSize.WidthMicrons / 10))
		}
	}

	if ticket.Print.Collate != nil && printer.Description.Collate != nil {
		if ticket.Print.Collate.Collate {
			jobContext.devMode.SetCollate(DMCOLLATE_TRUE)
		} else {
			jobContext.devMode.SetCollate(DMCOLLATE_FALSE)
		}
	}

	for i := 0; i < jobContext.pDoc.GetNPages(); i++ {
		if err := printPage(printer.Name, i, jobContext, fitToPage); err != nil {
			return 0, err
		}
	}

	return uint32(jobID), nil
}

// The following functions are not relevant to Windows printing, but are required by the NativePrintSystem interface.

func (w *WinSpool) Quit()                              {}
func (c *WinSpool) RemoveCachedPPD(printerName string) {}
