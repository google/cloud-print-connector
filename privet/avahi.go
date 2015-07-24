// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build !darwin

package privet

// #cgo LDFLAGS: -lavahi-client -lavahi-common
// #include "avahi.h"
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/golang/glog"
)

type record struct {
	name   string
	port   uint16
	ty     string
	url    string
	id     string
	online bool
	group  *C.AvahiEntryGroup
}

type zeroconf struct {
	threadedPoll *C.AvahiThreadedPoll
	client       *C.AvahiClient

	state    C.AvahiClientState
	printers map[string]record
	spMutex  sync.Mutex // Protects state and printers.

	restart chan struct{}
	q       chan struct{}
}

// Keep the only instance in a global (package) var for C event handling.
var instance *zeroconf

func newZeroconf() (*zeroconf, error) {
	z := zeroconf{
		state:    C.AVAHI_CLIENT_CONNECTING,
		printers: make(map[string]record),
		restart:  make(chan struct{}),
		q:        make(chan struct{}),
	}
	instance = &z

	var errstr *C.char
	C.startAvahiClient(&z.threadedPoll, &z.client, &errstr)
	if errstr != nil {
		err := errors.New(C.GoString(errstr))
		C.free(unsafe.Pointer(errstr))
		return nil, err
	}

	go z.restartAndQuit()

	return &z, nil
}

func (z *zeroconf) addPrinter(gcpID, name string, port uint16, ty, url, id string, online bool) error {
	r := record{
		name:   name,
		port:   port,
		ty:     ty,
		url:    url,
		id:     id,
		online: online,
	}

	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	if _, exists := z.printers[gcpID]; exists {
		return fmt.Errorf("printer %s was already added to Avahi publishing", gcpID)
	}
	if z.state == C.AVAHI_CLIENT_S_RUNNING {
		fmt.Println("avahi client running")
		n := C.CString(name)
		defer C.free(unsafe.Pointer(n))
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

		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		var errstr *C.char
		C.addAvahiGroup(z.threadedPoll, z.client, &r.group, n, C.ushort(port), y, u, i, o, &errstr)
		if errstr != nil {
			err := errors.New(C.GoString(errstr))
			C.free(unsafe.Pointer(errstr))
			return err
		}
	} else {
		fmt.Println("avahi client not running")
	}

	z.printers[gcpID] = r
	return nil
}

func (z *zeroconf) updatePrinterTXT(gcpID, ty, url, id string, online bool) error {
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	r, exists := z.printers[gcpID]
	if !exists {
		return fmt.Errorf("printer %s cannot be updated for Avahi publishing; it was never added", gcpID)
	}

	r.ty = ty
	r.url = url
	r.id = id
	r.online = online

	if z.state == C.AVAHI_CLIENT_S_RUNNING && r.group != nil {
		n := C.CString(r.name)
		defer C.free(unsafe.Pointer(n))
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

		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		var errstr *C.char
		C.updateAvahiGroup(z.threadedPoll, r.group, n, y, u, i, o, &errstr)
		if errstr != nil {
			err := errors.New(C.GoString(errstr))
			C.free(unsafe.Pointer(errstr))
			return err
		}
	}

	z.printers[gcpID] = r
	return nil
}

func (z *zeroconf) removePrinter(gcpID string) error {
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	r, exists := z.printers[gcpID]
	if !exists {
		return fmt.Errorf("printer %s cannot be updated for Avahi publishing; it was never added", gcpID)
	}

	if z.state == C.AVAHI_CLIENT_S_RUNNING && r.group != nil {
		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		var errstr *C.char
		C.removeAvahiGroup(z.threadedPoll, r.group, &errstr)
		if errstr != nil {
			err := errors.New(C.GoString(errstr))
			C.free(unsafe.Pointer(errstr))
			return err
		}
	}

	delete(z.printers, gcpID)
	return nil
}

func (z *zeroconf) quit() {
	z.q <- struct{}{}
	<-z.q
}

func (z *zeroconf) restartAndQuit() {
	for {
		select {
		case <-z.restart:
			glog.Warning("Avahi client failed. Make sure that avahi-daemon is running while I restart the client.")

			C.stopAvahiClient(z.threadedPoll, z.client)

			var errstr *C.char
			C.startAvahiClient(&z.threadedPoll, &z.client, &errstr)
			if errstr != nil {
				err := errors.New(C.GoString(errstr))
				C.free(unsafe.Pointer(errstr))
				glog.Errorf("Failed to restart Avahi client: %s", err)
			}

		case <-z.q:
			for gcpID := range z.printers {
				z.removePrinter(gcpID)
			}
			C.stopAvahiClient(z.threadedPoll, z.client)
			close(z.q)
			return
		}
	}
}

// handleClientStateChange makes clean transitions as the connection with
// avahi-daemon changes.
//export handleClientStateChange
func handleClientStateChange(newState C.AvahiClientState) {
	z := instance
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	// Transition from not connecting to connecting. Warn in logs.
	if z.state != C.AVAHI_CLIENT_CONNECTING && newState == C.AVAHI_CLIENT_CONNECTING {
		glog.Warning("Avahi client is looking for avahi-daemon. Is it running?")
	}

	// Transition from running to not running. Free all groups.
	if z.state == C.AVAHI_CLIENT_S_RUNNING && newState != C.AVAHI_CLIENT_S_RUNNING {
		glog.Info("Avahi client stopped running.")
		for gcpID, r := range z.printers {
			if r.group != nil {
				var errstr *C.char
				C.removeAvahiGroup(z.threadedPoll, r.group, &errstr)
				if errstr != nil {
					fmt.Println(C.GoString(errstr))
					C.free(unsafe.Pointer(errstr))
				}
				r.group = nil
				z.printers[gcpID] = r
			}
		}
	}

	// Transition from not running to running. Recreate all groups.
	if z.state != C.AVAHI_CLIENT_S_RUNNING && newState == C.AVAHI_CLIENT_S_RUNNING {
		glog.Info("Avahi client running.")
		for gcpID, r := range z.printers {
			n := C.CString(r.name)
			defer C.free(unsafe.Pointer(n))
			y := C.CString(r.ty)
			defer C.free(unsafe.Pointer(y))
			u := C.CString(r.url)
			defer C.free(unsafe.Pointer(u))
			i := C.CString(r.id)
			defer C.free(unsafe.Pointer(i))
			var o *C.char
			if r.online {
				o = C.CString("online")
			} else {
				o = C.CString("offline")
			}
			defer C.free(unsafe.Pointer(o))

			var errstr *C.char
			C.addAvahiGroup(z.threadedPoll, z.client, &r.group, n, C.ushort(r.port), y, u, i, o, &errstr)
			if errstr != nil {
				err := errors.New(C.GoString(errstr))
				C.free(unsafe.Pointer(errstr))
				glog.Error(err)
			}

			z.printers[gcpID] = r
		}
	}

	// Transition from not failure to failure. Recreate thread poll and client.
	if z.state != C.AVAHI_CLIENT_FAILURE && newState == C.AVAHI_CLIENT_FAILURE {
		z.restart <- struct{}{}
	}

	z.state = newState
}
