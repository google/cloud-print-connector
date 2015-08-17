/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package cups

/*
#include "cups.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// filePathMaxLength varies by operating system and file system.
// This value should be large enough to be useful and small enough
// to work on any platform.
const filePathMaxLength = 1024

// createTempFileLock protects C.cupsTempFd() from it's non-threadsafe-self.
var createTempFileLock sync.Mutex

// CreateTempFile calls cupsTempFd() to create a new file that (1) lives in a
// "temporary" location (like /tmp) and (2) is readable by CUPS. The caller
// is responsible for deleting the file.
func CreateTempFile() (*os.File, error) {
	length := C.size_t(filePathMaxLength)
	filename := (*C.char)(C.malloc(length))
	if filename == nil {
		return nil, errors.New("Failed to malloc(); out of memory?")
	}
	defer C.free(unsafe.Pointer(filename))

	createTempFileLock.Lock()
	defer createTempFileLock.Unlock()

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
// sysname, nodename, release, version, machine
func uname() (string, string, string, string, string, error) {
	var name C.struct_utsname
	_, err := C.uname(&name)
	if err != nil {
		var errno syscall.Errno = err.(syscall.Errno)
		return "", "", "", "", "", fmt.Errorf("Failed to call uname: %s", errno)
	}

	return C.GoString(&name.sysname[0]), C.GoString(&name.nodename[0]),
		C.GoString(&name.release[0]), C.GoString(&name.version[0]),
		C.GoString(&name.machine[0]), nil
}
