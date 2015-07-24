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

	"github.com/golang/glog"
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

func (z *zeroconf) addPrinter(gcpID, name string, port uint16, ty, url, id string, online bool) error {
	z.pMutex.RLock()
	if _, exists := z.printers[gcpID]; exists {
		z.pMutex.RUnlock()
		return fmt.Errorf("Bonjour already has printer %s", gcpID)
	}
	z.pMutex.RUnlock()

	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	t := C.CString(serviceType)
	defer C.free(unsafe.Pointer(t))
	y := C.CString(ty)
	defer C.free(unsafe.Pointer(y))
	u := C.CString(url)
	defer C.free(unsafe.Pointer(u))
	i := C.CString(id)
	defer C.free(unsafe.Pointer(i))
	var o *C.char
	if online {
		o = C.CString("online")
	} else {
		o = C.CString("offline")
	}
	defer C.free(unsafe.Pointer(o))

	var errstr *C.char = nil
	service := C.startBonjour(n, t, C.ushort(port), y, u, i, o, &errstr)
	if errstr != nil {
		defer C.free(unsafe.Pointer(errstr))
		return errors.New(C.GoString(errstr))
	}

	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	z.printers[gcpID] = service
	return nil
}

// updatePrinterTXT updates the advertised TXT record.
func (z *zeroconf) updatePrinterTXT(gcpID, ty, url, id string, online bool) error {
	y := C.CString(ty)
	defer C.free(unsafe.Pointer(y))
	u := C.CString(url)
	defer C.free(unsafe.Pointer(u))
	i := C.CString(id)
	defer C.free(unsafe.Pointer(i))
	var o *C.char
	if online {
		o = C.CString("online")
	} else {
		o = C.CString("offline")
	}
	defer C.free(unsafe.Pointer(o))

	z.pMutex.RLock()
	defer z.pMutex.RUnlock()

	if service, exists := z.printers[gcpID]; exists {
		C.updateBonjour(service, y, u, i, o)
	} else {
		return fmt.Errorf("Bonjour can't update printer %s that hasn't been added", gcpID)
	}
	return nil
}

func (z *zeroconf) removePrinter(gcpID string) error {
	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	if service, exists := z.printers[gcpID]; exists {
		C.stopBonjour(service)
		delete(z.printers, gcpID)
	} else {
		return fmt.Errorf("Bonjour can't remove printer %s that hasn't been added", gcpID)
	}
	return nil
}

func (z *zeroconf) quit() {
	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	for gcpID, service := range z.printers {
		C.stopBonjour(service)
		delete(z.printers, gcpID)
	}
}

//export logBonjourError
func logBonjourError(err *C.char) {
	glog.Warningf("Bonjour: %s", C.GoString(err))
}
