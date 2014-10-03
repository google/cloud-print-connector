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
*/
import "C"
import (
	"cups-connector/lib"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unsafe"
)

// These variables would be C.free()'d, but we treat them like constants.
var (
	cupsPrinterInfo      *C.char = C.CString("printer-info")
	cupsPrinterAccepting *C.char = C.CString("printer-is-accepting-jobs")
	cupsPrinterLocation  *C.char = C.CString("printer-location")
	cupsPrinterMakeModel *C.char = C.CString("printer-make-and-model")
	cupsPrinterState     *C.char = C.CString("printer-state")
	cupsGCPID            *C.char = C.CString("gcp-id")
)

// Interface between Go and the CUPS API.
type CUPS struct {
	httpConnection    *C.http_t
	pc                *ppdCache
	infoToDisplayName bool
}

// Connects to the CUPS server specified by environment vars, client.conf, etc.
func NewCUPS(infoToDisplayName bool) (*CUPS, error) {
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
	c := &CUPS{c_http, pc, infoToDisplayName}

	return c, nil
}

func (c *CUPS) Quit() {
	c.pc.quit()
}

// Calls cupsGetDests2().
func (c *CUPS) GetDests() ([]lib.Printer, error) {
	var c_dests *C.cups_dest_t
	c_numDests := C.cupsGetDests2(c.httpConnection, &c_dests)
	if c_numDests < 0 {
		text := fmt.Sprintf("CUPS failed to call cupsGetDests2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(text)
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

// Converts a cups_dest_t to a *lib.Printer.
func (c *CUPS) destToPrinter(c_dest *C.cups_dest_t) (lib.Printer, error) {
	name := C.GoString(c_dest.name)

	var info string
	c_info := C.cupsGetOption(cupsPrinterInfo, c_dest.num_options, c_dest.options)
	if c_info != nil {
		info = C.GoString(c_info)
	}

	var makeModel string
	c_makeModel := C.cupsGetOption(cupsPrinterMakeModel, c_dest.num_options, c_dest.options)
	if c_makeModel != nil {
		makeModel = C.GoString(c_makeModel)
	}

	acceptingJobs := true
	c_acceptingJobs := C.cupsGetOption(cupsPrinterAccepting, c_dest.num_options, c_dest.options)
	if c_acceptingJobs != nil {
		acceptingJobs = C.GoString(c_acceptingJobs) != "false"
	}
	var status lib.PrinterStatus
	if acceptingJobs {
		c_printerState := C.cupsGetOption(cupsPrinterState, c_dest.num_options, c_dest.options)
		status = lib.PrinterStatusFromString(C.GoString(c_printerState))
	} else {
		status = lib.PrinterStopped
	}

	var location string
	c_printerLocation := C.cupsGetOption(cupsPrinterLocation, c_dest.num_options, c_dest.options)
	if c_printerLocation != nil {
		location = C.GoString(c_printerLocation)
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
		Location:    location,
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
	c_numJobs := C.cupsGetJobs2(c.httpConnection, &c_jobs, nil, 1, C.CUPS_WHICHJOBS_ALL)
	if c_numJobs < 0 {
		text := fmt.Sprintf("CUPS failed to call cupsGetJobs2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(text)
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

	c_jobID := C.cupsPrintFile2(c.httpConnection, c_printerName, c_fileName, c_title, 0, nil)
	jobID := uint32(c_jobID)
	if jobID == 0 {
		text := fmt.Sprintf("CUPS failed call cupsPrintFile2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return 0, errors.New(text)
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
