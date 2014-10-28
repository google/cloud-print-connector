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
#include <stdlib.h> // free
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

// Parses octets from IPP date format (RFC 2579).
void parseDate(ipp_uchar_t *ipp_date, unsigned short *year, unsigned char *month, unsigned char *day,
		unsigned char *hour, unsigned char *minutes, unsigned char *seconds, unsigned char *deciseconds,
		unsigned char *utcDirection, unsigned char *utcHours, unsigned char *utcMinutes) {
	*year = ntohs(*(unsigned short*)ipp_date);
	*month = ipp_date[2];
	*day = ipp_date[3];
	*hour = ipp_date[4];
	*minutes = ipp_date[5];
	*seconds = ipp_date[6];
	*deciseconds = ipp_date[7];
	*utcDirection = ipp_date[8] == '+' ? 1 : -1;
	*utcHours = ipp_date[9];
	*utcMinutes = ipp_date[10];
}
*/
import "C"
import (
	"cups-connector/lib"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/golang/glog"
)

// Interface between Go and the CUPS API.
type CUPS struct {
	c_http                *C.http_t
	pc                    *ppdCache
	infoToDisplayName     bool
	c_printerAttributes   **C.char
	printerAttributesSize int
	c_jobAttributes       **C.char
}

// Connects to the CUPS server specified by environment vars, client.conf, etc.
func NewCUPS(infoToDisplayName bool, printerAttributes []string) (*CUPS, error) {
	c_host := C.cupsServer()
	c_port := C.ippPort()
	c_encryption := C.cupsEncryption()

	c_http := C.httpConnectEncrypt(c_host, c_port, c_encryption)
	if c_http == nil {
		msg := fmt.Sprintf("Failed to connect to %s:%d", C.GoString(c_host), int(c_port))
		return nil, errors.New(msg)
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

	printerAttributesSize := len(printerAttributes)
	c_printerAttributes := C.newArrayOfStrings(C.int(printerAttributesSize))
	for i, a := range printerAttributes {
		C.setStringArrayValue(c_printerAttributes, C.int(i), C.CString(a))
	}

	c_jobAttributes := C.newArrayOfStrings(C.int(2))
	C.setStringArrayValue(c_jobAttributes, C.int(0), C.JOB_STATE)
	C.setStringArrayValue(c_jobAttributes, C.int(1), C.JOB_STATE_REASONS)

	c := &CUPS{c_http, pc, infoToDisplayName, c_printerAttributes, printerAttributesSize, c_jobAttributes}

	return c, nil
}

func (c *CUPS) Quit() {
	c.pc.quit()
	C.freeStringArrayAndStrings(c.c_printerAttributes, C.int(c.printerAttributesSize))
}

// Calls httpReconnect().
func (c *CUPS) reconnect() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	c_ippStatus := C.httpReconnect(c.c_http)
	if c_ippStatus != C.IPP_STATUS_OK {
		return errors.New(fmt.Sprintf("Failed to call cupsReconnect(): %d %s",
			int(c_ippStatus), C.GoString(C.cupsLastErrorString())))
	}
	return nil
}

