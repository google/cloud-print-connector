/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package cups

/*
#cgo LDFLAGS: -lcups
#include <cups/cups.h>
#include <stddef.h> // size_t
#include <stdlib.h> // malloc, free
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// CreateTempFile calls cupsTempFd() to create a new file that (1) lives in a
// "temporary" location (like /tmp) and (2) is readable by CUPS. The caller
// is responsible for deleting the file.
func CreateTempFile() (*os.File, error) {
	length := C.size_t(syscall.PathMax)
	filename := (*C.char)(C.malloc(length))
	if filename == nil {
		return nil, errors.New("Failed to malloc(); out of memory?")
	}
	defer C.free(unsafe.Pointer(filename))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fd := C.cupsTempFd(filename, C.int(length))
	if fd == C.int(-1) {
		err := fmt.Errorf("Failed to call cupsTempFd(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, err
	}

	return os.NewFile(uintptr(fd), C.GoString(filename)), nil
}

// uname returns strings similar to the Unix uname command:
// sysname, nodename, release, version, machine, domainname
func uname() (string, string, string, string, string, string, error) {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return "", "", "", "", "", "", err
	}
	return charsToString(u.Sysname), charsToString(u.Nodename),
		charsToString(u.Release), charsToString(u.Version), charsToString(u.Machine),
		charsToString(u.Domainname), nil
}

func charsToString(chars [65]int8) string {
	s := make([]byte, len(chars))
	var lens int
	for ; lens < len(chars) && chars[lens] != 0; lens++ {
		s[lens] = byte(chars[lens])
	}
	return string(s[:lens])
}
