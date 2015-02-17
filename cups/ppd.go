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
	"sync"
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
	cc         *cupsCore
	cache      map[string]*ppdCacheEntry
	cacheMutex sync.RWMutex
}

func newPPDCache(cc *cupsCore) *ppdCache {
	cache := make(map[string]*ppdCacheEntry)
	pc := ppdCache{cc, cache, sync.RWMutex{}}
	return &pc
}

func (pc *ppdCache) quit() {
	pc.cacheMutex.Lock()
	defer pc.cacheMutex.Unlock()

	for printername, pce := range pc.cache {
		pce.free()
		delete(pc.cache, printername)
	}
}

// removePPD removes a cache entry from the cache.
func (pc *ppdCache) removePPD(printername string) {
	pc.cacheMutex.Lock()
	defer pc.cacheMutex.Unlock()

	if pce, exists := pc.cache[printername]; exists {
		pce.free()
		delete(pc.cache, printername)
	}
}

func (pc *ppdCache) getPPD(printername string) (string, error) {
	filename, _, err := pc.getPPDCacheEntry(printername)
	if err != nil {
		return "", err
	}

	ppd, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(ppd), nil
}

func (pc *ppdCache) getPPDHash(printername string) (string, error) {
	_, hash, err := pc.getPPDCacheEntry(printername)
	return hash, err
}

func (pc *ppdCache) getPPDCacheEntry(printername string) (string, string, error) {
	pc.cacheMutex.RLock()
	pce, exists := pc.cache[printername]
	pc.cacheMutex.RUnlock()

	if !exists {
		pce, err := createPPDCacheEntry(printername)
		if err != nil {
			return "", "", err
		}
		if err = pce.refresh(pc.cc); err != nil {
			return "", "", err
		}

		pc.cacheMutex.Lock()
		defer pc.cacheMutex.Unlock()

		if firstPCE, exists := pc.cache[printername]; exists {
			// Two entries were created at the same time. Remove the older one.
			delete(pc.cache, printername)
			go firstPCE.free()
		}
		pc.cache[printername] = pce
		filename, hash := pce.getFilenameAndHash()
		return filename, hash, nil

	} else {
		if err := pce.refresh(pc.cc); err != nil {
			return "", "", err
		}
		filename, hash := pce.getFilenameAndHash()
		return filename, hash, nil
	}
}

// Holds persistent data needed for calling C.cupsGetPPD3.
type ppdCacheEntry struct {
	printername *C.char
	modtime     C.time_t
	filename    string
	hash        string
	// mutex protects fields modtime, filename, and hash.
	mutex sync.Mutex
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

	pce := &ppdCacheEntry{C.CString(name), C.time_t(0), filename, "", sync.Mutex{}}

	return pce, nil
}

func (pce *ppdCacheEntry) getFilenameAndHash() (string, string) {
	pce.mutex.Lock()
	defer pce.mutex.Unlock()
	return pce.filename, pce.hash
}

// free frees the memory that stores the name and buffer fields, and deletes
// the file named by the buffer field. If the file doesn't exist, no error is
// returned.
func (pce *ppdCacheEntry) free() {
	pce.mutex.Lock()
	defer pce.mutex.Unlock()

	C.free(unsafe.Pointer(pce.printername))
	os.Remove(pce.filename)
}

// refresh calls cupsGetPPD3() to refresh this PPD information, in
// case CUPS has a new PPD for the printer.
func (pce *ppdCacheEntry) refresh(cc *cupsCore) error {
	pce.mutex.Lock()
	defer pce.mutex.Unlock()

	ppdFilename, err := cc.getPPD(pce.printername, &pce.modtime)
	if err != nil {
		return err
	}

	if ppdFilename == nil {
		// Cache hit.
		return nil
	}

	// (else) Cache miss.
	defer C.free(unsafe.Pointer(ppdFilename))
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