// Calls cupsDoRequest() with IPP_OP_CUPS_GET_PRINTERS.
func (c *CUPS) GetPrinters() ([]lib.Printer, error) {
	// ippNewRequest() returns ipp_t pointer does not need explicit free.
	c_req := C.ippNewRequest(C.IPP_OP_CUPS_GET_PRINTERS)
	C.ippAddStrings(c_req, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		C.int(c.printerAttributesSize), nil, c.c_printerAttributes)

	if err := c.reconnect(); err != nil {
		return nil, err
	}

	runtime.LockOSThread()

	c_res := C.cupsDoRequest(c.c_http, c_req, C.POST_RESOURCE)
	if c_res == nil {
		msg := fmt.Sprintf(
			"Failed to call cupsDoRequest() [IPP_OP_CUPS_GET_PRINTERS]: %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		runtime.UnlockOSThread()
		return nil, errors.New(msg)
	}
	runtime.UnlockOSThread()

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(c_res)

	if C.ippGetStatusCode(c_res) == C.IPP_STATUS_ERROR_NOT_FOUND {
		// Normal error when there are no printers.
		return make([]lib.Printer, 0), nil
	} else if C.ippGetStatusCode(c_res) != C.IPP_STATUS_OK {
		msg := fmt.Sprintf(
			"Failed to call cupsDoRequest() [IPP_OP_CUPS_GET_PRINTERS], IPP status code: %d",
			C.ippGetStatusCode(c_res))
		return nil, errors.New(msg)
	}

	printers := make([]lib.Printer, 0, 1)

	for a := C.ippFirstAttribute(c_res); a != nil; a = C.ippNextAttribute(c_res) {
		if C.ippGetGroupTag(a) != C.IPP_TAG_PRINTER {
			continue
		}

		attributes := make([]*C.ipp_attribute_t, 0, c.printerAttributesSize)

		for ; a != nil && C.ippGetGroupTag(a) == C.IPP_TAG_PRINTER; a = C.ippNextAttribute(c_res) {
			attributes = append(attributes, a)
		}

		printer, err := c.printerFromAttributes(attributes)
		if err != nil {
			return nil, err
		}
		printers = append(printers, printer)

		if a == nil {
			break
		}
	}

	return printers, nil
}

// Converts slice of attributes to a Printer.
func (c *CUPS) printerFromAttributes(attributes []*C.ipp_attribute_t) (lib.Printer, error) {
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
				var c_year C.ushort
				var c_month, c_day, c_hour, c_minutes, c_seconds, c_deciSeconds, c_utcDirection, c_utcHours, c_utcMinutes C.uchar
				C.parseDate(c_date, &c_year, &c_month, &c_day, &c_hour, &c_minutes, &c_seconds, &c_deciSeconds, &c_utcDirection, &c_utcHours, &c_utcMinutes)
				l := time.FixedZone("", 60*int(uint8(c_utcDirection)*uint8(c_utcHours)*uint8(c_utcMinutes)))
				t := time.Date(int(c_year), (time.Month)(int(c_month)), int(c_day), int(c_hour), int(c_minutes), int(c_seconds), int(c_deciSeconds)*100000000, l)
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
				values = []string{"unknown"}
			}
		}

		value := strings.Join(values, ",")
		if value == "none" {
			value = ""
		}
		tags[key] = value
	}

	printerName, exists := tags["printer-name"]
	if !exists {
		return lib.Printer{}, errors.New("printer-name tag missing; did you remove it from the config file?")
	}
	ppdHash, err := c.pc.getPPDHash(printerName)
	if err != nil {
		return lib.Printer{}, err
	}

	p := lib.Printer{
		Name:        tags["printer-name"],
		Description: tags["printer-make-and-model"],
		Status:      lib.PrinterStatusFromString(tags["printer-state"]),
		CapsHash:    ppdHash,
		Tags:        tags,
	}
	if c.infoToDisplayName {
		p.DefaultDisplayName = tags["printer-info"]
	}

	return p, nil
}

// Gets the PPD for printer.
//
// Calls cupsGetPPD3().
func (c *CUPS) GetPPD(printerName string) (string, error) {
	return c.pc.getPPD(printerName)
}

// Gets the PPD hash, aka capsHash, for printer.
//
// Calls cupsGetPPD3().
func (c *CUPS) GetPPDHash(printerName string) (string, error) {
	return c.pc.getPPDHash(printerName)
}

