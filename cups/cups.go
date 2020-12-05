// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin freebsd

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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/cloud-print-connector/cdd"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
)

const (
	// CUPS "URL" length are always less than 40. For example: /job/1234567
	urlMaxLength = 100

	// Attributes that CUPS uses to describe printers.
	attrCUPSVersion                   = "cups-version"
	attrCopiesDefault                 = "copies-default"
	attrCopiesSupported               = "copies-supported"
	attrDeviceURI                     = "device-uri"
	attrDocumentFormatSupported       = "document-format-supported"
	attrMarkerLevels                  = "marker-levels"
	attrMarkerNames                   = "marker-names"
	attrMarkerTypes                   = "marker-types"
	attrNumberUpDefault               = "number-up-default"
	attrNumberUpSupported             = "number-up-supported"
	attrOrientationRequestedDefault   = "orientation-requested-default"
	attrOrientationRequestedSupported = "orientation-requested-supported"
	attrPDFVersionsSupported          = "pdf-versions-supported"
	attrPrintColorModeDefault         = "print-color-mode-default"
	attrPrintColorModeSupported       = "print-color-mode-supported"
	attrPrinterInfo                   = "printer-info"
	attrPrinterName                   = "printer-name"
	attrPrinterState                  = "printer-state"
	attrPrinterStateReasons           = "printer-state-reasons"
	attrPrinterUUID                   = "printer-uuid"

	// Attributes that the connector uses to describe print jobs to CUPS.
	attrCopies               = "copies"
	attrCollate              = "collate"
	attrFalse                = "false"
	attrFitToPage            = "fit-to-page"
	attrMediaBottomMargin    = "media-bottom-margin"
	attrMediaLeftMargin      = "media-left-margin"
	attrMediaRightMargin     = "media-right-margin"
	attrMediaTopMargin       = "media-top-margin"
	attrNormal               = "normal"
	attrNumberUp             = "number-up"
	attrOrientationRequested = "orientation-requested"
	attrOutputOrder          = "outputorder"
	attrPrintColorMode       = "print-color-mode"
	attrReverse              = "reverse"
	attrTrue                 = "true"

	// Attributes that CUPS uses to describe job state.
	attrJobMediaSheetsCompleted = "job-media-sheets-completed"
	attrJobState                = "job-state"
)

var (
	requiredPrinterAttributes []string = []string{
		attrCopiesDefault,
		attrCopiesSupported,
		attrDeviceURI,
		attrDocumentFormatSupported,
		attrMarkerLevels,
		attrMarkerNames,
		attrMarkerTypes,
		attrNumberUpDefault,
		attrNumberUpSupported,
		attrOrientationRequestedDefault,
		attrOrientationRequestedSupported,
		attrPDFVersionsSupported,
		attrPrintColorModeDefault,
		attrPrintColorModeSupported,
		attrPrinterInfo,
		attrPrinterName,
		attrPrinterState,
		attrPrinterStateReasons,
		attrPrinterUUID,
	}

	jobAttributes []string = []string{
		attrJobState,
		attrJobMediaSheetsCompleted,
	}

	// cupsPDS represents capabilities that CUPS always provides.
	cupsPDS = cdd.PrinterDescriptionSection{
		FitToPage: &cdd.FitToPage{
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
		},
		ReverseOrder: &cdd.ReverseOrder{Default: false},
		Collate:      &cdd.Collate{Default: true},
	}
)

// Interface between Go and the CUPS API.
type CUPS struct {
	cc                    *cupsCore
	pc                    *ppdCache
	infoToDisplayName     bool
	prefixJobIDToJobTitle bool
	displayNamePrefix     string
	printerAttributes     []string
	systemTags            map[string]string
	printerBlacklist      map[string]interface{}
	printerWhitelist      map[string]interface{}
	ignoreRawPrinters     bool
	ignoreClassPrinters   bool
}

