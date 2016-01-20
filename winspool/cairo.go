/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package winspool

/*
#cgo pkg-config: cairo-win32
#include <cairo-win32.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func cairoStatusToError(status C.cairo_status_t) error {
	s := C.cairo_status_to_string(status)
	return fmt.Errorf("Cairo error: %s", C.GoString(s))
}

type CairoSurface uintptr

func (s CairoSurface) nativePointer() *C.struct__cairo_surface {
	return (*C.struct__cairo_surface)(unsafe.Pointer(s))
}

func CairoWin32PrintingSurfaceCreate(hDC HDC) (CairoSurface, error) {
	cHDC := (*C.struct_HDC__)(unsafe.Pointer(hDC))
	surface := C.cairo_win32_printing_surface_create(cHDC)
	s := CairoSurface(unsafe.Pointer(surface))
	if err := s.status(); err != nil {
		return 0, err
	}
	return s, nil
}

func (s CairoSurface) status() error {
	status := C.cairo_surface_status(s.nativePointer())
	if status != 0 {
		return cairoStatusToError(status)
	}
	return nil
}

func (s CairoSurface) ShowPage() error {
	C.cairo_surface_show_page(s.nativePointer())
	return s.status()
}

func (s CairoSurface) Finish() error {
	C.cairo_surface_finish(s.nativePointer())
	return s.status()
}

func (s *CairoSurface) Destroy() error {
	C.cairo_surface_destroy(s.nativePointer())
	if err := s.status(); err != nil {
		return err
	}

	*s = 0
	return nil
}

func (s *CairoSurface) GetDeviceOffset() (float64, float64, error) {
	var xOffset, yOffset float64
	C.cairo_surface_get_device_offset(s.nativePointer(), (*C.double)(&xOffset), (*C.double)(&yOffset))
	return xOffset, yOffset, s.status()
}

func (s *CairoSurface) GetDeviceScale() (float64, float64, error) {
	var xScale, yScale float64
	C.cairo_surface_get_device_scale(s.nativePointer(), (*C.double)(&xScale), (*C.double)(&yScale))
	return xScale, yScale, s.status()
}

func (s *CairoSurface) SetFallbackResolution(xPPI, yPPI float64) error {
	C.cairo_surface_set_fallback_resolution(s.nativePointer(), C.double(xPPI), C.double(yPPI))
	return s.status()
}

type CairoContext uintptr

func (c CairoContext) nativePointer() *C.struct__cairo {
	return (*C.struct__cairo)(unsafe.Pointer(c))
}

func CairoCreateContext(surface CairoSurface) (CairoContext, error) {
	context := C.cairo_create(surface.nativePointer())
	c := CairoContext(unsafe.Pointer(context))
	if err := c.status(); err != nil {
		return 0, err
	}
	return c, nil
}

func (c CairoContext) status() error {
	status := C.cairo_status(c.nativePointer())
	if status != 0 {
		return cairoStatusToError(status)
	}
	return nil
}

func (c *CairoContext) Destroy() error {
	C.cairo_destroy(c.nativePointer())

	*c = 0
	return nil
}

func (c CairoContext) Save() error {
	C.cairo_save(c.nativePointer())
	return c.status()
}

func (c CairoContext) Restore() error {
	C.cairo_restore(c.nativePointer())
	return c.status()
}

func (c CairoContext) IdentityMatrix() error {
	C.cairo_identity_matrix(c.nativePointer())
	return c.status()
}

type CairoMatrix struct {
	xx float64
	yx float64
	xy float64
	yy float64
	x0 float64
	y0 float64
}

func (c CairoContext) GetMatrix() (*CairoMatrix, error) {
	var m CairoMatrix
	C.cairo_get_matrix(c.nativePointer(), (*C.struct__cairo_matrix)(unsafe.Pointer(&m)))
	return &m, c.status()
}

func (c CairoContext) Translate(x, y float64) error {
	C.cairo_translate(c.nativePointer(), C.double(x), C.double(y))
	return c.status()
}

func (c CairoContext) Scale(x, y float64) error {
	C.cairo_scale(c.nativePointer(), C.double(x), C.double(y))
	return c.status()
}

func (c CairoContext) Clip() error {
	C.cairo_clip(c.nativePointer())
	return c.status()
}

func (c CairoContext) Rectangle(x, y, width, height float64) error {
	C.cairo_rectangle(c.nativePointer(), C.double(x), C.double(y), C.double(width), C.double(height))
	return c.status()
}