// Calls cupsDoRequest() with IPP_OP_GET_JOB_ATTRIBUTES
func (c *CUPS) GetJobStatus(jobID uint32) (lib.JobStatus, string, error) {
	c_uri, err := createJobURI(jobID)
	if err != nil {
		return 0, "", err
	}
	defer C.free(unsafe.Pointer(c_uri))

	// ippNewRequest() returns ipp_t pointer does not need explicit free.
	c_req := C.ippNewRequest(C.IPP_OP_GET_JOB_ATTRIBUTES)

	C.ippAddString(c_req, C.IPP_TAG_OPERATION, C.IPP_TAG_URI, C.JOB_URI, nil, c_uri)
	C.ippAddStrings(c_req, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		C.int(0), nil, c.c_jobAttributes)

	if err := c.reconnect(); err != nil {
		return 0, "", err
	}

	runtime.LockOSThread()

	c_res := C.cupsDoRequest(c.c_http, c_req, C.POST_RESOURCE)
	if c_res == nil {
		msg := fmt.Sprintf("Failed to call cupsDoRequest() [IPP_OP_GET_JOB_ATTRIBUTES]: %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		runtime.UnlockOSThread()
		return 0, "", errors.New(msg)
	}
	runtime.UnlockOSThread()

	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(c_res)

	if C.ippGetStatusCode(c_res) != C.IPP_STATUS_OK {
		msg := fmt.Sprintf(
			"Failed to call cupsDoRequest() [IPP_OP_GET_JOB_ATTRIBUTES], IPP status code: %d",
			C.ippGetStatusCode(c_res))
		return 0, "", errors.New(msg)
	}

	c_status := C.ippFindAttribute(c_res, C.JOB_STATE, C.IPP_TAG_ENUM)
	status := lib.JobStatusFromInt(uint8(C.ippGetInteger(c_status, C.int(0))))

	c_statusReason := C.ippFindAttribute(c_res, C.JOB_STATE_REASONS, C.IPP_TAG_STRING)
	var statusReason string
	if c_statusReason != nil {
		statusReason = C.GoString(C.ippGetString(c_statusReason, C.int(0), nil))
	}

	return status, statusReason, nil
}

// Calls cupsSetUser() and cupsPrintFile2().
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

	if err := c.reconnect(); err != nil {
		return 0, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.cupsSetUser(c_user)
	c_jobID := C.cupsPrintFile2(c.c_http, c_printerName, c_fileName, c_title, c_numOptions, c_options)
	jobID := uint32(c_jobID)
	if jobID == 0 {
		msg := fmt.Sprintf("Failed to call cupsPrintFile2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return 0, errors.New(msg)
	}

	return jobID, nil
}

// Calls cupsTempFd().
func (c *CUPS) CreateTempFile() (*os.File, error) {
	c_len := C.size_t(200)
	c_filename := (*C.char)(C.malloc(c_len))
	if c_filename == nil {
		return nil, errors.New("Failed to malloc(); out of memory?")
	}
	defer C.free(unsafe.Pointer(c_filename))

	runtime.LockOSThread()

	c_fd := C.cupsTempFd(c_filename, C.int(c_len))
	if c_fd == C.int(-1) {
		msg := fmt.Sprintf("Failed to call cupsTempFd(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		runtime.UnlockOSThread()
		return nil, errors.New(msg)
	}

	runtime.UnlockOSThread()

	return os.NewFile(uintptr(c_fd), C.GoString(c_filename)), nil
}

// Creates a uri for the job-uri attribute.
//
// Calls httpAssembleURI().
func createJobURI(jobID uint32) (*C.char, error) {
	c_len := C.size_t(200)
	c_uri := (*C.char)(C.malloc(c_len))
	if c_uri == nil {
		return nil, errors.New("Failed to malloc; out of memory?")
	}

	c_resource := C.CString(fmt.Sprintf("/jobs/%d", jobID))
	defer C.free(unsafe.Pointer(c_resource))
	C.httpAssembleURI(C.HTTP_URI_CODING_ALL, c_uri, C.int(c_len), C.IPP, nil, C.cupsServer(), C.ippPort(), c_resource)

	return c_uri, nil
}
