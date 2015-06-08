/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

// #cgo LDFLAGS: -framework CoreServices
// #include "bonjour.h"
import "C"
import (
	"errors"
	"unsafe"

	"github.com/golang/glog"
)

type bonjour struct {
	service C.CFNetServiceRef
}

// NewZeroconf starts a new Bonjour service for a printer shared via Privet.
func NewZeroconf(name, serviceType, domain string, port uint16, url, id string, online bool) (*bonjour, error) {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	t := C.CString(serviceType)
	defer C.free(unsafe.Pointer(t))
	d := C.CString(domain)
	defer C.free(unsafe.Pointer(d))
	u := C.CString(url)
	defer C.free(unsafe.Pointer(u))
	i := C.CString(id)
	defer C.free(unsafe.Pointer(i))
	var o *C.char
	defer C.free(unsafe.Pointer(o))
	if online {
		o = C.CString("online")
	} else {
		o = C.CString("offline")
	}

	var errstr *C.char = nil
	service := C.startBonjour(n, t, d, C.ushort(port), u, i, o, &errstr)
	if errstr != nil {
		defer C.free(unsafe.Pointer(errstr))
		return nil, errors.New(C.GoString(errstr))
	}

	return &bonjour{service}, nil
}

func (b *bonjour) Quit() {
	C.stopBonjour(b.service)
}

//export logBonjourError
func logBonjourError(err *C.char) {
	glog.Warningf("Bonjour: %s", C.GoString(err))
}
