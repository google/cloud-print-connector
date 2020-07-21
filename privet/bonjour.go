// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build darwin

package privet

// #cgo LDFLAGS: -framework CoreServices
// #include "bonjour.h"
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/google/cloud-print-connector/log"
)

// TODO: How to add the _printer subtype?
const serviceType = "_privet._tcp"

type zeroconf struct {
	printers map[string]C.CFNetServiceRef
	pMutex   sync.RWMutex // Protects printers.
	q        chan struct{}
}

// NewZeroconf manages Bonjour services for printers shared via Privet.
func newZeroconf() (*zeroconf, error) {
	z := zeroconf{
		printers: make(map[string]C.CFNetServiceRef),
		q:        make(chan struct{}),
	}
	return &z, nil
}

func (z *zeroconf) addPrinter(name string, port uint16, ty, note, url, id string, online bool) error {
	z.pMutex.RLock()
	if _, exists := z.printers[name]; exists {
		z.pMutex.RUnlock()
		return fmt.Errorf("Bonjour already has printer %s", name)
	}
	z.pMutex.RUnlock()

	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))
	serviceTypeC := C.CString(serviceType)
	defer C.free(unsafe.Pointer(serviceTypeC))
	tyC := C.CString(ty)
	defer C.free(unsafe.Pointer(tyC))
	noteC := C.CString(note)
	defer C.free(unsafe.Pointer(noteC))
	urlC := C.CString(url)
	defer C.free(unsafe.Pointer(urlC))
	idC := C.CString(id)
	defer C.free(unsafe.Pointer(idC))
	var onlineC *C.char
	if online {
		onlineC = C.CString("online")
	} else {
		onlineC = C.CString("offline")
	}
	defer C.free(unsafe.Pointer(onlineC))

	var errstr *C.char = nil
	service := C.startBonjour(nameC, serviceTypeC, C.ushort(port), tyC, noteC, urlC, idC, onlineC, &errstr)
	if errstr != nil {
		defer C.free(unsafe.Pointer(errstr))
		return errors.New(C.GoString(errstr))
	}

	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	z.printers[name] = service
	return nil
}

// updatePrinterTXT updates the advertised TXT record.
func (z *zeroconf) updatePrinterTXT(name, ty, note, url, id string, online bool) error {
	tyC := C.CString(ty)
	defer C.free(unsafe.Pointer(tyC))
	noteC := C.CString(note)
	defer C.free(unsafe.Pointer(noteC))
	urlC := C.CString(url)
	defer C.free(unsafe.Pointer(urlC))
	idC := C.CString(id)
	defer C.free(unsafe.Pointer(idC))
	var onlineC *C.char
	if online {
		onlineC = C.CString("online")
	} else {
		onlineC = C.CString("offline")
	}
	defer C.free(unsafe.Pointer(onlineC))

	z.pMutex.RLock()
	defer z.pMutex.RUnlock()

	if service, exists := z.printers[name]; exists {
		C.updateBonjour(service, tyC, noteC, urlC, idC, onlineC)
	} else {
		return fmt.Errorf("Bonjour can't update printer %s that hasn't been added", name)
	}
	return nil
}

func (z *zeroconf) removePrinter(name string) error {
	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	if service, exists := z.printers[name]; exists {
		C.stopBonjour(service)
		delete(z.printers, name)
	} else {
		return fmt.Errorf("Bonjour can't remove printer %s that hasn't been added", name)
	}
	return nil
}

func (z *zeroconf) quit() {
	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	for name, service := range z.printers {
		C.stopBonjour(service)
		delete(z.printers, name)
	}
}

//export logBonjourError
func logBonjourError(err *C.char) {
	log.Warningf("Bonjour: %s", C.GoString(err))
}
