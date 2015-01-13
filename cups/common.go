/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

// reconnect calls httpReconnect() via cgo, which re-opens the connection to
// the CUPS server, if needed.
func reconnect(http *C.http_t) error {
	// Lock the OS thread so that thread-local storage is available to
	// cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ippStatus := C.httpReconnect(http)
	if ippStatus != C.IPP_STATUS_OK {
		return fmt.Errorf("Failed to call cupsReconnect(): %d %s",
			int(ippStatus), C.GoString(C.cupsLastErrorString()))
	}
	return nil
}
