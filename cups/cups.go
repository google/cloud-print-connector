/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package cups

/*
#cgo LDFLAGS: -lcups
#include <cups/cups.h>
#include <stdlib.h> // free, calloc

const char
		*JOB_STATE         = "job-state",
		*JOB_STATE_REASONS = "job-state-reasons";

// Allocates a new char**, initializes the values to NULL.
char **newArrayOfStrings(int size) {
	return calloc(size, sizeof(char *));
}

// Sets one value in a char**.
void setStringArrayValue(char **stringArray, int index, char *value) {
	stringArray[index] = value;
}

// Frees a char** and associated non-NULL char*.
void freeStringArrayAndStrings(char **stringArray, int size) {
	int i;
	for (i = 0; i < size; i++) {
		if (stringArray[i] != NULL)
			free(stringArray[i]);
	}
	free(stringArray);
}

// Wraps ippGetResolution() until bug fixed:
// https://code.google.com/p/go/issues/detail?id=7270
int ippGetResolutionWrapper(ipp_attribute_t *attr, int element, int *yres, int *units) {
	return ippGetResolution(attr, element, yres, (ipp_res_t *)units);
}
*/
import "C"
import (
	"bytes"
	"cups-connector/lib"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/golang/glog"
)

const (
	// CUPS "URL" length are always less than 40. For example: /job/1234567
	urlMaxLength = 100

	attrPrinterName         = "printer-name"
	attrPrinterInfo         = "printer-info"
	attrPrinterMakeAndModel = "printer-make-and-model"
	attrPrinterState        = "printer-state"

	attrJobState       = "job-state"
	attrJobStateReason = "job-state-reason"
)

