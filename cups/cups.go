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
    *PRINTER_INFO = "printer-info",
    *PRINTER_ACCEPTING = "printer-is-accepting-jobs",
    *PRINTER_MAKE_MODEL = "printer-make-and-model",
    *PRINTER_STATE = "printer-state",
    *REQUESTED_ATTRIBUTES = "requested-attributes";

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
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// Interface between Go and the CUPS API.
type CUPS struct {
	c_http                *C.http_t
	pc                    *ppdCache
	infoToDisplayName     bool
	c_printerAttributes   **C.char
	printerAttributesSize int
}

// Connects to the CUPS server specified by environment vars, client.conf, etc.
func NewCUPS(infoToDisplayName bool, printerAttributes []string) (*CUPS, error) {
	printerAttributesSize := len(printerAttributes)
	c_printerAttributes := C.newArrayOfStrings(C.int(printerAttributesSize))
	for i, a := range printerAttributes {
		C.setStringArrayValue(c_printerAttributes, C.int(i), C.CString(a))
	}

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

	fmt.Printf("connected to CUPS server %s:%d %s\n", C.GoString(c_host), int(c_port), e)

	pc := newPPDCache(c_http)
	c := &CUPS{c_http, pc, infoToDisplayName, c_printerAttributes, printerAttributesSize}

	return c, nil
}

func (c *CUPS) Quit() {
	c.pc.quit()
	C.freeStringArrayAndStrings(c.c_printerAttributes, C.int(c.printerAttributesSize))
}

// Calls cupsGetDests2().
func (c *CUPS) GetDests() ([]lib.Printer, error) {
	var c_dests *C.cups_dest_t
	c_numDests := C.cupsGetDests2(c.c_http, &c_dests)
	if c_numDests < 0 {
		msg := fmt.Sprintf("CUPS failed to call cupsGetDests2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(msg)
	}
	defer C.cupsFreeDests(c_numDests, c_dests)

	numDests := int(c_numDests)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(c_dests)),
		Len:  numDests,
		Cap:  numDests,
	}
	dests := *(*[]C.cups_dest_t)(unsafe.Pointer(&hdr))

	printers := make([]lib.Printer, 0, len(dests))
	for i := 0; i < len(dests); i++ {
		c_dest := dests[i]
		printer, err := c.destToPrinter(&c_dest)
		if err != nil {
			return nil, err
		}
		printers = append(printers, printer)
	}

	return printers, nil
}

// Calls cupsDoRequest() with IPP_OP_CUPS_GET_PRINTERS.
func (c *CUPS) GetPrinters() ([]lib.Printer, error) {
	// ippNewRequest() returned ipp_t pointer does not need explicit free.
	c_req := C.ippNewRequest(C.IPP_OP_CUPS_GET_PRINTERS)
	C.ippAddStrings(c_req, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		C.int(c.printerAttributesSize), nil, c.c_printerAttributes)

	c_res := C.cupsDoRequest(c.c_http, c_req, C.POST_RESOURCE)
	if c_res == nil {
		msg := fmt.Sprintf("CUPS failed to call cupsDoRequest(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(msg)
	}
	// cupsDoRequest() returned ipp_t pointer needs explicit free.
	defer C.ippDelete(c_res)

	if C.ippGetStatusCode(c_res) != C.IPP_STATUS_OK {
		msg := fmt.Sprintf("CUPS failed while calling cupsDoRequest(), IPP status code: %d",
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

// Converts a cups_dest_t to a *lib.Printer.
func (c *CUPS) destToPrinter(c_dest *C.cups_dest_t) (lib.Printer, error) {
	name := C.GoString(c_dest.name)

	var info string
	c_info := C.cupsGetOption(C.PRINTER_INFO, c_dest.num_options, c_dest.options)
	if c_info != nil {
		info = C.GoString(c_info)
	}

	var makeModel string
	c_makeModel := C.cupsGetOption(C.PRINTER_MAKE_MODEL, c_dest.num_options, c_dest.options)
	if c_makeModel != nil {
		makeModel = C.GoString(c_makeModel)
	}

	acceptingJobs := true
	c_acceptingJobs := C.cupsGetOption(C.PRINTER_ACCEPTING, c_dest.num_options, c_dest.options)
	if c_acceptingJobs != nil {
		acceptingJobs = C.GoString(c_acceptingJobs) != "false"
	}
	var status lib.PrinterStatus
	if acceptingJobs {
		c_printerState := C.cupsGetOption(C.PRINTER_STATE, c_dest.num_options, c_dest.options)
		status = lib.PrinterStatusFromString(C.GoString(c_printerState))
	} else {
		status = lib.PrinterStopped
	}

	ppdHash, err := c.pc.getPPDHash(name)
	if err != nil {
		return lib.Printer{}, err
	}

	printer := lib.Printer{
		Name:        name,
		Description: makeModel,
		Status:      status,
		CapsHash:    ppdHash,
	}
	if c.infoToDisplayName {
		printer.DefaultDisplayName = info
	}
	return printer, nil
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

// Gets the status of all jobs, jobID:status map.
//
// Calls cupsGetJobs2().
func (c *CUPS) GetJobs() (map[uint32]lib.JobStatus, error) {
	// TODO: Get status message string like pycups Connection.getJobAttributes().
	var c_jobs *C.cups_job_t
	c_numJobs := C.cupsGetJobs2(c.c_http, &c_jobs, nil, 1, C.CUPS_WHICHJOBS_ALL)
	if c_numJobs < 0 {
		msg := fmt.Sprintf("CUPS failed to call cupsGetJobs2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(msg)
	}
	defer C.cupsFreeJobs(c_numJobs, c_jobs)

	numJobs := int(c_numJobs)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(c_jobs)),
		Len:  numJobs,
		Cap:  numJobs,
	}
	jobs := *(*[]C.cups_job_t)(unsafe.Pointer(&hdr))

	m := make(map[uint32]lib.JobStatus, numJobs)
	for i := 0; i < numJobs; i++ {
		jobID := uint32(jobs[i].id)
		state := lib.JobStatusFromInt(uint8(jobs[i].state))
		m[jobID] = state
	}

	return m, nil
}

// Calls cupsPrintFile2().
func (c *CUPS) Print(printerName, fileName, title string) (uint32, error) {
	c_printerName := C.CString(printerName)
	defer C.free(unsafe.Pointer(c_printerName))
	c_fileName := C.CString(fileName)
	defer C.free(unsafe.Pointer(c_fileName))
	c_title := C.CString(title)
	defer C.free(unsafe.Pointer(c_title))

	c_jobID := C.cupsPrintFile2(c.c_http, c_printerName, c_fileName, c_title, 0, nil)
	jobID := uint32(c_jobID)
	if jobID == 0 {
		msg := fmt.Sprintf("CUPS failed call cupsPrintFile2(): %d %s",
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
		return nil, errors.New("Failed to malloc; out of memory?")
	}
	defer C.free(unsafe.Pointer(c_filename))

	c_fd := C.cupsTempFd(c_filename, C.int(c_len))

	return os.NewFile(uintptr(c_fd), C.GoString(c_filename)), nil
}
