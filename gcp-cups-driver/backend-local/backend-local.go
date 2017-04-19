/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Google Cloud Print CUPS Virtual Driver, backend
//
// Implements a CUPS backend as described here:
// https://www.cups.org/documentation.php/man-backend.html
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/gcp-cups-driver/backend-common"
)

const (
	schema        = "gcp-local"
	backendFormat = `network gcp-local://%s "%s" "%s" "%s"`
	ieee1284PDF   = "MANUFACTURER:Google;MODEL:Cloud Print - PDF;SERIALNUMBER:%s"
	ieee1284PWG   = "MANUFACTURER:Google;MODEL:Cloud Print - PWG;SERIALNUMBER:%s"
)

type printer struct {
	name         string
	info         *privetInfo
	capabilities *cdd.CloudDeviceDescription
}

func main() {
	switch len(os.Args) {
	case 1:
		if !discover() {
			break
		}
		return

	case 6, 7:
		var f *os.File
		var err error
		if len(os.Args) == 7 {
			// Read file at filename. Assume the caller will delete the file.
			f, err = os.Open(os.Args[6])
			if err != nil {
				common.Error("Failed to open job payload file %s: %s", os.Args[6], err)
				break
			}
			defer f.Close()

		} else {
			f, ok := stdinToFile()
			if !ok {
				break
			}
			defer func() {
				f.Close()
				os.Remove(f.Name())
			}()
		}
		ok := printJob(os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], f)
		if ok {
			return
		}

	default:
		common.Info("Usage: %s job-id user title copies options [filename]", os.Args[0])
	}

	os.Exit(1)
}

// stdinToFile copies the contents of os.Stdin to a temporary file. This is necessary
// to get the size of the incoming job for the Content-Length HTTP request header.
// Returns the file object and true, or nil and false on failure.
func stdinToFile() (*os.File, bool) {
	tmpDir, ok := common.Env("TMPDIR")
	if !ok {
		return nil, false
	}

	f, err := ioutil.TempFile(tmpDir, "gcp-local-job")
	if err != nil {
		common.Error("Failed to create a temporary file to copy STDIN contents to: %s, err")
		return nil, false
	}
	contentLength, err := io.Copy(f, os.Stdin)
	if err != nil {
		common.Error("Failed to copy STDIN contents to %s", f.Name())
		f.Close()
		os.Remove(f.Name())
		return nil, false
	}
	if contentLength == 0 {
		common.Error("Cannot print empty payload")
		f.Close()
		os.Remove(f.Name())
		return nil, false
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		common.Error("Failed to seek to start of new temporary file %s: %s", f.Name(), err)
		f.Close()
		os.Remove(f.Name())
		return nil, false
	}

	return f, true
}

// discover finds all Privet printers via DNS-SD.
// Returns true on success, false on failure.
func discover() bool {
	var wg sync.WaitGroup
	fmt.Println(`network gcp-local "Unknown" "Google Cloud Print (local)"`)

	for d, i := discoverPrinters(), 0; i < len(d); i++ {
		wg.Add(1)
		go func(d *dnssdService) {
			defer wg.Done()

			pc := newPrivetClient(d.hostname, d.port)
			info := pc.info()
			if info == nil {
				return
			}

			capabilities, ok := pc.capabilities()
			if !ok {
				return
			}

			var serialNumber string
			if info.SerialNumber != "" {
				serialNumber = info.SerialNumber
			} else {
				serialNumber = d.name
			}

			var deviceID string
			if capabilities.supportsPDF() {
				deviceID = fmt.Sprintf(ieee1284PDF, serialNumber)
			} else {
				deviceID = fmt.Sprintf(ieee1284PWG, serialNumber)
			}

			fmt.Println(fmt.Sprintf(backendFormat, d.name, info.deviceMakeAndModel(), info.Name, deviceID))
		}(&d[i])
	}

	wg.Wait()
	return true
}

func printJob(id, user, title, copies, options string, f *os.File) bool {
	deviceURI, ok := common.Env("DEVICE_URI")
	if !ok {
		return false
	}

	contentType, ok := common.Env("CONTENT_TYPE")
	if !ok {
		return false
	}

	printerName := strings.TrimPrefix(deviceURI, "gcp-local://")
	service, ok := resolvePrinter(printerName)
	if !ok {
		return false
	}
	pc := newPrivetClient(service.hostname, service.port)

	info := pc.info()
	if info == nil {
		return false
	}

	// TODO: Options to CJT; call pc.createJob.

	jobID, ok := pc.submitDoc("", user, title, f, contentType)
	if !ok {
		return false
	}
	common.Info("Done printing %s", jobID)
	return true
}
