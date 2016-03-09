// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package winspool

import (
	"reflect"
	"syscall"
	"unsafe"
)

const utf16StringMaxBytes = 1024

func utf16PtrToStringSize(s *uint16, bytes uint32) string {
	if s == nil {
		return ""
	}

	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(s)),
		Len:  int(bytes / 2),
		Cap:  int(bytes / 2),
	}
	c := *(*[]uint16)(unsafe.Pointer(&hdr))

	return syscall.UTF16ToString(c)
}

func utf16PtrToString(s *uint16) string {
	return utf16PtrToStringSize(s, utf16StringMaxBytes)
}
