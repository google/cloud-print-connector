// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux freebsd

package privet

// #cgo linux LDFLAGS: -lavahi-client -lavahi-common
// #cgo freebsd CFLAGS: -I/usr/local/include
// #cgo freebsd LDFLAGS: -L/usr/local/lib -lavahi-client -lavahi-common
// #include "avahi.h"
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/google/cloud-print-connector/log"
)

var (
	txtversKey     = C.CString("txtvers")
	txtversValue   = C.CString("1")
	tyKey          = C.CString("ty")
	noteKey        = C.CString("note")
	urlKey         = C.CString("url")
	typeKey        = C.CString("type")
	typeValue      = C.CString("printer")
	idKey          = C.CString("id")
	csKey          = C.CString("cs")
	csValueOnline  = C.CString("online")
	csValueOffline = C.CString("offline")
)

type record struct {
	// printerName is the name of the printer, which must live on the heap so that the
	// C event handler can see it.
	// serviceName is the name of the service, which is the same as name except
	// when there is a collision.
	printerName *C.char
	serviceName *C.char
	port        uint16
	ty          string
	note        string
	url         string
	id          string
	online      bool
	group       *C.AvahiEntryGroup
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

	if errstr := C.startAvahiClient(&z.threadedPoll, &z.client); errstr != nil {
		err := fmt.Errorf("Failed to start Avahi client: %s", C.GoString(errstr))
		return nil, err
	}

	go z.restartAndQuit()

	return &z, nil
}

func prepareTXT(ty, note, url, id string, online bool) *C.AvahiStringList {
	var txt *C.AvahiStringList
	txt = C.avahi_string_list_add_pair(txt, txtversKey, txtversValue)
	txt = C.avahi_string_list_add_pair(txt, typeKey, typeValue)

	tyValue := C.CString(ty)
	defer C.free(unsafe.Pointer(tyValue))
	txt = C.avahi_string_list_add_pair(txt, tyKey, tyValue)

	if note != "" {
		noteValue := C.CString(note)
		defer C.free(unsafe.Pointer(noteValue))
		txt = C.avahi_string_list_add_pair(txt, noteKey, noteValue)
	}

	urlValue := C.CString(url)
	defer C.free(unsafe.Pointer(urlValue))
	txt = C.avahi_string_list_add_pair(txt, urlKey, urlValue)

	idValue := C.CString(id)
	defer C.free(unsafe.Pointer(idValue))
	txt = C.avahi_string_list_add_pair(txt, idKey, idValue)

	if online {
		txt = C.avahi_string_list_add_pair(txt, csKey, csValueOnline)
	} else {
		txt = C.avahi_string_list_add_pair(txt, csKey, csValueOffline)
	}

	return txt
}

func (z *zeroconf) addPrinter(name string, port uint16, ty, note, url, id string, online bool) error {
	nameValue := C.CString(name)
	r := record{
		printerName: nameValue,
		serviceName: C.avahi_strdup(nameValue),
		port:        port,
		ty:          ty,
		note:        note,
		url:         url,
		id:          id,
		online:      online,
	}

	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	if _, exists := z.printers[name]; exists {
		C.free(unsafe.Pointer(r.printerName))
		C.avahi_free(unsafe.Pointer(r.serviceName))
		return fmt.Errorf("printer %s was already added to Avahi publishing", name)
	}
	if z.state == C.AVAHI_CLIENT_S_RUNNING {
		txt := prepareTXT(ty, note, url, id, online)
		defer C.avahi_string_list_free(txt)

		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		if errstr := C.addAvahiGroup(z.client, &r.group, r.printerName, r.serviceName, C.ushort(r.port), txt); errstr != nil {
			C.free(unsafe.Pointer(r.printerName))
			C.avahi_free(unsafe.Pointer(r.serviceName))
			err := fmt.Errorf("Failed to add Avahi group: %s", C.GoString(errstr))
			return err
		}
	}

	z.printers[name] = r
	return nil
}

func (z *zeroconf) updatePrinterTXT(name, ty, note, url, id string, online bool) error {
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	r, exists := z.printers[name]
	if !exists {
		return fmt.Errorf("printer %s cannot be updated for Avahi publishing; it was never added", name)
	}

	r.ty = ty
	r.url = url
	r.id = id
	r.online = online

	if z.state == C.AVAHI_CLIENT_S_RUNNING && r.group != nil {
		txt := prepareTXT(ty, note, url, id, online)
		defer C.avahi_string_list_free(txt)

		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		if errstr := C.updateAvahiGroup(r.group, r.serviceName, txt); errstr != nil {
			err := fmt.Errorf("Failed to update Avahi group: %s", C.GoString(errstr))
			return err
		}
	}

	z.printers[name] = r
	return nil
}