func NewCUPS(infoToDisplayName, prefixJobIDToJobTitle bool, displayNamePrefix string,
	printerAttributes, vendorPPDOptions []string, maxConnections uint, connectTimeout time.Duration,
	printerBlacklist, printerWhitelist []string, ignoreRawPrinters bool, ignoreClassPrinters bool,
	fcmNotificationsEnable bool) (*CUPS, error) {
	if err := checkPrinterAttributes(printerAttributes); err != nil {
		return nil, err
	}

	cc, err := newCUPSCore(maxConnections, connectTimeout)
	if err != nil {
		return nil, err
	}
	pc := newPPDCache(cc, vendorPPDOptions)

	systemTags, err := getSystemTags(fcmNotificationsEnable)
	if err != nil {
		return nil, err
	}

	pb := map[string]interface{}{}
	for _, p := range printerBlacklist {
		pb[p] = struct{}{}
	}

	pw := map[string]interface{}{}
	for _, p := range printerWhitelist {
		pw[p] = struct{}{}
	}

	c := &CUPS{
		cc:                  cc,
		pc:                  pc,
		infoToDisplayName:   infoToDisplayName,
		displayNamePrefix:   displayNamePrefix,
		printerAttributes:   printerAttributes,
		systemTags:          systemTags,
		printerBlacklist:    pb,
		printerWhitelist:    pw,
		ignoreRawPrinters:   ignoreRawPrinters,
		ignoreClassPrinters: ignoreClassPrinters,
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

	if c.ignoreRawPrinters {
		printers = filterRawPrinters(printers)
	}
	if c.ignoreClassPrinters {
		printers = filterClassPrinters(printers)
	}
	printers = c.addPPDDescriptionToPrinters(printers)
	printers = addStaticDescriptionToPrinters(printers)
	printers = c.addSystemTagsToPrinters(printers)

	return printers, nil
}

// responseToPrinters converts a C.ipp_t to a slice of lib.Printers.
func (c *CUPS) responseToPrinters(response *C.ipp_t) []lib.Printer {
	printers := make([]lib.Printer, 0, 1)

	for a := C.ippFirstAttribute(response); a != nil; a = C.ippNextAttribute(response) {
		if C.ippGetGroupTag(a) != C.IPP_TAG_PRINTER {
			continue
		}

		attributes := make([]*C.ipp_attribute_t, 0, C.int(len(c.printerAttributes)))
		for ; a != nil && C.ippGetGroupTag(a) == C.IPP_TAG_PRINTER; a = C.ippNextAttribute(response) {
			attributes = append(attributes, a)
		}
		mAttributes := attributesToMap(attributes)
		pds, pss, name, defaultDisplayName, uuid, tags := translateAttrs(mAttributes)

		// Check whitelist/blacklist in loop once we have printer name.
		// Avoids unnecessary processing of excluded printers.
		if _, exists := c.printerBlacklist[name]; exists {
			log.Debugf("Ignoring blacklisted printer %s", name)
			if a == nil {
				break
			}
			continue
		}
		if len(c.printerWhitelist) != 0 {
			if _, exists := c.printerWhitelist[name]; !exists {
				log.Debugf("Ignoring non-whitelisted printer %s", name)
				if a == nil {
					break
				}
				continue
			}
		}

		if !c.infoToDisplayName || defaultDisplayName == "" {
			defaultDisplayName = name
		}
		defaultDisplayName = c.displayNamePrefix + defaultDisplayName
		p := lib.Printer{
			Name:               name,
			DefaultDisplayName: defaultDisplayName,
			UUID:               uuid,
			State:              pss,
			Description:        pds,
			Tags:               tags,
		}

		printers = append(printers, p)
		if a == nil {
			break
		}
	}

	return printers
}

// filterClassPrinters removes class printers from the slice.
func filterClassPrinters(printers []lib.Printer) []lib.Printer {
	result := make([]lib.Printer, 0, len(printers))
	for i := range printers {
		if !lib.PrinterIsClass(printers[i]) {
			result = append(result, printers[i])
		}
	}
	return result
}

// filterRawPrinters removes raw printers from the slice.
func filterRawPrinters(printers []lib.Printer) []lib.Printer {
	result := make([]lib.Printer, 0, len(printers))
	for i := range printers {
		if !lib.PrinterIsRaw(printers[i]) {
			result = append(result, printers[i])
		}
	}
	return result
}

// addPPDDescriptionToPrinters fetches description, PPD hash, manufacturer, model
// for argument printers, concurrently. These are the fields derived from PPD.
func (c *CUPS) addPPDDescriptionToPrinters(printers []lib.Printer) []lib.Printer {
	var wg sync.WaitGroup
	ch := make(chan *lib.Printer, len(printers))

	for i := range printers {
		wg.Add(1)
		go func(p *lib.Printer) {
			if description, manufacturer, model, duplexMap, err := c.pc.getPPDCacheEntry(p.Name); err == nil {
				p.Description.Absorb(description)
				p.Manufacturer = manufacturer
				p.Model = model
				if duplexMap != nil {
					p.DuplexMap = duplexMap
				}
				ch <- p
			} else {
				log.ErrorPrinter(p.Name, err)
			}
			wg.Done()
		}(&printers[i])
	}

	wg.Wait()
	close(ch)

	result := make([]lib.Printer, 0, len(ch))
	for printer := range ch {
		result = append(result, *printer)
	}

	return result
}

// addStaticDescriptionToPrinters adds information that is true for all
// printers to printers.
func addStaticDescriptionToPrinters(printers []lib.Printer) []lib.Printer {
	for i := range printers {
		printers[i].GCPVersion = lib.GCPAPIVersion
		printers[i].SetupURL = lib.ConnectorHomeURL
		printers[i].SupportURL = lib.ConnectorHomeURL
		printers[i].UpdateURL = lib.ConnectorHomeURL
		printers[i].ConnectorVersion = lib.ShortName
		printers[i].Description.Absorb(&cupsPDS)
	}
	return printers
}

func (c *CUPS) addSystemTagsToPrinters(printers []lib.Printer) []lib.Printer {
	for i := range printers {
		for k, v := range c.systemTags {
			printers[i].Tags[k] = v
		}
	}
	return printers
}

// uname returns strings similar to the Unix uname command:
// sysname, nodename, release, version, machine
func uname() (string, string, string, string, string, error) {
	var name C.struct_utsname
	_, err := C.uname(&name)
	if err != nil {
		var errno syscall.Errno = err.(syscall.Errno)
		return "", "", "", "", "", fmt.Errorf("Failed to call uname: %s", errno)
	}

	return C.GoString(&name.sysname[0]), C.GoString(&name.nodename[0]),
		C.GoString(&name.release[0]), C.GoString(&name.version[0]),
		C.GoString(&name.machine[0]), nil
}

func getSystemTags(fcmNotificationsEnable bool) (map[string]string, error) {
	tags := make(map[string]string)

	tags["connector-version"] = lib.BuildDate
	hostname, err := os.Hostname()
	if err == nil {
		tags["system-hostname"] = hostname
	}
	tags["system-arch"] = runtime.GOARCH
	tags["system-golang-version"] = runtime.Version()
	if fcmNotificationsEnable {
		tags["system-notifications-channel"] = "fcm"
	} else {
		tags["system-notifications-channel"] = "xmpp"
	}
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
func (c *CUPS) GetJobState(_ string, jobID uint32) (*cdd.PrintJobStateDiff, error) {
	ja := C.newArrayOfStrings(C.int(len(jobAttributes)))
	defer C.freeStringArrayAndStrings(ja, C.int(len(jobAttributes)))
	for i, attribute := range jobAttributes {
		C.setStringArrayValue(ja, C.int(i), C.CString(attribute))
	}

	response, err := c.cc.getJobAttributes(C.int(jobID), ja)
	if err != nil {
		return nil, err
	}

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(response)

	s := C.ippFindAttribute(response, C.JOB_STATE, C.IPP_TAG_ENUM)
	state := int32(C.getAttributeIntegerValue(s, C.int(0)))

	return convertJobState(state), nil
}

// convertJobState converts CUPS job state to cdd.PrintJobStateDiff.
func convertJobState(cupsState int32) *cdd.PrintJobStateDiff {
	var state cdd.PrintJobStateDiff

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

	return &state
}

// Print sends a new print job to the specified printer. The job ID
// is returned.
func (c *CUPS) Print(printer *lib.Printer, filename, title, user, gcpJobID string, ticket *cdd.CloudJobTicket) (uint32, error) {
	printer.NativeJobSemaphore.Acquire()
	defer printer.NativeJobSemaphore.Release()

	pn := C.CString(printer.Name)
	defer C.free(unsafe.Pointer(pn))
	fn := C.CString(filename)
	defer C.free(unsafe.Pointer(fn))
	var t *C.char

	if c.prefixJobIDToJobTitle {
		title = fmt.Sprintf("gcp:%s %s", gcpJobID, title)
	}
	if len(title) > 255 {
		t = C.CString(title[:255])
	} else {
		t = C.CString(title)
	}
	defer C.free(unsafe.Pointer(t))

	options, err := translateTicket(printer, ticket)
	if err != nil {
		return 0, err
	}
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

	cupsJobID, err := c.cc.printFile(u, pn, fn, t, numOptions, o)
	if err != nil {
		return 0, err
	}

	return uint32(cupsJobID), nil
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

// attributesToMap converts a slice of C.ipp_attribute_t to a
// string:string "tag" map.
func attributesToMap(attributes []*C.ipp_attribute_t) map[string][]string {
	m := make(map[string][]string)

	for _, a := range attributes {
		key := C.GoString(C.ippGetName(a))
		count := int(C.ippGetCount(a))
		values := make([]string, count)

		switch C.ippGetValueTag(a) {
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

		if len(values) == 1 && (values[0] == "none" || len(values[0]) == 0) {
			values = []string{}
		}
		m[key] = values
	}

	return m
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

// The following functions are not relevant to CUPS printing, but are required by the NativePrintSystem interface.

func (c *CUPS) ReleaseJob(printerName string, jobID uint32) error {
	return nil
}
