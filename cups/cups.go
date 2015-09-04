/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package cups

/*
#cgo LDFLAGS: -lcups
#include "cups.h"
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"

	"github.com/golang/glog"
)

const (
	// CUPS "URL" length are always less than 40. For example: /job/1234567
	urlMaxLength = 100

	attrDeviceURI               = "device-uri"
	attrDocumentFormatSupported = "document-format-supported"
	attrMarkerLevels            = "marker-levels"
	attrMarkerNames             = "marker-names"
	attrMarkerTypes             = "marker-types"
	attrPrinterInfo             = "printer-info"
	attrPrinterMakeAndModel     = "printer-make-and-model"
	attrPrinterName             = "printer-name"
	attrPrinterState            = "printer-state"
	attrPrinterStateReasons     = "printer-state-reasons"
	attrPrinterUUID             = "printer-uuid"

	attrJobState                = "job-state"
	attrJobMediaSheetsCompleted = "job-media-sheets-completed"
)

var (
	requiredPrinterAttributes []string = []string{
		attrDeviceURI,
		attrDocumentFormatSupported,
		attrMarkerLevels,
		attrMarkerNames,
		attrMarkerTypes,
		attrPrinterInfo,
		attrPrinterMakeAndModel,
		attrPrinterName,
		attrPrinterState,
		attrPrinterStateReasons,
		attrPrinterUUID,
	}

	jobAttributes []string = []string{
		attrJobState,
		attrJobMediaSheetsCompleted,
	}

	numberUpCapability = cdd.VendorCapability{
		ID:   "number-up",
		Type: cdd.VendorCapabilitySelect,
		SelectCap: &cdd.SelectCapability{
			Option: []cdd.SelectCapabilityOption{
				cdd.SelectCapabilityOption{
					Value:                "1",
					IsDefault:            true,
					DisplayNameLocalized: cdd.NewLocalizedString("1"),
				},
				cdd.SelectCapabilityOption{
					Value:                "2",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("2"),
				},
				cdd.SelectCapabilityOption{
					Value:                "4",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("4"),
				},
				cdd.SelectCapabilityOption{
					Value:                "6",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("6"),
				},
				cdd.SelectCapabilityOption{
					Value:                "9",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("9"),
				},
				cdd.SelectCapabilityOption{
					Value:                "16",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("16"),
				},
			},
		},
		DisplayNameLocalized: cdd.NewLocalizedString("Pages per sheet"),
	}
)

// Interface between Go and the CUPS API.
type CUPS struct {
	cc                *cupsCore
	pc                *ppdCache
	infoToDisplayName bool
	printerAttributes []string
	systemTags        map[string]string
}

func NewCUPS(infoToDisplayName bool, printerAttributes []string, maxConnections uint, connectTimeout time.Duration) (*CUPS, error) {
	if err := checkPrinterAttributes(printerAttributes); err != nil {
		return nil, err
	}

	cc, err := newCUPSCore(maxConnections, connectTimeout)
	if err != nil {
		return nil, err
	}
	pc := newPPDCache(cc)

	systemTags, err := getSystemTags()
	if err != nil {
		return nil, err
	}

	c := &CUPS{
		cc:                cc,
		pc:                pc,
		infoToDisplayName: infoToDisplayName,
		printerAttributes: printerAttributes,
		systemTags:        systemTags,
	}

	return c, nil
}

func (c *CUPS) Quit() {
	c.pc.quit()
}

// ConnQtyOpen gets the current quantity of open CUPS connections.
func (c *CUPS) ConnQtyOpen() uint {
	return c.cc.connQtyOpen()
}

// ConnQtyOpen gets the maximum quantity of open CUPS connections.
func (c *CUPS) ConnQtyMax() uint {
	return c.cc.connQtyMax()
}

// GetPrinters gets all CUPS printers found on the CUPS server.
func (c *CUPS) GetPrinters() ([]lib.Printer, error) {
	pa := C.newArrayOfStrings(C.int(len(c.printerAttributes)))
	defer C.freeStringArrayAndStrings(pa, C.int(len(c.printerAttributes)))
	for i, a := range c.printerAttributes {
		C.setStringArrayValue(pa, C.int(i), C.CString(a))
	}

	response, err := c.cc.getPrinters(pa, C.int(len(c.printerAttributes)))
	if err != nil {
		return nil, err
	}

	// cupsDoRequest() returns ipp_t pointer which needs explicit free.
	defer C.ippDelete(response)

	if C.getIPPRequestStatusCode(response) == C.IPP_STATUS_ERROR_NOT_FOUND {
		// Normal error when there are no printers.
		return make([]lib.Printer, 0), nil
	}

	printers := c.responseToPrinters(response)
	for i := range printers {
		printers[i].GCPVersion = lib.GCPAPIVersion
		printers[i].ConnectorVersion = lib.ShortName
		printers[i].SetupURL = lib.ConnectorHomeURL
		printers[i].SupportURL = lib.ConnectorHomeURL
		printers[i].UpdateURL = lib.ConnectorHomeURL
	}
	printers = c.addDescriptionToPrinters(printers)

	return printers, nil
}

// responseToPrinters converts a C.ipp_t to a slice of lib.Printers.
func (c *CUPS) responseToPrinters(response *C.ipp_t) []lib.Printer {
	printers := make([]lib.Printer, 0, 1)

	for a := response.attrs; a != nil; a = a.next {
		if a.group_tag != C.IPP_TAG_PRINTER {
			continue
		}

		attributes := make([]*C.ipp_attribute_t, 0, C.int(len(c.printerAttributes)))
		for ; a != nil && a.group_tag == C.IPP_TAG_PRINTER; a = a.next {
			attributes = append(attributes, a)
		}
		tags := attributesToTags(attributes)
		p := tagsToPrinter(tags, c.systemTags, c.infoToDisplayName)

		printers = append(printers, p)
		if a == nil {
			break
		}
	}

	return printers
}

// addDescriptionToPrinters fetches description, PPD hash, manufacturer, model
// for argument printers, concurrently. These are the fields derived from PPD.
//
// Returns a new printer slice, because it can shrink due to raw or
// mis-configured printers.
func (c *CUPS) addDescriptionToPrinters(printers []lib.Printer) []lib.Printer {
	var wg sync.WaitGroup
	ch := make(chan *lib.Printer, len(printers))

	for i := range printers {
		if !lib.PrinterIsRaw(printers[i]) {
			wg.Add(1)
			go func(p *lib.Printer) {
				if description, ppdHash, manufacturer, model, err := c.pc.getPPDCacheEntry(p.Name); err == nil {
					p.Description.Absorb(description)
					p.CapsHash = ppdHash
					p.Manufacturer = manufacturer
					p.Model = model
					ch <- p
				} else {
					glog.Error(err)
				}
				wg.Done()
			}(&printers[i])
		}
	}

	wg.Wait()
	close(ch)

	result := make([]lib.Printer, 0, len(ch))
	for printer := range ch {
		result = append(result, *printer)
	}

	return result
}

func getSystemTags() (map[string]string, error) {
	tags := make(map[string]string)

	tags["connector-version"] = lib.BuildDate
	hostname, err := os.Hostname()
	if err == nil {
		tags["system-hostname"] = hostname
	}
	tags["system-arch"] = runtime.GOARCH
	tags["system-golang-version"] = runtime.Version()

	sysname, nodename, release, version, machine, err := uname()
	if err != nil {
		return nil, fmt.Errorf("CUPS failed to call uname while initializing: %s", err)
	}

	tags["system-uname-sysname"] = sysname
	tags["system-uname-nodename"] = nodename
	tags["system-uname-release"] = release
	tags["system-uname-version"] = version
	tags["system-uname-machine"] = machine

	tags["connector-cups-client-version"] = fmt.Sprintf("%d.%d.%d",
		C.CUPS_VERSION_MAJOR, C.CUPS_VERSION_MINOR, C.CUPS_VERSION_PATCH)

	return tags, nil
}

// RemoveCachedPPD removes a printer's PPD from the cache.
func (c *CUPS) RemoveCachedPPD(printername string) {
	c.pc.removePPD(printername)
}

// GetJobState gets the current state of the job indicated by jobID.
func (c *CUPS) GetJobState(jobID uint32) (cdd.PrintJobStateDiff, error) {
	ja := C.newArrayOfStrings(C.int(len(jobAttributes)))
	defer C.freeStringArrayAndStrings(ja, C.int(len(jobAttributes)))
	for i, attribute := range jobAttributes {
		C.setStringArrayValue(ja, C.int(i), C.CString(attribute))
	}

	response, err := c.cc.getJobAttributes(C.int(jobID), ja)
	if err != nil {
		return cdd.PrintJobStateDiff{}, err
	}

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(response)

	s := C.ippFindAttribute(response, C.JOB_STATE, C.IPP_TAG_ENUM)
	state := int32(C.getAttributeIntegerValue(s, C.int(0)))

	p := C.ippFindAttribute(response, C.JOB_MEDIA_SHEETS_COMPLETED, C.IPP_TAG_INTEGER)
	var pages int32
	if p != nil {
		pages = int32(C.getAttributeIntegerValue(p, C.int(0)))
	}

	return convertJobState(state, pages), nil
}

// convertJobState converts CUPS job state to cdd.PrintJobStateDiff.
func convertJobState(cupsState, pages int32) cdd.PrintJobStateDiff {
	state := cdd.PrintJobStateDiff{PagesPrinted: &pages}

	switch cupsState {
	case 3, 4, 5: // PENDING, HELD, PROCESSING
		state.State = &cdd.JobState{Type: cdd.JobStateInProgress}
	case 6: // STOPPED
		state.State = &cdd.JobState{
			Type:              cdd.JobStateStopped,
			DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCauseOther},
		}
	case 7: // CANCELED
		state.State = &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		}
	case 8: // ABORTED
		state.State = &cdd.JobState{
			Type:              cdd.JobStateAborted,
			DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCausePrintFailure},
		}
	case 9: // COMPLETED
		state.State = &cdd.JobState{Type: cdd.JobStateDone}
	}

	return state
}

// Print sends a new print job to the specified printer. The job ID
// is returned.
func (c *CUPS) Print(printername, filename, title, user string, ticket *cdd.CloudJobTicket) (uint32, error) {
	pn := C.CString(printername)
	defer C.free(unsafe.Pointer(pn))
	fn := C.CString(filename)
	defer C.free(unsafe.Pointer(fn))
	var t *C.char
	if len(title) > 255 {
		t = C.CString(title[:255])
	} else {
		t = C.CString(title)
	}
	defer C.free(unsafe.Pointer(t))

	options := ticketToOptions(ticket)
	numOptions := C.int(0)
	var o *C.cups_option_t = nil
	for key, value := range options {
		k, v := C.CString(key), C.CString(value)
		numOptions = C.cupsAddOption(k, v, numOptions, &o)
		C.free(unsafe.Pointer(k))
		C.free(unsafe.Pointer(v))
	}
	defer C.cupsFreeOptions(numOptions, o)

	u := C.CString(user)
	defer C.free(unsafe.Pointer(u))

	jobID, err := c.cc.printFile(u, pn, fn, t, numOptions, o)
	if err != nil {
		return 0, err
	}

	return uint32(jobID), nil
}

func ticketToOptions(ticket *cdd.CloudJobTicket) map[string]string {
	m := make(map[string]string)
	if ticket == nil {
		return m
	}

	for _, vti := range ticket.Print.VendorTicketItem {
		m[vti.ID] = vti.Value
	}
	if ticket.Print.Color != nil {
		m["ColorModel"] = ticket.Print.Color.VendorID
	}
	if ticket.Print.Duplex != nil {
		switch ticket.Print.Duplex.Type {
		case "LONG_EDGE":
			m["Duplex"] = "DuplexNoTumble"
		case "SHORT_EDGE":
			m["Duplex"] = "DuplexTumble"
		case "NO_DUPLEX":
			m["Duplex"] = "None"
		}
	}
	if ticket.Print.PageOrientation != nil {
		switch ticket.Print.PageOrientation.Type {
		case "PORTRAIT":
			m["orientation-requested"] = "3"
		case "LANDSCAPE":
			m["orientation-requested"] = "4"
		}
	}
	if ticket.Print.Copies != nil {
		m["copies"] = strconv.FormatInt(int64(ticket.Print.Copies.Copies), 10)
	}
	if ticket.Print.Margins != nil {
		m["page-top"] = micronsToPoints(ticket.Print.Margins.TopMicrons)
		m["page-right"] = micronsToPoints(ticket.Print.Margins.RightMicrons)
		m["page-bottom"] = micronsToPoints(ticket.Print.Margins.BottomMicrons)
		m["page-left"] = micronsToPoints(ticket.Print.Margins.LeftMicrons)
	}
	if ticket.Print.DPI != nil {
		if ticket.Print.DPI.VendorID != "" {
			m["Resolution"] = ticket.Print.DPI.VendorID
		} else {
			m["Resolution"] = fmt.Sprintf("%dx%xdpi",
				ticket.Print.DPI.HorizontalDPI, ticket.Print.DPI.VerticalDPI)
		}
	}
	if ticket.Print.FitToPage != nil {
		switch ticket.Print.FitToPage.Type {
		case "FIT_TO_PAGE":
			m["fit-to-page"] = "true"
		case "NO_FITTING":
			m["fit-to-page"] = "false"
		}
	}
	if ticket.Print.MediaSize != nil {
		if ticket.Print.MediaSize.VendorID != "" {
			m["media"] = ticket.Print.MediaSize.VendorID
		} else {
			widthPoints := micronsToPoints(ticket.Print.MediaSize.WidthMicrons)
			heightPoints := micronsToPoints(ticket.Print.MediaSize.HeightMicrons)
			m["media"] = fmt.Sprintf("Custom.%sx%s", widthPoints, heightPoints)
		}
	}
	if ticket.Print.Collate != nil {
		if ticket.Print.Collate.Collate {
			m["Collate"] = "true"
		} else {
			m["Collate"] = "false"
		}
	}
	if ticket.Print.ReverseOrder != nil {
		if ticket.Print.ReverseOrder.ReverseOrder {
			m["outputorder"] = "reverse"
		} else {
			m["outputorder"] = "normal"
		}
	}

	return m
}

func micronsToPoints(microns int32) string {
	return strconv.Itoa(int(float32(microns)*72/25400 + 0.5))
}

// convertIPPDateToTime converts an RFC 2579 date to a time.Time object.
func convertIPPDateToTime(date *C.ipp_uchar_t) time.Time {
	r := bytes.NewReader(C.GoBytes(unsafe.Pointer(date), 11))
	var year uint16
	var month, day, hour, min, sec, dsec uint8
	binary.Read(r, binary.BigEndian, &year)
	binary.Read(r, binary.BigEndian, &month)
	binary.Read(r, binary.BigEndian, &day)
	binary.Read(r, binary.BigEndian, &hour)
	binary.Read(r, binary.BigEndian, &min)
	binary.Read(r, binary.BigEndian, &sec)
	binary.Read(r, binary.BigEndian, &dsec)

	var utcDirection, utcHour, utcMin uint8
	binary.Read(r, binary.BigEndian, &utcDirection)
	binary.Read(r, binary.BigEndian, &utcHour)
	binary.Read(r, binary.BigEndian, &utcMin)

	var utcOffset time.Duration
	utcOffset += time.Duration(utcHour) * time.Hour
	utcOffset += time.Duration(utcMin) * time.Minute
	var loc *time.Location
	if utcDirection == '-' {
		loc = time.FixedZone("", -int(utcOffset.Seconds()))
	} else {
		loc = time.FixedZone("", int(utcOffset.Seconds()))
	}

	nsec := int(dsec) * 100 * int(time.Millisecond)

	return time.Date(int(year), time.Month(month), int(day), int(hour), int(min), int(sec), nsec, loc)
}

// attributesToTags converts a slice of C.ipp_attribute_t to a
// string:string "tag" map. Outside of this package, "printer attributes" are
// known as "tags".
func attributesToTags(attributes []*C.ipp_attribute_t) map[string][]string {
	tags := make(map[string][]string)

	for _, a := range attributes {
		key := C.GoString(a.name)
		count := int(a.num_values)
		values := make([]string, count)

		switch a.value_tag {
		case C.IPP_TAG_NOVALUE, C.IPP_TAG_NOTSETTABLE:
			// No value means no value.

		case C.IPP_TAG_INTEGER, C.IPP_TAG_ENUM:
			for i := 0; i < count; i++ {
				values[i] = strconv.FormatInt(int64(C.getAttributeIntegerValue(a, C.int(i))), 10)
			}

		case C.IPP_TAG_BOOLEAN:
			for i := 0; i < count; i++ {
				if int(C.getAttributeIntegerValue(a, C.int(i))) == 0 {
					values[i] = "false"
				} else {
					values[i] = "true"
				}
			}

		case C.IPP_TAG_TEXTLANG, C.IPP_TAG_NAMELANG, C.IPP_TAG_TEXT, C.IPP_TAG_NAME, C.IPP_TAG_KEYWORD, C.IPP_TAG_URI, C.IPP_TAG_URISCHEME, C.IPP_TAG_CHARSET, C.IPP_TAG_LANGUAGE, C.IPP_TAG_MIMETYPE:
			for i := 0; i < count; i++ {
				values[i] = C.GoString(C.getAttributeStringValue(a, C.int(i)))
			}

		case C.IPP_TAG_DATE:
			for i := 0; i < count; i++ {
				date := C.getAttributeDateValue(a, C.int(i))
				t := convertIPPDateToTime(date)
				values[i] = strconv.FormatInt(t.Unix(), 10)
			}

		case C.IPP_TAG_RESOLUTION:
			for i := 0; i < count; i++ {
				xres, yres := C.int(0), C.int(0)
				C.getAttributeValueResolution(a, C.int(i), &xres, &yres)
				values[i] = fmt.Sprintf("%dx%dppi", int(xres), int(yres))
			}

		case C.IPP_TAG_RANGE:
			for i := 0; i < count; i++ {
				upper, lower := C.int(0), C.int(0)
				C.getAttributeValueRange(a, C.int(i), &lower, &upper)
				values[i] = fmt.Sprintf("%d~%d", int(lower), int(upper))
			}

		default:
			if count > 0 {
				values = []string{"unknown or unsupported type"}
			}
		}

		if len(values) == 1 && values[0] == "none" {
			values = []string{}
		}
		// This block fixes some drivers' marker types, which list an extra
		// type containing a comma, which CUPS interprets as an extra type.
		// The extra type starts with a space, so it's easy to detect.
		if len(values) > 1 && len(values[len(values)-1]) > 1 && values[len(values)-1][0:1] == " " {
			newValues := make([]string, len(values)-1)
			for i := 0; i < len(values)-2; i++ {
				newValues[i] = values[i]
			}
			newValues[len(newValues)-1] = strings.Join(values[len(values)-2:], ",")
			values = newValues
		}
		tags[key] = values
	}

	return tags
}

// tagsToPrinter converts a map of tags to a Printer.
func tagsToPrinter(printerTags map[string][]string, systemTags map[string]string, infoToDisplayName bool) lib.Printer {
	tags := make(map[string]string)

	for k, v := range printerTags {
		tags[k] = strings.Join(v, ",")
	}
	for k, v := range systemTags {
		tags[k] = v
	}

	var name string
	if n, ok := printerTags[attrPrinterName]; ok {
		name = n[0]
	}
	var uuid string
	if u, ok := printerTags[attrPrinterUUID]; ok {
		uuid = u[0]
	}

	state := cdd.PrinterStateSection{}

	if s, ok := printerTags[attrPrinterState]; ok {
		switch s[0] {
		case "3":
			state.State = cdd.CloudDeviceStateIdle
		case "4":
			state.State = cdd.CloudDeviceStateProcessing
		case "5":
			state.State = cdd.CloudDeviceStateStopped
		default:
			state.State = cdd.CloudDeviceStateIdle
		}
	}

	if reasons, ok := printerTags[attrPrinterStateReasons]; ok && len(reasons) > 0 {
		sort.Strings(reasons)
		state.VendorState = &cdd.VendorState{Item: make([]cdd.VendorStateItem, len(reasons))}
		for i, reason := range reasons {
			vendorState := cdd.VendorStateItem{DescriptionLocalized: cdd.NewLocalizedString(reason)}
			if strings.HasSuffix(reason, "-error") {
				vendorState.State = cdd.VendorStateError
			} else if strings.HasSuffix(reason, "-warning") {
				vendorState.State = cdd.VendorStateWarning
			} else if strings.HasSuffix(reason, "-report") {
				vendorState.State = cdd.VendorStateInfo
			} else {
				vendorState.State = cdd.VendorStateInfo
			}
			state.VendorState.Item[i] = vendorState
		}
	}

	description := cdd.PrinterDescriptionSection{
		Copies: &cdd.Copies{
			Default: 1,
			Max:     1000,
		},
		Collate: &cdd.Collate{
			Default: true,
		},
		VendorCapability: &[]cdd.VendorCapability{numberUpCapability},
	}

	if mimeTypes, ok := printerTags[attrDocumentFormatSupported]; ok && len(mimeTypes) > 0 {
		// Preferred order:
		//  1) PDF because it's small.
		//  2) Postscript because it's a vector format.
		//  3) Any "native" formats so that they don't need conversion.
		//  4) PWG-Raster because it should work any time, but it's huge.

		sct := append(make([]cdd.SupportedContentType, 0, len(mimeTypes)),
			cdd.SupportedContentType{ContentType: "application/pdf"},
			cdd.SupportedContentType{ContentType: "application/postscript"})
		for i := range mimeTypes {
			if mimeTypes[i] == "application/octet-stream" || // Avoid random byte blobs.
				mimeTypes[i] == "application/pdf" ||
				mimeTypes[i] == "application/postscript" ||
				mimeTypes[i] == "image/pwg-raster" {
				continue
			}
			sct = append(sct, cdd.SupportedContentType{ContentType: mimeTypes[i]})
		}
		/*
			TODO: Consider adding pwg-raster with config option to enable/disable.
			- All clients authored by Google do not create PWG Raster jobs.
			- cups-filters only supports pwg-raster input in recent versions.
			  https://www.cups.org/pipermail/cups/2015-July/026927.html
		*/
		description.SupportedContentType = &sct
	}

	markers, markerState := convertMarkers(printerTags[attrMarkerNames], printerTags[attrMarkerTypes], printerTags[attrMarkerLevels])
	state.MarkerState = markerState
	description.Marker = markers

	p := lib.Printer{
		Name:               name,
		DefaultDisplayName: name,
		UUID:               uuid,
		State:              &state,
		Description:        &description,
		Tags:               tags,
	}
	p.SetTagshash()

	if printerInfo, ok := printerTags[attrPrinterInfo]; ok && infoToDisplayName && len(printerInfo) > 0 && printerInfo[0] != "" {
		p.DefaultDisplayName = printerInfo[0]
	}

	return p
}