func (z *zeroconf) removePrinter(name string) error {
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	r, exists := z.printers[name]
	if !exists {
		return fmt.Errorf("printer %s cannot be updated for Avahi publishing; it was never added", name)
	}

	if z.state == C.AVAHI_CLIENT_S_RUNNING && r.group != nil {
		C.avahi_threaded_poll_lock(z.threadedPoll)
		defer C.avahi_threaded_poll_unlock(z.threadedPoll)

		if errstr := C.removeAvahiGroup(r.group); errstr != nil {
			err := fmt.Errorf("Failed to remove Avahi group: %s", C.GoString(errstr))
			return err
		}
	}

	C.free(unsafe.Pointer(r.printerName))
	C.avahi_free(unsafe.Pointer(r.serviceName))

	delete(z.printers, name)
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
			log.Warning("Avahi client failed. Make sure that avahi-daemon is running while I restart the client.")

			C.stopAvahiClient(z.threadedPoll, z.client)

			if errstr := C.startAvahiClient(&z.threadedPoll, &z.client); errstr != nil {
				err := errors.New(C.GoString(errstr))
				log.Errorf("Failed to restart Avahi client: %s", err)
			}

		case <-z.q:
			for name := range z.printers {
				z.removePrinter(name)
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
func handleClientStateChange(client *C.AvahiClient, newState C.AvahiClientState, userdata unsafe.Pointer) {
	z := instance
	z.spMutex.Lock()
	defer z.spMutex.Unlock()

	// Name conflict.
	if newState == C.AVAHI_CLIENT_S_COLLISION {
		log.Warning("Avahi reports a host name collision.")
	}

	// Transition from not connecting to connecting. Warn in logs.
	if newState == C.AVAHI_CLIENT_CONNECTING {
		log.Warning("Cannot find Avahi daemon. Is it running?")
	}

	// Transition from running to not running. Free all groups.
	if newState != C.AVAHI_CLIENT_S_RUNNING {
		log.Info("Local printing disabled (Avahi client is not running).")
		for name, r := range z.printers {
			if r.group != nil {
				if errstr := C.removeAvahiGroup(r.group); errstr != nil {
					err := errors.New(C.GoString(errstr))
					log.Errorf("Failed to remove Avahi group: %s", err)
				}
				r.group = nil
				z.printers[name] = r
			}
		}
	}

	// Transition from not running to running. Recreate all groups.
	if newState == C.AVAHI_CLIENT_S_RUNNING {
		log.Info("Local printing enabled (Avahi client is running).")
		for name, r := range z.printers {
			txt := prepareTXT(r.ty, r.note, r.url, r.id, r.online)
			defer C.avahi_string_list_free(txt)

			if errstr := C.addAvahiGroup(z.client, &r.group, r.printerName, r.serviceName, C.ushort(r.port), txt); errstr != nil {
				err := errors.New(C.GoString(errstr))
				log.Errorf("Failed to add Avahi group: %s", err)
			}

			z.printers[name] = r
		}
	}

	// Transition from not failure to failure. Recreate thread poll and client.
	if newState == C.AVAHI_CLIENT_FAILURE {
		z.restart <- struct{}{}
	}

	z.state = newState
}

//export handleGroupStateChange
func handleGroupStateChange(group *C.AvahiEntryGroup, state C.AvahiEntryGroupState, name unsafe.Pointer) {
	switch state {
	case C.AVAHI_ENTRY_GROUP_COLLISION:
		z := instance
		z.spMutex.Lock()
		defer z.spMutex.Unlock()

		// Pick a new name.
		printerName := C.GoString((*C.char)(name))
		r := z.printers[printerName]
		txt := prepareTXT(r.ty, r.note, r.url, r.id, r.online)
		defer C.avahi_string_list_free(txt)
		altName := C.avahi_alternative_service_name(r.serviceName)
		C.avahi_free(unsafe.Pointer(r.serviceName))
		r.serviceName = altName
		log.Warningf("Avahi failed to register '%s' due to a naming collision, trying with '%s'", printerName, C.GoString((*C.char)(altName)))
		if errstr := C.resetAvahiGroup(z.client, r.group, r.serviceName, C.ushort(r.port), txt); errstr != nil {
			r.group = nil
			err := errors.New(C.GoString(errstr))
			log.Errorf("Failed to reset Avahi group: %s", err)
		}
		z.printers[printerName] = r

	case C.AVAHI_ENTRY_GROUP_FAILURE:
		log.Warningf("Avahi failed to register '%s', don't know why", C.GoString((*C.char)(name)))
	}
}
