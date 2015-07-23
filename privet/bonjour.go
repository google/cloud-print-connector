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
	"unsafe"

	"github.com/golang/glog"
)

type zeroconf struct {
	service C.CFNetServiceRef
}

// NewZeroconf starts a new Bonjour service for a printer shared via Privet.
// TODO: Change ty, url, id, online params to TXT map.
func newZeroconf(name, serviceType string, port uint16, ty, url, id string, online bool) (*zeroconf, error) {
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
		return nil, errors.New(C.GoString(errstr))
	}

	return &zeroconf{service}, nil
}

// UpdateTXT updates the advertised TXT record.
func (b *zeroconf) updateTXT(ty, url, id string, online bool) {
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

	C.updateBonjour(b.service, y, u, i, o)
}

func (b *zeroconf) quit() {
	C.stopBonjour(b.service)
}

//export logBonjourError
func logBonjourError(err *C.char) {
	glog.Warningf("Bonjour: %s", C.GoString(err))
}
