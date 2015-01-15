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
#include <time.h>   // time_t

const char
    *POST_RESOURCE        = "/",
    *REQUESTED_ATTRIBUTES = "requested-attributes",
		*JOB_URI_ATTRIBUTE    = "job-uri",
		*IPP                  = "ipp";
*/
import "C"
import (
	"cups-connector/lib"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/golang/glog"
)

const (
	maxConnectionAge = "2m"
	jobURIFormat     = "/jobs/%d"
)

// cupsCore protects the CUPS C.http_t connection with a mutex. Although
// the CUPS API claims that it is thread-safe, this library panics under
// very little pressure without the mutex.
type cupsCore struct {
	http             *C.http_t
	lastConnect      time.Time
	maxConnectionAge time.Duration
	httpMutex        *lib.Semaphore
}

// newCUPSCore calls C.httpConnectEncrypt to create a new, open
// connection to the CUPS server specified by environment variables,
// client.conf, etc.
func newCUPSCore() (*cupsCore, error) {
	host := C.cupsServer()
	port := C.ippPort()
	encryption := C.cupsEncryption()

	http := C.httpConnectEncrypt(host, port, encryption)
	if http == nil {
		return nil, fmt.Errorf("Failed to connect to %s:%d", C.GoString(host), int(port))
	}

	var e string
	switch encryption {
	case C.HTTP_ENCRYPTION_ALWAYS:
		e = "encrypting ALWAYS"
	case C.HTTP_ENCRYPTION_IF_REQUESTED:
		e = "encrypting IF REQUESTED"
	case C.HTTP_ENCRYPTION_NEVER:
		e = "encrypting NEVER"
	case C.HTTP_ENCRYPTION_REQUIRED:
		e = "encryption REQUIRED"
	default:
		encryption = C.HTTP_ENCRYPTION_REQUIRED
		e = "encrypting REQUIRED"
	}

	parsedMaxConnectionAge, err := time.ParseDuration(maxConnectionAge)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse max CUPS connection time: %s", err)
	}

	glog.Infof("connected to CUPS server %s:%d %s\n", C.GoString(host), int(port), e)

	cc := &cupsCore{
		http:             http,
		lastConnect:      time.Now(),
		maxConnectionAge: parsedMaxConnectionAge,
		httpMutex:        lib.NewSemaphore(1),
	}

	return cc, nil
}

