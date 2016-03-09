// +build linux darwin

/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package winspool

/*
#cgo pkg-config: poppler-glib

#include <glib/poppler.h>
#include <glib.h>

#include <stdlib.h> // free
*/
import "C"
import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"
)

func gErrorToGoError(gerr *C.GError) error {
	if gerr == nil {
		return errors.New("Poppler/GLib: unknown error")
	}

	defer C.g_error_free(gerr)

	message := C.GoString((*C.char)(gerr.message))
	if message == "No error" {
		// Work around inconsistent error message when named file doesn't exist.
		quarkString := C.GoString((*C.char)(C.g_quark_to_string(gerr.domain)))
		if "g-file-error-quark" == quarkString {
			return fmt.Errorf("Poppler/GLib: file error, code %d", gerr.code)
		}
		return fmt.Errorf("Poppler/GLib: unknown error, domain %d, code %d", gerr.domain, gerr.code)
	}

	return fmt.Errorf("Poppler/GLib: %s", C.GoString((*C.char)(gerr.message)))
}

type PopplerDocument uintptr

func (d PopplerDocument) nativePointer() *C.struct__PopplerDocument {
	return (*C.struct__PopplerDocument)(unsafe.Pointer(d))
}

func PopplerDocumentNewFromFile(filename string) (PopplerDocument, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return 0, err
	}

	cFilename := (*C.gchar)(C.CString(filename))
	defer C.free(unsafe.Pointer(cFilename))

	var gerr *C.GError
	uri := C.g_filename_to_uri(cFilename, nil, &gerr)
	if uri == nil || gerr != nil {
		return 0, gErrorToGoError(gerr)
	}
	defer C.g_free(C.gpointer(uri))

	doc := C.poppler_document_new_from_file((*C.char)(uri), nil, &gerr)
	if gerr != nil {
		return 0, gErrorToGoError(gerr)
	}

	return PopplerDocument(unsafe.Pointer(doc)), nil
}

func (d PopplerDocument) GetNPages() int {
	n := C.poppler_document_get_n_pages(d.nativePointer())
	return int(n)
}

func (d PopplerDocument) GetPage(index int) PopplerPage {
	p := C.poppler_document_get_page(d.nativePointer(), C.int(index))
	return PopplerPage(uintptr(unsafe.Pointer(p)))
}

func (d *PopplerDocument) Unref() {
	C.g_object_unref(C.gpointer(*d))
	*d = 0
}

type PopplerPage uintptr

func (p PopplerPage) nativePointer() *C.struct__PopplerPage {
	return (*C.struct__PopplerPage)(unsafe.Pointer(p))
}

// GetSize returns the width and height of the page, in points (1/72 inch).
func (p PopplerPage) GetSize() (float64, float64, error) {
	var width, height C.double
	C.poppler_page_get_size(p.nativePointer(), &width, &height)
	return float64(width), float64(height), nil
}

func (p PopplerPage) RenderForPrinting(context CairoContext) {
	C.poppler_page_render_for_printing(p.nativePointer(), context.nativePointer())
}

func (p *PopplerPage) Unref() {
	C.g_object_unref(C.gpointer(*p))
	*p = 0
}
