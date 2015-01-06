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
package cups

/*
#cgo LDFLAGS: -lcups
#include <cups/cups.h>
#include <stddef.h> // size_t
#include <stdlib.h> // free, malloc
#include <arpa/inet.h> // ntohs

const char
    *POST_RESOURCE = "/",
    *REQUESTED_ATTRIBUTES = "requested-attributes",
		*JOB_URI = "job-uri",
		*JOB_STATE = "job-state",
		*JOB_STATE_REASONS = "job-state-reasons",
		*IPP = "ipp";

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
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/golang/glog"
)

const (
	// CUPS "URL" length are always less than 40. For example: /job/1234567
	urlMaxLength = 100

	tagPrinterName         = "printer-name"
	tagPrinterInfo         = "printer-info"
	tagPrinterMakeAndModel = "printer-make-and-model"
	tagPrinterState        = "printer-state"
)

var requiredPrinterAttributes []string = []string{
	tagPrinterName,
	tagPrinterInfo,
	tagPrinterMakeAndModel,
	tagPrinterState,
}

// Interface between Go and the CUPS API.
type CUPS struct {
	c_http                *C.http_t
	pc                    *ppdCache
	infoToDisplayName     bool
	c_printerAttributes   **C.char
	printerAttributesSize int
	c_jobAttributes       **C.char
}

// NewCUPS calls httpConnectEncrypt() via cgo, to create a new, open,
// connection to the CUPS server specified by environment variables,
// client.conf, etc.
func NewCUPS(infoToDisplayName bool, printerAttributes []string) (*CUPS, error) {
	for _, requiredAttribute := range requiredPrinterAttributes {
		found := false
		for _, attribute := range printerAttributes {
			if attribute == requiredAttribute {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Attribute %s missing from config file", requiredAttribute)
		}
	}

	printerAttributesSize := len(printerAttributes)
	c_printerAttributes := C.newArrayOfStrings(C.int(printerAttributesSize))
	for i, a := range printerAttributes {
		C.setStringArrayValue(c_printerAttributes, C.int(i), C.CString(a))
	}

	c_jobAttributes := C.newArrayOfStrings(C.int(2))
	C.setStringArrayValue(c_jobAttributes, C.int(0), C.JOB_STATE)
	C.setStringArrayValue(c_jobAttributes, C.int(1), C.JOB_STATE_REASONS)

	c_host := C.cupsServer()
	c_port := C.ippPort()
	c_encryption := C.cupsEncryption()

	c_http := C.httpConnectEncrypt(c_host, c_port, c_encryption)
	if c_http == nil {
		return nil, fmt.Errorf("Failed to connect to %s:%d", C.GoString(c_host), int(c_port))
	}

	var e string
	switch c_encryption {
	case C.HTTP_ENCRYPTION_ALWAYS:
		e = "encrypting ALWAYS"
	case C.HTTP_ENCRYPTION_IF_REQUESTED:
		e = "encrypting IF REQUESTED"
	case C.HTTP_ENCRYPTION_NEVER:
		e = "encrypting NEVER"
	case C.HTTP_ENCRYPTION_REQUIRED:
		e = "encryption REQUIRED"
	default:
		c_encryption = C.HTTP_ENCRYPTION_IF_REQUESTED
		e = "encrypting IF REQUESTED"
	}

	glog.Infof("connected to CUPS server %s:%d %s\n", C.GoString(c_host), int(c_port), e)

	pc := newPPDCache(c_http)

	c := &CUPS{
		c_http, pc,
		infoToDisplayName, c_printerAttributes, printerAttributesSize, c_jobAttributes}

	return c, nil
}

func (c *CUPS) Quit() {
	c.pc.quit()
	C.freeStringArrayAndStrings(c.c_printerAttributes, C.int(c.printerAttributesSize))
}

// doRequestWithRetry calls doRequest and retries once on failure.
func (c *CUPS) doRequestWithRetry(c_request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	c_response, err := c.doRequest(c_request, acceptableStatusCodes)
	if err == nil {
		return c_response, err
	}

	return c.doRequest(c_request, acceptableStatusCodes)
}

// doRequest calls cupsDoRequest() via cgo.
//
// Uses []C.ipp_status_t type for acceptableStatusCodes because compiler fails on
// "...C.ipp_status_t" type.
func (c *CUPS) doRequest(c_request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	c_response := C.cupsDoRequest(c.c_http, c_request, C.POST_RESOURCE)
	if c_response == nil {
		return nil, fmt.Errorf("%d %s", int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
	}
	c_statusCode := C.ippGetStatusCode(c_response)
	for _, sc := range acceptableStatusCodes {
		if c_statusCode == sc {
			return c_response, nil
		}
	}

	return nil, fmt.Errorf("IPP status code %d", int(c_statusCode))
}

// GetPrinters calls cupsDoRequest() with IPP_OP_CUPS_GET_PRINTERS, and returns
// all CUPS printers found on the CUPS server.
func (c *CUPS) GetPrinters() ([]lib.Printer, error) {
	// ippNewRequest() returns ipp_t pointer which does not need explicit free.
	c_request := C.ippNewRequest(C.IPP_OP_CUPS_GET_PRINTERS)
	C.ippAddStrings(c_request, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		C.int(c.printerAttributesSize), nil, c.c_printerAttributes)

	if err := reconnect(c.c_http); err != nil {
		return nil, err
	}

	c_response, err := c.doRequestWithRetry(c_request, []C.ipp_status_t{C.IPP_STATUS_OK, C.IPP_STATUS_ERROR_NOT_FOUND})
	if err != nil {
		err = fmt.Errorf(
			"Failed to call cupsDoRequest() [IPP_OP_CUPS_GET_PRINTERS]: %s", err)
		return nil, err
	}

	// cupsDoRequest() returns ipp_t pointer which needs explicit free.
	defer C.ippDelete(c_response)

	if C.ippGetStatusCode(c_response) == C.IPP_STATUS_ERROR_NOT_FOUND {
		// Normal error when there are no printers.
		return make([]lib.Printer, 0), nil
	}

	printers := make([]lib.Printer, 0, 1)

	for a := C.ippFirstAttribute(c_response); a != nil; a = C.ippNextAttribute(c_response) {
		if C.ippGetGroupTag(a) != C.IPP_TAG_PRINTER {
			continue
		}

		attributes := make([]*C.ipp_attribute_t, 0, c.printerAttributesSize)

		for ; a != nil && C.ippGetGroupTag(a) == C.IPP_TAG_PRINTER; a = C.ippNextAttribute(c_response) {
			attributes = append(attributes, a)
		}

		tags := attributesToTags(attributes)
		printer, err := c.tagsToPrinter(tags)
		if err != nil {
			glog.Error(err)
			continue
		}
		printers = append(printers, printer)
	}

	return printers, nil
}

// convertIPPDateToTime converts an RFC 2579 date to a time.Time object.
func convertIPPDateToTime(c_date *C.ipp_uchar_t) time.Time {
	r := bytes.NewReader(C.GoBytes(unsafe.Pointer(c_date), 11))
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
				c_date := C.ippGetDate(a, C.int(i))
				t := convertIPPDateToTime(c_date)
				values[i] = fmt.Sprintf("%d", t.Unix())
			}

		case C.IPP_TAG_RESOLUTION:
			for i := 0; i < count; i++ {
				c_yres := C.int(-1)
				c_unit := C.int(-1)
				c_xres := C.ippGetResolutionWrapper(a, C.int(i), &c_yres, &c_unit)
				var unit string
				if c_unit == C.IPP_RES_PER_CM {
					unit = "cm"
				} else {
					unit = "i"
				}
				values[i] = fmt.Sprintf("%dx%dpp%s", int(c_xres), int(c_yres), unit)
			}

		case C.IPP_TAG_RANGE:
			for i := 0; i < count; i++ {
				c_uppervalue := C.int(-1)
				c_lowervalue := C.ippGetRange(a, C.int(i), &c_uppervalue)
				values[i] = fmt.Sprintf("%d~%d", int(c_lowervalue), int(c_uppervalue))
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
func (c *CUPS) tagsToPrinter(tags map[string]string) (lib.Printer, error) {
	p := lib.Printer{
		Name:        tags[tagPrinterName],
		Description: tags[tagPrinterMakeAndModel],
		Status:      lib.PrinterStatusFromString(tags[tagPrinterState]),
		Tags:        tags,
	}
	if c.infoToDisplayName {
		p.DefaultDisplayName = tags[tagPrinterInfo]
	}
	if !lib.PrinterIsRaw(p) {
		ppdHash, err := c.pc.getPPDHash(p.Name)
		if err != nil {
			return lib.Printer{}, err
		}
		p.CapsHash = ppdHash
	}

	return p, nil
}

// GetPPD calls cupsGetPPD3() to get the PPD for the specified printer.
func (c *CUPS) GetPPD(printerName string) (string, error) {
	return c.pc.getPPD(printerName)
}

// GetPPDHash calls cupsGetPPD3() to gets the PPD hash (aka capsHash) for the
// specified printer.
func (c *CUPS) GetPPDHash(printerName string) (string, error) {
	return c.pc.getPPDHash(printerName)
}

// GetJobStatus calls cupsDoRequest() with IPP_OP_GET_JOB_ATTRIBUTES to get the
// current status of the job indicated by the CUPS jobID.
func (c *CUPS) GetJobStatus(jobID uint32) (lib.CUPSJobStatus, string, error) {
	c_uri, err := createJobURI(jobID)
	if err != nil {
		return "", "", err
	}
	defer C.free(unsafe.Pointer(c_uri))

	// ippNewRequest() returns ipp_t pointer does not need explicit free.
	c_request := C.ippNewRequest(C.IPP_OP_GET_JOB_ATTRIBUTES)

	C.ippAddString(
		c_request, C.IPP_TAG_OPERATION, C.IPP_TAG_URI, C.JOB_URI, nil, c_uri)
	C.ippAddStrings(c_request, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD,
		C.REQUESTED_ATTRIBUTES, C.int(0), nil, c.c_jobAttributes)

	if err := reconnect(c.c_http); err != nil {
		return "", "", err
	}

	c_response, err := c.doRequestWithRetry(c_request, []C.ipp_status_t{C.IPP_STATUS_OK})
	if err != nil {
		err = fmt.Errorf(
			"Failed to call cupsDoRequest() [IPP_OP_GET_JOB_ATTRIBUTES]: %s", err)
		return "", "", err
	}

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(c_response)

	c_status := C.ippFindAttribute(c_response, C.JOB_STATE, C.IPP_TAG_ENUM)
	status := lib.CUPSJobStatusFromInt(uint8(C.ippGetInteger(c_status, C.int(0))))

	c_statusReason := C.ippFindAttribute(
		c_response, C.JOB_STATE_REASONS, C.IPP_TAG_STRING)
	var statusReason string
	if c_statusReason != nil {
		statusReason = C.GoString(C.ippGetString(c_statusReason, C.int(0), nil))
	}

	return status, statusReason, nil
}

// Print calls cupsSetUser() and cupsPrintFile2() to send a new print job to the
// CUPS server.
func (c *CUPS) Print(printerName, fileName, title, ownerID string, options map[string]string) (uint32, error) {
	c_printerName := C.CString(printerName)
	defer C.free(unsafe.Pointer(c_printerName))
	c_fileName := C.CString(fileName)
	defer C.free(unsafe.Pointer(c_fileName))
	c_title := C.CString(title)
	defer C.free(unsafe.Pointer(c_title))
	c_numOptions := C.int(0)
	var c_options *C.cups_option_t = nil
	defer C.cupsFreeOptions(c_numOptions, c_options)

	for key, value := range options {
		c_key, c_value := C.CString(key), C.CString(value)
		c_numOptions = C.cupsAddOption(c_key, c_value, c_numOptions, &c_options)
		C.free(unsafe.Pointer(c_key))
		C.free(unsafe.Pointer(c_value))
	}

	c_user := C.CString(ownerID)
	defer C.free(unsafe.Pointer(c_user))

	if err := reconnect(c.c_http); err != nil {
		return 0, err
	}

	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.cupsSetUser(c_user)
	c_jobID := C.cupsPrintFile2(
		c.c_http, c_printerName, c_fileName, c_title, c_numOptions, c_options)
	jobID := uint32(c_jobID)
	if jobID == 0 {
		return 0, fmt.Errorf("Failed to call cupsPrintFile2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
	}

	return jobID, nil
}

// createJobURI creates a uri string for the job-uri attribute, used to get the
// status of a CUPS job.
func createJobURI(jobID uint32) (*C.char, error) {
	c_len := C.size_t(urlMaxLength)
	c_uri := (*C.char)(C.malloc(c_len))
	if c_uri == nil {
		return nil, errors.New("Failed to malloc; out of memory?")
	}

	c_resource := C.CString(fmt.Sprintf("/jobs/%d", jobID))
	defer C.free(unsafe.Pointer(c_resource))
	C.httpAssembleURI(C.HTTP_URI_CODING_ALL,
		c_uri, C.int(c_len), C.IPP, nil, C.cupsServer(), C.ippPort(), c_resource)

	return c_uri, nil
}
