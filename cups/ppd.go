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
#include <stdlib.h> // free
#include <time.h>   // time_t
*/
import "C"
import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	cc      *cupsCore
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

func newPPDCache(cc *cupsCore) *ppdCache {
	cache := make(map[string]*ppdCacheEntry)
	pc := ppdCache{cc, cache, make(chan ppdRequest), make(chan bool)}
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
			if err = pce.refreshPPDCacheEntry(pc.cc); err != nil {
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
	printername *C.char
	modtime     C.time_t
	filename    string
	hash        string
}

// createPPDCacheEntry creates an instance of ppdCache with the name field set,
// all else empty. The caller must free the name and buffer fields with
// ppdCacheEntry.free()
func createPPDCacheEntry(name string) (*ppdCacheEntry, error) {
	file, err := CreateTempFile()
	if err != nil {
		return nil, fmt.Errorf("Failed to create PPD cache entry file: %s", err)
	}
	defer file.Close()
	filename := file.Name()

	pce := &ppdCacheEntry{C.CString(name), C.time_t(0), filename, ""}

	return pce, nil
}

// free frees the memory that stores the name and buffer fields, and deletes
// the file named by the buffer field. If the file doesn't exist, no error is
// returned.
func (pce *ppdCacheEntry) free() {
	C.free(unsafe.Pointer(pce.printername))
	os.Remove(pce.filename)
}

// refreshPPDCacheEntry calls cupsGetPPD3() to refresh this PPD information, in
// case CUPS has a new PPD for the printer.
func (pce *ppdCacheEntry) refreshPPDCacheEntry(cc *cupsCore) error {
	ppdFilename, err := cc.getPPD(pce.printername, &pce.modtime)
	if err != nil {
		return err
	}

	if ppdFilename == nil {
		// Cache hit.
		return nil
	}

	// (else) Cache miss.
	defer os.Remove(C.GoString(ppdFilename))

	// Read from CUPS temporary file.
	r, err := os.Open(C.GoString(ppdFilename))
	if err != nil {
		return err
	}
	defer r.Close()

	// Write to this PPD cache file.
	file, err := os.OpenFile(pce.filename, os.O_WRONLY|os.O_TRUNC, 0200)
	if err != nil {
		return fmt.Errorf("Failed to open already-created PPD cache file: %s", err)
	}
	defer file.Close()

	// Also write to an MD5 hash.
	hash := md5.New()
	w := io.MultiWriter(hash, file)

	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	pce.hash = fmt.Sprintf("%x", hash.Sum(nil))

	return nil
}
