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