var (
	requiredPrinterAttributes []string = []string{
		attrPrinterName,
		attrPrinterInfo,
		attrPrinterMakeAndModel,
		attrPrinterState,
	}

	jobAttributes []string = []string{
		attrJobState,
		attrJobStateReason,
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

	c := &CUPS{cc, pc, infoToDisplayName, printerAttributes, systemTags}

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

	if C.ippGetStatusCode(response) == C.IPP_STATUS_ERROR_NOT_FOUND {
		// Normal error when there are no printers.
		return make([]lib.Printer, 0), nil
	}

	printers := make([]lib.Printer, 0, 1)

	for a := C.ippFirstAttribute(response); a != nil; a = C.ippNextAttribute(response) {
		if C.ippGetGroupTag(a) != C.IPP_TAG_PRINTER {
			continue
		}

		attributes := make([]*C.ipp_attribute_t, 0, C.int(len(c.printerAttributes)))

		for ; a != nil && C.ippGetGroupTag(a) == C.IPP_TAG_PRINTER; a = C.ippNextAttribute(response) {
			attributes = append(attributes, a)
		}

		tags := attributesToTags(attributes)

		p, err := tagsToPrinter(tags, c.systemTags, c.infoToDisplayName)
		if err != nil {
			glog.Error(err)
			continue
		}

		printers = append(printers, p)
	}

	c.addPPDHashToPrinters(printers)

	return printers, nil
}

// addPPDHashToPrinters fetches PPD hashes for all printers concurrently.
func (c *CUPS) addPPDHashToPrinters(printers []lib.Printer) {
	var wg sync.WaitGroup

	for i := range printers {
		if !lib.PrinterIsRaw(printers[i]) {
			wg.Add(1)
			go func(p *lib.Printer) {
				if ppdHash, err := c.pc.getPPDHash(p.Name); err == nil {
					p.CapsHash = ppdHash
				} else {
					glog.Error(err)
				}
				wg.Done()
			}(&printers[i])
		}
	}

	wg.Wait()
}

func getSystemTags() (map[string]string, error) {
	tags := make(map[string]string)

	tags["connector-version"] = lib.GetBuildDate()
	hostname, err := os.Hostname()
	if err == nil {
		tags["system-hostname"] = hostname
	}
	tags["system-arch"] = runtime.GOARCH

	sysname, nodename, release, version, machine, domainname, err := uname()
	if err != nil {
		return nil, fmt.Errorf("CUPS failed to call uname while initializing: %s", err)
	}

	tags["system-uname-sysname"] = sysname
	tags["system-uname-nodename"] = nodename
	tags["system-uname-release"] = release
	tags["system-uname-version"] = version
	tags["system-uname-machine"] = machine
	tags["system-uname-domainname"] = domainname

	tags["connector-cups-api-version"] = fmt.Sprintf("%d.%d.%d",
		C.CUPS_VERSION_MAJOR, C.CUPS_VERSION_MINOR, C.CUPS_VERSION_PATCH)

	return tags, nil
}

// GetPPD gets the PPD for the specified printer.
func (c *CUPS) GetPPD(printername string) (string, error) {
	return c.pc.getPPD(printername)
}

// RemoveCachedPPD removes a printer's PPD from the cache.
func (c *CUPS) RemoveCachedPPD(printername string) {
	c.pc.removePPD(printername)
}

// GetJobStatus gets the current status of the job indicated by jobID.
func (c *CUPS) GetJobStatus(jobID uint32) (lib.CUPSJobStatus, string, error) {
	ja := C.newArrayOfStrings(C.int(len(jobAttributes)))
	defer C.freeStringArrayAndStrings(ja, C.int(len(jobAttributes)))
	for i, attribute := range jobAttributes {
		C.setStringArrayValue(ja, C.int(i), C.CString(attribute))
	}

	response, err := c.cc.getJobAttributes(C.int(jobID), ja)
	if err != nil {
		return "", "", err
	}

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(response)

	s := C.ippFindAttribute(response, C.JOB_STATE, C.IPP_TAG_ENUM)
	status := lib.CUPSJobStatusFromInt(uint8(C.ippGetInteger(s, C.int(0))))

	sr := C.ippFindAttribute(response, C.JOB_STATE_REASONS, C.IPP_TAG_STRING)
	var statusReason string
	if sr != nil {
		statusReason = C.GoString(C.ippGetString(sr, C.int(0), nil))
	}

	return status, statusReason, nil
}

// Print sends a new print job to the specified printer. The job ID
// is returned.
func (c *CUPS) Print(printername, filename, title, user string, options map[string]string) (uint32, error) {
	pn := C.CString(printername)
	defer C.free(unsafe.Pointer(pn))
	fn := C.CString(filename)
	defer C.free(unsafe.Pointer(fn))
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))

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
func attributesToTags(attributes []*C.ipp_attribute_t) map[string]string {
	tags := make(map[string]string)

	for _, a := range attributes {
		key := C.GoString(C.ippGetName(a))
		count := int(C.ippGetCount(a))
		values := make([]string, count)

		switch C.ippGetValueTag(a) {
		case C.IPP_TAG_NOVALUE, C.IPP_TAG_NOTSETTABLE:
			// No value means no value.

		case C.IPP_TAG_INTEGER, C.IPP_TAG_ENUM:
			for i := 0; i < count; i++ {
				values[i] = fmt.Sprintf("%d", int(C.ippGetInteger(a, C.int(i))))
			}

		case C.IPP_TAG_BOOLEAN:
			for i := 0; i < count; i++ {
				if int(C.ippGetInteger(a, C.int(i))) == 0 {
					values[i] = "false"
				} else {
					values[i] = "true"
				}
			}

		case C.IPP_TAG_STRING, C.IPP_TAG_TEXT, C.IPP_TAG_NAME, C.IPP_TAG_KEYWORD, C.IPP_TAG_URI, C.IPP_TAG_CHARSET, C.IPP_TAG_LANGUAGE, C.IPP_TAG_MIMETYPE:
			for i := 0; i < count; i++ {
				values[i] = C.GoString(C.ippGetString(a, C.int(i), nil))
			}

		case C.IPP_TAG_DATE:
			for i := 0; i < count; i++ {
				date := C.ippGetDate(a, C.int(i))
				t := convertIPPDateToTime(date)
				values[i] = fmt.Sprintf("%d", t.Unix())
			}

		case C.IPP_TAG_RESOLUTION:
			for i := 0; i < count; i++ {
				yres := C.int(-1)
				unit := C.int(-1)
				xres := C.ippGetResolutionWrapper(a, C.int(i), &yres, &unit)
				if unit == C.IPP_RES_PER_CM {
					values[i] = fmt.Sprintf("%dx%dpp%s", int(xres), int(yres), "cm")
				} else {
					values[i] = fmt.Sprintf("%dx%dpp%s", int(xres), int(yres), "i")
				}
			}

		case C.IPP_TAG_RANGE:
			for i := 0; i < count; i++ {
				uppervalue := C.int(-1)
				lowervalue := C.ippGetRange(a, C.int(i), &uppervalue)
				values[i] = fmt.Sprintf("%d~%d", int(lowervalue), int(uppervalue))
			}

		default:
			if count > 0 {
				values = []string{"unknown or unsupported type"}
			}
		}

		value := strings.Join(values, ",")
		if value == "none" {
			value = ""
		}
		tags[key] = value
	}

	return tags
}

// tagsToPrinter converts a map of tags to a Printer.
func tagsToPrinter(printerTags, systemTags map[string]string, infoToDisplayName bool) (lib.Printer, error) {
	tags := make(map[string]string)

	for k, v := range printerTags {
		tags[k] = v
	}
	for k, v := range systemTags {
		tags[k] = v
	}

	p := lib.Printer{
		Name:        printerTags[attrPrinterName],
		Description: printerTags[attrPrinterMakeAndModel],
		Status:      lib.PrinterStatusFromString(printerTags[attrPrinterState]),
		Tags:        tags,
	}
	p.SetTagshash()

	if infoToDisplayName {
		p.DefaultDisplayName = printerTags[attrPrinterInfo]
	}

	return p, nil
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
