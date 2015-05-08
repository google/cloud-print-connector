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
#include <stddef.h>     // size_t
#include <stdlib.h>     // free, malloc
#include <sys/socket.h> // AF_UNSPEC
#include <time.h>       // time_t

const char
    *POST_RESOURCE        = "/",
    *REQUESTED_ATTRIBUTES = "requested-attributes",
		*JOB_URI_ATTRIBUTE    = "job-uri",
		*IPP                  = "ipp";
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/google/cups-connector/lib"

	"github.com/golang/glog"
)

const (
	// jobURIFormat is the string format required by the CUPS API
	// to do things like query the state of a job.
	jobURIFormat = "/jobs/%d"
)

// cupsCore handles CUPS API interaction and connection management.
type cupsCore struct {
	host           *C.char
	port           C.int
	encryption     C.http_encryption_t
	connectTimeout C.int
	// connectionSemaphore limits the quantity of open CUPS connections.
	connectionSemaphore *lib.Semaphore
	// connectionPool allows a connection to be reused instead of closed.
	connectionPool chan *C.http_t
	hostIsLocal    bool
}

func newCUPSCore(maxConnections uint, connectTimeout time.Duration) (*cupsCore, error) {
	host := C.cupsServer()
	port := C.ippPort()
	encryption := C.cupsEncryption()
	timeout := C.int(connectTimeout / time.Millisecond)

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

	var hostIsLocal bool
	if h := C.GoString(host); strings.HasPrefix(h, "/") || h == "localhost" {
		hostIsLocal = true
	}

	cs := lib.NewSemaphore(maxConnections)
	cp := make(chan *C.http_t)

	cc := &cupsCore{host, port, encryption, timeout, cs, cp, hostIsLocal}

	// This connection isn't used, just checks that a connection is possible
	// before returning from the constructor.
	http, err := cc.connect()
	if err != nil {
		return nil, err
	}
	cc.disconnect(http)

	glog.Infof("connected to CUPS server %s:%d %s\n", C.GoString(host), int(port), e)

	return cc, nil
}

// printFile prints by calling C.cupsPrintFile2().
// Returns the CUPS job ID, which is 0 (and meaningless) when err
// is not nil.
func (cc *cupsCore) printFile(user, printername, filename, title *C.char, numOptions C.int, options *C.cups_option_t) (C.int, error) {
	http, err := cc.connect()
	if err != nil {
		return 0, err
	}
	defer cc.disconnect(http)

	C.cupsSetUser(user)
	jobID := C.cupsPrintFile2(http, printername, filename, title, numOptions, options)
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

	response, err := cc.doRequestWithRetry(request,
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
// The caller is responsible to C.free the returned *C.char filename
// if the returned filename is not nil.
func (cc *cupsCore) getPPD(printername *C.char, modtime *C.time_t) (*C.char, error) {
	bufsize := C.size_t(filePathMaxLength)
	buffer := (*C.char)(C.malloc(bufsize))
	if buffer == nil {
		return nil, errors.New("Failed to malloc; out of memory?")
	}
	C.memset(unsafe.Pointer(buffer), 0, bufsize)

	var http *C.http_t
	if !cc.hostIsLocal {
		// Don't need a connection or corresponding semaphore if the PPD
		// is on the local filesystem.
		// Still need OS thread lock; see else.
		var err error
		http, err = cc.connect()
		if err != nil {
			return nil, err
		}
		defer cc.disconnect(http)

	} else {
		// Lock the OS thread so that thread-local storage is available to
		// cupsLastError() and cupsLastErrorString().
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	httpStatus := C.cupsGetPPD3(http, printername, modtime, buffer, bufsize)

	switch httpStatus {
	case C.HTTP_STATUS_NOT_MODIFIED:
		// Cache hit.
		if len(C.GoString(buffer)) > 0 {
			os.Remove(C.GoString(buffer))
		}
		C.free(unsafe.Pointer(buffer))
		return nil, nil

	case C.HTTP_STATUS_OK:
		// Cache miss.
		return buffer, nil

	default:
		if len(C.GoString(buffer)) > 0 {
			os.Remove(C.GoString(buffer))
		}
		C.free(unsafe.Pointer(buffer))
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

	response, err := cc.doRequestWithRetry(request, []C.ipp_status_t{C.IPP_STATUS_OK})
	if err != nil {
		err = fmt.Errorf("Failed to call cupsDoRequest() [IPP_OP_GET_JOB_ATTRIBUTES]: %s", err)
		return nil, err
	}

	return response, nil
}

// createJobURI creates a uri string for the job-uri attribute, used to get the
// state of a CUPS job.
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
func (cc *cupsCore) doRequestWithRetry(request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	response, err := cc.doRequest(request, acceptableStatusCodes)
	if err == nil {
		return response, err
	}

	return cc.doRequest(request, acceptableStatusCodes)
}

// doRequest calls cupsDoRequest().
func (cc *cupsCore) doRequest(request *C.ipp_t, acceptableStatusCodes []C.ipp_status_t) (*C.ipp_t, error) {
	http, err := cc.connect()
	if err != nil {
		return nil, err
	}
	defer cc.disconnect(http)

	if C.ippValidateAttributes(request) != 1 {
		return nil, fmt.Errorf("Bad IPP request: %s", C.GoString(C.cupsLastErrorString()))
	}

	response := C.cupsDoRequest(http, request, C.POST_RESOURCE)
	if response == nil {
		return nil, fmt.Errorf("cupsDoRequest failed: %d %s", int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
	}
	statusCode := C.ippGetStatusCode(response)
	for _, sc := range acceptableStatusCodes {
		if statusCode == sc {
			return response, nil
		}
	}

	return nil, fmt.Errorf("IPP status code %d", int(statusCode))
}

// connect calls C.httpConnect2 to create a new, open connection to
// the CUPS server specified by environment variables, client.conf, etc.
//
// connect also acquires the connection semaphore and locks the OS
// thread to allow the CUPS API to use thread-local storage cleanly.
//
// The caller is responsible to close the connection when finished
// using cupsCore.disconnect.
func (cc *cupsCore) connect() (*C.http_t, error) {
	cc.connectionSemaphore.Acquire()

	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()

	var http *C.http_t

	select {
	case h := <-cc.connectionPool:
		// Reuse another connection.
		http = h
	default:
		// No connection available for reuse; create a new one.
		http = C.httpConnect2(cc.host, cc.port, nil, C.AF_UNSPEC, cc.encryption, 1, cc.connectTimeout, nil)
		if http == nil {
			defer cc.disconnect(http)
			return nil, fmt.Errorf("Failed to connect to CUPS server %s:%d because %d %s",
				C.GoString(cc.host), int(cc.port), int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		}
	}

	return http, nil
}

// disconnect calls C.httpClose to close an open CUPS connection, then
// unlocks the OS thread and the connection semaphore.
//
// The http argument may be nil; the OS thread and semaphore are still
// treated the same as described above.
func (cc *cupsCore) disconnect(http *C.http_t) {
	go func() {
		select {
		case cc.connectionPool <- http:
			// Hand this connection to the next guy who needs it.
		case <-time.After(time.Second):
			// Don't wait very long; stale connections are no fun.
			C.httpClose(http)
		}
	}()
	runtime.UnlockOSThread()
	cc.connectionSemaphore.Release()
}

func (cc *cupsCore) connQtyOpen() uint {
	return cc.connectionSemaphore.Count()
}

func (cc *cupsCore) connQtyMax() uint {
	return cc.connectionSemaphore.Size()
}