var cupsMarkerNameToGCP map[string]cdd.MarkerColorType = map[string]cdd.MarkerColorType{
	"black":        cdd.MarkerColorBlack,
	"color":        cdd.MarkerColorColor,
	"cyan":         cdd.MarkerColorCyan,
	"magenta":      cdd.MarkerColorMagenta,
	"yellow":       cdd.MarkerColorYellow,
	"lightcyan":    cdd.MarkerColorLightCyan,
	"lightmagenta": cdd.MarkerColorLightMagenta,
	"gray":         cdd.MarkerColorGray,
	"lightgray":    cdd.MarkerColorLightGray,
	"pigmentblack": cdd.MarkerColorPigmentBlack,
	"matteblack":   cdd.MarkerColorMatteBlack,
	"photocyan":    cdd.MarkerColorPhotoCyan,
	"photomagenta": cdd.MarkerColorPhotoMagenta,
	"photoyellow":  cdd.MarkerColorPhotoYellow,
	"photogray":    cdd.MarkerColorPhotoGray,
	"red":          cdd.MarkerColorRed,
	"green":        cdd.MarkerColorGreen,
	"blue":         cdd.MarkerColorBlue,
}

// convertMarkers converts CUPS marker-(names|types|levels) to *[]cdd.Marker and *cdd.MarkerState.
//
// Normalizes marker type: toner(Cartridge|-cartridge) => toner,
// ink(Cartridge|-cartridge|Ribbon|-ribbon) => ink
func convertMarkers(names, types, levels []string) (*[]cdd.Marker, *cdd.MarkerState) {
	if len(names) == 0 || len(types) == 0 || len(levels) == 0 {
		return nil, nil
	}
	if len(names) != len(types) || len(types) != len(levels) {
		glog.Warningf("Received badly-formatted markers from CUPS: %s, %s, %s",
			strings.Join(names, ";"), strings.Join(types, ";"), strings.Join(levels, ";"))
		return nil, nil
	}

	markers := make([]cdd.Marker, 0, len(names))
	states := cdd.MarkerState{make([]cdd.MarkerStateItem, 0, len(names))}
	for i := 0; i < len(names); i++ {
		if len(names[i]) == 0 {
			return nil, nil
		}
		var markerType cdd.MarkerType
		switch strings.ToLower(types[i]) {
		case "toner", "tonercartridge", "toner-cartridge":
			markerType = cdd.MarkerToner
		case "ink", "inkcartridge", "ink-cartridge", "ink-ribbon", "inkribbon":
			markerType = cdd.MarkerInk
		case "staples":
			markerType = cdd.MarkerStaples
		default:
			continue
		}

		nameStripped := strings.Replace(strings.Replace(strings.ToLower(names[i]), " ", "", -1), "-", "", -1)
		colorType := cdd.MarkerColorCustom
		for k, v := range cupsMarkerNameToGCP {
			if strings.HasPrefix(nameStripped, k) {
				colorType = v
				break
			}
		}
		color := cdd.MarkerColor{Type: colorType}
		if colorType == cdd.MarkerColorCustom {
			name := names[i]
			name = strings.TrimSuffix(name, " Cartridge")
			name = strings.TrimSuffix(name, " cartridge")
			name = strings.TrimSuffix(name, " Ribbon")
			name = strings.TrimSuffix(name, " ribbon")
			name = strings.TrimSuffix(name, " Toner")
			name = strings.TrimSuffix(name, " toner")
			name = strings.TrimSuffix(name, " Ink")
			name = strings.TrimSuffix(name, " ink")
			name = strings.Replace(name, "-", " ", -1)
			color.CustomDisplayNameLocalized = cdd.NewLocalizedString(name)
		}

		marker := cdd.Marker{
			VendorID: names[i],
			Type:     markerType,
			Color:    &color,
		}

		level, err := strconv.ParseInt(levels[i], 10, 32)
		if err != nil {
			glog.Warningf("Failed to parse CUPS marker state %s=%s: %s", names[i], levels[i], err)
			return nil, nil
		}
		if level > 100 {
			// Lop off extra (proprietary?) bits.
			level = level & 0x7f
		}
		if level < 0 || level > 100 {
			return nil, nil
		}

		var state cdd.MarkerStateType
		if level > 10 {
			state = cdd.MarkerStateOK
		} else {
			state = cdd.MarkerStateExhausted
		}
		level32 := int32(level)
		markerState := cdd.MarkerStateItem{
			VendorID:     names[i],
			State:        state,
			LevelPercent: &level32,
		}

		markers = append(markers, marker)
		states.Item = append(states.Item, markerState)
	}

	return &markers, &states
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if needle == h {
			return true
		}
	}
	return false
}

func findMissing(haystack, needles []string) []string {
	missing := make([]string, 0)
	for _, n := range needles {
		if !contains(haystack, n) {
			missing = append(missing, n)
		}
	}
	return missing
}

func checkPrinterAttributes(printerAttributes []string) error {
	if !contains(printerAttributes, "all") {
		missing := findMissing(printerAttributes, requiredPrinterAttributes)
		if len(missing) > 0 {
			return fmt.Errorf("Printer attributes missing from config file: %s",
				strings.Join(missing, ","))
		}
	}

	return nil
}
