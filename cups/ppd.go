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
#include <stdlib.h> // malloc, free
#include <string.h> // memset
#include <time.h>   // time_t
*/
import "C"
import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// This isn't really a cache, but an interface to CUPS' quirky PPD interface.
// The connector needs to know when a PPD changes, but the CUPS API can only:
// (1) fetch a PPD to a file
// (2) indicate whether a PPD file is up-to-date.
// So, this "cache":
// (1) maintains temporary file copies of PPDs for each printer
// (2) updates those PPD files as necessary
// (3) and keeps MD5 hashes of the PPD contents, to minimize disk I/O.
type ppdCache struct {
	c_http  *C.http_t
	cache   map[string]*ppdCacheEntry
	request chan ppdRequest
	q       chan bool
}

type ppdRequest struct {
	printerName string
	response    chan ppdResponse
}

type ppdResponse struct {
	filename string
	hash     string
	err      error
}

func newPPDCache(c_http *C.http_t) *ppdCache {
	cache := make(map[string]*ppdCacheEntry)
	pc := ppdCache{c_http, cache, make(chan ppdRequest), make(chan bool)}
	go pc.servePPDs()
	return &pc
}

func (pc *ppdCache) quit() {
	pc.q <- true
	<-pc.q
	for printerName, pce := range pc.cache {
		pce.free()
		delete(pc.cache, printerName)
	}
}

func (pc *ppdCache) getPPD(printerName string) (string, error) {
	ch := make(chan ppdResponse)
	request := ppdRequest{printerName, ch}
	pc.request <- request
	response := <-ch

	if response.err != nil {
		return "", response.err
	}
	ppd, err := ioutil.ReadFile(response.filename)
	if err != nil {
		return "", err
	}

	return string(ppd), nil
}

func (pc *ppdCache) getPPDHash(printerName string) (string, error) {
	ch := make(chan ppdResponse)
	request := ppdRequest{printerName, ch}
	pc.request <- request
	response := <-ch
	return response.hash, response.err
}

func (pc *ppdCache) servePPDs() {
	for {
		select {
		case r := <-pc.request:
			var err error
			pce, exists := pc.cache[r.printerName]
			if !exists {
				pce, err = createPPDCacheEntry(r.printerName)
				if err != nil {
					r.response <- ppdResponse{"", "", err}
					continue
				}
				pc.cache[r.printerName] = pce
			}
			if err = reconnect(pc.c_http); err != nil {
				r.response <- ppdResponse{"", "",
					fmt.Errorf("Failed to reconnect while serving a PPD for %s", r.printerName)}
			}
			if err = pce.refreshPPDCacheEntry(pc.c_http); err != nil {
				r.response <- ppdResponse{"", "", err}
				continue
			}
			r.response <- ppdResponse{pce.filename, pce.hash, nil}

		case <-pc.q:
			pc.q <- true
			return
		}
	}
}

// Holds persistent data needed for calling C.cupsGetPPD3.
type ppdCacheEntry struct {
	c_name    *C.char
	c_modtime C.time_t
	filename  string
	hash      string
}

// createPPDCacheEntry creates an instance of ppdCache with the name field set,
// all else empty. The caller must free the name and buffer fields with
// ppdCacheEntry.free()
func createPPDCacheEntry(name string) (*ppdCacheEntry, error) {
	c_name := C.CString(name)
	c_modtime := C.time_t(0)
	file, err := CreateTempFile()
	if err != nil {
		return nil, fmt.Errorf("Failed to create PPD cache entry file: %s", err)
	}
	filename := file.Name()

	defer file.Close()

	pce := &ppdCacheEntry{c_name, c_modtime, filename, ""}

	return pce, nil
}

// free frees the memory that stores the name and buffer fields, and deletes
// the file named by the buffer field. If the file doesn't exist, no error is
// returned.
func (pce *ppdCacheEntry) free() {
	C.free(unsafe.Pointer(pce.c_name))
	os.Remove(pce.filename)
}

// refreshPPDCacheEntry calls cupsGetPPD3() to refresh this PPD information, in
// case CUPS has a new PPD for the printer.
func (pce *ppdCacheEntry) refreshPPDCacheEntry(c_http *C.http_t) error {
	c_bufsize := C.size_t(syscall.PathMax)
	c_buffer := (*C.char)(C.malloc(c_bufsize))
	if c_buffer == nil {
		return errors.New("Failed to malloc; out of memory?")
	}
	defer C.free(unsafe.Pointer(c_buffer))
	C.memset(unsafe.Pointer(c_buffer), 0, c_bufsize)

	// Lock the OS thread so that thread-local storage is available to
	// cupsLastError() and cupsLastErrorString().
	runtime.LockOSThread()
	c_http_status := C.cupsGetPPD3(c_http, pce.c_name, &pce.c_modtime, c_buffer, c_bufsize)
	// Only use these values if the returned status is unacceptable.
	// Fetch them here so that we can unlock the OS thread immediately.
	c_cupsLastError, c_cupsLastErrorString := C.cupsLastError(), C.cupsLastErrorString()
	runtime.UnlockOSThread()
	defer os.Remove(C.GoString(c_buffer))

	switch c_http_status {
	case C.HTTP_STATUS_NOT_MODIFIED:
		// Cache hit.
		return nil

	case C.HTTP_STATUS_OK:
		// Cache miss.

		// Read from CUPS temporary file.
		r, err := os.Open(C.GoString(c_buffer))
		if err != nil {
			return err
		}
		// This was already C.free()'d as c_buffer, above.
		defer r.Close()

		// Copy to both of these through a MultiWriter.
		hash := md5.New()
		file, err := os.OpenFile(pce.filename, os.O_WRONLY|os.O_TRUNC, 0200)
		if err != nil {
			return fmt.Errorf("Failed to open already-created PPD cache file: %s", err)
		}
		defer file.Close()
		w := io.MultiWriter(hash, file)

		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		pce.hash = fmt.Sprintf("%x", hash.Sum(nil))

		return nil

	default:
		if c_cupsLastError != C.IPP_STATUS_OK {
			err := fmt.Errorf("Failed to call cupsGetPPD3(): %d %s",
				int(c_cupsLastError), C.GoString(c_cupsLastErrorString))
			return err
		}

		return fmt.Errorf("Failed to call cupsGetPPD3(); HTTP status: %d", int(c_http_status))
	}
}
