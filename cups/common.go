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
	c_len := C.size_t(syscall.PathMax)
	c_filename := (*C.char)(C.malloc(c_len))
	if c_filename == nil {
		return nil, errors.New("Failed to malloc(); out of memory?")
	}
	defer C.free(unsafe.Pointer(c_filename))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	c_fd := C.cupsTempFd(c_filename, C.int(c_len))
	if c_fd == C.int(-1) {
		err := fmt.Errorf("Failed to call cupsTempFd(): %d %s",
			int(C.cupsLastError()), C.GoString(C.cupsLastErrorString()))
		return nil, err
	}

	return os.NewFile(uintptr(c_fd), C.GoString(c_filename)), nil
}

// reconnect calls httpReconnect() via cgo, which re-opens the connection to
// the CUPS server, if needed.
func reconnect(c_http *C.http_t) error {
	// Lock the OS thread so that thread-local storage is available to
	// cupsLastErrorString().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	c_ippStatus := C.httpReconnect(c_http)
	if c_ippStatus != C.IPP_STATUS_OK {
		return fmt.Errorf("Failed to call cupsReconnect(): %d %s",
			int(c_ippStatus), C.GoString(C.cupsLastErrorString()))
	}
	return nil
}