// printFile prints by calling C.cupsPrintFile2().
// Returns the CUPS job ID, which is 0 (and meaningless) when err
// is not nil.
func (cc *cupsCore) printFile(user, printername, filename, title *C.char, numOptions C.int, options *C.cups_option_t) (C.int, error) {
	cc.httpMutex.Acquire()
	defer cc.httpMutex.Release()

	if err := cc.reconnectIfNeeded(); err != nil {
		return 0, err
	}

	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.cupsSetUser(user)
	jobID := C.cupsPrintFile2(cc.http, printername, filename, title, numOptions, options)
	if jobID == 0 {
		return 0, fmt.Errorf("Failed to call cupsPrintFile2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
	}

	return jobID, nil
}

// getPrinters gets the current list and state of printers by calling
// C.doRequest (IPP_OP_CUPS_GET_PRINTERS).
//
// The caller is responsible to C.ippDelete the returned *C.ipp_t response.
func (cc *cupsCore) getPrinters(attributes **C.char, attrSize C.int) (*C.ipp_t, error) {
	// ippNewRequest() returns ipp_t pointer which does not need explicit free.
	request := C.ippNewRequest(C.IPP_OP_CUPS_GET_PRINTERS)
	C.ippAddStrings(request, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		attrSize, nil, attributes)

	cc.httpMutex.Acquire()
	defer cc.httpMutex.Release()

	if err := cc.reconnectIfNeeded(); err != nil {
		return nil, err
	}

	response, err := doRequestWithRetry(cc.http, request,
		[]C.ipp_status_t{C.IPP_STATUS_OK, C.IPP_STATUS_ERROR_NOT_FOUND})
	if err != nil {
		err = fmt.Errorf("Failed to call cupsDoRequest() [IPP_OP_CUPS_GET_PRINTERS]: %s", err)
		return nil, err
	}

	return response, nil
}

// getPPD gets the filename of the PPD for a printer by calling
// C.cupsGetPPD3. If the PPD hasn't changed since the time indicated
// by modtime, then the returned filename is a nil pointer.
//
// Note that modtime is a pointer whose value is changed by this
// function.
//
// The caller is responsible to C.free the returned *C.char filename.
func (cc *cupsCore) getPPD(printername *C.char, modtime *C.time_t) (*C.char, error) {
	bufsize := C.size_t(syscall.PathMax)
	buffer := (*C.char)(C.malloc(bufsize))
	if buffer == nil {
		return nil, errors.New("Failed to malloc; out of memory?")
	}
	C.memset(unsafe.Pointer(buffer), 0, bufsize)

	cc.httpMutex.Acquire()
	defer cc.httpMutex.Release()

	if err := cc.reconnectIfNeeded(); err != nil {
		return nil, fmt.Errorf("Failed to reconnect while serving a PPD for %s", printername)
	}

	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	httpStatus := C.cupsGetPPD3(cc.http, printername, modtime, buffer, bufsize)

	switch httpStatus {
	case C.HTTP_STATUS_NOT_MODIFIED:
		// Cache hit.
		return nil, nil

	case C.HTTP_STATUS_OK:
		// Cache miss.
		return buffer, nil

	default:
		cupsLastError := C.cupsLastError()
		if cupsLastError != C.IPP_STATUS_OK {
			return nil, fmt.Errorf("Failed to call cupsGetPPD3(): %d %s",
				int(cupsLastError), C.GoString(C.cupsLastErrorString()))
		}

		return nil, fmt.Errorf("Failed to call cupsGetPPD3(); HTTP status: %d", int(httpStatus))
	}
}

// getJobAttributes gets the requested attributes for a job by calling
// C.doRequest (IPP_OP_GET_JOB_ATTRIBUTES).
//
// The caller is responsible to C.ippDelete the returned *C.ipp_t response.
func (cc *cupsCore) getJobAttributes(jobID C.int, attributes **C.char) (*C.ipp_t, error) {
	uri, err := createJobURI(jobID)
	if err != nil {
		return nil, err
	}
	defer C.free(unsafe.Pointer(uri))

	// ippNewRequest() returns ipp_t pointer does not need explicit free.
	request := C.ippNewRequest(C.IPP_OP_GET_JOB_ATTRIBUTES)

	C.ippAddString(request, C.IPP_TAG_OPERATION, C.IPP_TAG_URI, C.JOB_URI_ATTRIBUTE, nil, uri)
	C.ippAddStrings(request, C.IPP_TAG_OPERATION, C.IPP_TAG_KEYWORD, C.REQUESTED_ATTRIBUTES,
		C.int(0), nil, attributes)

	cc.httpMutex.Acquire()
	defer cc.httpMutex.Release()

	if err := cc.reconnectIfNeeded(); err != nil {
		return nil, err
	}

	response, err := doRequestWithRetry(cc.http, request, []C.ipp_status_t{C.IPP_STATUS_OK})
	if err != nil {
		err = fmt.Errorf("Failed to call cupsDoRequest() [IPP_OP_GET_JOB_ATTRIBUTES]: %s", err)
		return nil, err
	}

	return response, nil
}

// createJobURI creates a uri string for the job-uri attribute, used to get the
// status of a CUPS job.
func createJobURI(jobID C.int) (*C.char, error) {
	length := C.size_t(urlMaxLength)
	uri := (*C.char)(C.malloc(length))
	if uri == nil {
		return nil, errors.New("Failed to malloc; out of memory?")
	}

	resource := C.CString(fmt.Sprintf(jobURIFormat, uint32(jobID)))
	defer C.free(unsafe.Pointer(resource))
	C.httpAssembleURI(C.HTTP_URI_CODING_ALL,
		uri, C.int(length), C.IPP, nil, C.cupsServer(), C.ippPort(), resource)

	return uri, nil
}

// doRequestWithRetry calls doRequest and retries once on failure.
func doRequestWithRetry(http *C.http_t, request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	response, err := doRequest(http, request, acceptableStatusCodes)
	if err == nil {
		return response, err
	}

	return doRequest(http, request, acceptableStatusCodes)
}

// doRequest calls cupsDoRequest().
func doRequest(http *C.http_t, request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	response := C.cupsDoRequest(http, request, C.POST_RESOURCE)
	if response == nil {
		return nil, fmt.Errorf("%d %s", int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
	}
	statusCode := C.ippGetStatusCode(response)
	for _, sc := range acceptableStatusCodes {
		if statusCode == sc {
			return response, nil
		}
	}

	return nil, fmt.Errorf("IPP status code %d", int(statusCode))
}

// reconnectIfNeeded checks the age of the current CUPS connection.
// If too old, then it reconnects, else nothing happens.
func (cc *cupsCore) reconnectIfNeeded() error {
	if time.Since(cc.lastConnect) < cc.maxConnectionAge {
		return nil
	}

	if err := reconnect(cc.http); err != nil {
		return err
	}

	cc.lastConnect = time.Now()
	return nil
}

// reconnect calls C.httpReconnect, which re-starts the connection to
// the CUPS server.
func reconnect(http *C.http_t) error {
	// Lock the OS thread so that thread-local storage is available to
	// cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ippStatus := C.httpReconnect(http)
	if ippStatus != C.IPP_STATUS_OK {
		return fmt.Errorf("Failed to call cupsReconnect(): %d %s",
			int(ippStatus), C.GoString(C.cupsLastErrorString()))
	}
	return nil
}
