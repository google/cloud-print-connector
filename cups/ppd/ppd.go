package ppd

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
	"log"
	"os"
	"reflect"
	"unsafe"
)

type PPDCache struct {
	httpConnection *C.http_t
	m              map[string]*ppdCacheEntry
	request        chan ppdRequest
	q              chan bool
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

func NewPPDCache(httpConnection *C.http_t) *PPDCache {
	m := make(map[string]*ppdCacheEntry)
	pc := PPDCache{httpConnection, m, make(chan ppdRequest), make(chan bool)}
	go pc.servePPDs()
	return &pc
}

func (pc *PPDCache) Quit() {
	pc.q <- true
	<-pc.q
	for printerName, pce := range pc.m {
		pce.free()
		delete(pc.m, printerName)
	}
}

func (pc *PPDCache) GetPPD(printerName string) (string, error) {
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

func (pc *PPDCache) GetPPDHash(printerName string) (string, error) {
	ch := make(chan ppdResponse)
	request := ppdRequest{printerName, ch}
	pc.request <- request
	response := <-ch
	return response.hash, response.err
}

func (pc *PPDCache) getPrinterNames() ([]string, error) {
	var c_dests *C.cups_dest_t
	c_num_dests := C.cupsGetDests2(pc.httpConnection, &c_dests)
	if c_num_dests < 0 {
		text := fmt.Sprintf("CUPS failed to call cupsGetDests2(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, errors.New(text)
	}
	defer C.cupsFreeDests(c_num_dests, c_dests)

	num_dests := int(c_num_dests)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(c_dests)),
		Len:  num_dests,
		Cap:  num_dests,
	}
	dests := *(*[]C.cups_dest_t)(unsafe.Pointer(&hdr))

	names := make([]string, 0, num_dests)
	for _, dest := range dests {
		names = append(names, C.GoString(dest.name))
	}

	return names, nil
}

func (pc *PPDCache) servePPDs() {
	// Prime the cache.
	printerNames, err := pc.getPrinterNames()
	if err != nil {
		log.Fatal(err)
	}
	for _, printerName := range printerNames {
		pce, err := createPPDCacheEntry(printerName)
		if err != nil {
			log.Fatal(err)
		}
		pce.refreshPPDCacheEntry(pc.httpConnection)
		pc.m[printerName] = pce
	}

	for {
		select {
		case r := <-pc.request:
			pce, exists := pc.m[r.printerName]
			if !exists {
				pce, err = createPPDCacheEntry(r.printerName)
				if err != nil {
					r.response <- ppdResponse{"", "", err}
				}
			}
			if err := pce.refreshPPDCacheEntry(pc.httpConnection); err != nil {
				r.response <- ppdResponse{"", "", err}
			}
			r.response <- ppdResponse{C.GoString(pce.buffer), pce.hash, nil}

		case <-pc.q:
			pc.q <- true
			return
		}
	}
}

// Holds persistent data needed for calling C.cupsGetPPD3.
type ppdCacheEntry struct {
	name    *C.char
	modtime C.time_t
	buffer  *C.char
	bufsize C.size_t
	hash    string
}

// Creates an instance of PPDCache with the name field set, all else empty.
// Don't forget to call C.free() for the name and buffer fields with
// ppdCacheEntry.free()!
func createPPDCacheEntry(name string) (*ppdCacheEntry, error) {
	c_name := C.CString(name)
	modtime := C.time_t(0)
	bufsize := C.size_t(200)
	buffer := (*C.char)(C.malloc(bufsize))
	if buffer == nil {
		C.free(unsafe.Pointer(c_name))
		return nil, errors.New("Failed to malloc; out of memory?")
	}
	C.memset(unsafe.Pointer(buffer), 0, bufsize)

	pce := &ppdCacheEntry{c_name, modtime, buffer, bufsize, ""}

	return pce, nil
}

func (pce *ppdCacheEntry) free() {
	C.free(unsafe.Pointer(pce.name))
	os.Remove(C.GoString(pce.buffer))
	C.free(unsafe.Pointer(pce.buffer))
}

// Calls cupsGetPPD3().
func (pce *ppdCacheEntry) refreshPPDCacheEntry(httpConnection *C.http_t) error {
	c_http_status := C.cupsGetPPD3(httpConnection, pce.name, &pce.modtime, pce.buffer, pce.bufsize)

	switch c_http_status {
	case C.HTTP_STATUS_NOT_MODIFIED:
		return nil

	case C.HTTP_STATUS_OK:
		ppd, err := os.Open(C.GoString(pce.buffer))
		if err != nil {
			return err
		}
		defer ppd.Close()

		hash := md5.New()

		if _, err := io.Copy(hash, ppd); err != nil {
			return err
		}
		pce.hash = fmt.Sprintf("%x", hash.Sum(nil))

		return nil

	default:
		return errors.New(fmt.Sprintf("Failed to get PPD (%s)", int(c_http_status)))
	}
}
