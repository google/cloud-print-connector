/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

/*
#include "dnssd.h"
*/
import "C"
import "unsafe"

type dnssdService struct {
	name     string
	hostname string
	port     uint16
}

// discoverPrinters discovers all Privet printers via DNS-SD.
func discoverPrinters() []dnssdService {
	var services []dnssdService

	for dp := C.discoverPrinters(); dp != nil; dp = dp.next {
		service := dnssdService{
			C.GoString(dp.service.name),
			C.GoString(dp.service.hostname),
			uint16(dp.service.port),
		}
		services = append(services, service)

		defer C.free(unsafe.Pointer(dp.service.name))
		defer C.free(unsafe.Pointer(dp.service.hostname))
		defer C.free(unsafe.Pointer(dp.service))
		defer C.free(unsafe.Pointer(dp))
	}

	return services
}

// resolvePrinter resolves a printer name so we know it's hostname:port.
// The second return value is false when the printer can't be resolved.
func resolvePrinter(name string) (dnssdService, bool) {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	rp := C.resolvePrinter(nameC)
	if rp == nil {
		return dnssdService{}, false
	}
	defer C.free(unsafe.Pointer(rp))

	if rp.hostname != nil {
		hostname := C.GoString(rp.hostname)
		C.free(unsafe.Pointer(rp.hostname))
		port := uint16(rp.port)
		return dnssdService{name, hostname, port}, true
	}

	return dnssdService{}, false
}
