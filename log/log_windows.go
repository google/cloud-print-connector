// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

// The log package logs to the Windows Event Log, or stdout.
package log

import (
	"fmt"
	"time"
)

const (
	logJobFormat     = "[Job %s] %s"
	logPrinterFormat = "[Printer %s] %s"

	dateTimeFormat = "2006-Jan-02 15:04:05"
)

var logger struct {
	level         LogLevel
	logToStdout   bool
	logToEventLog bool
}

func init() {
	logger.level = INFO
	logger.logToStdout = false
	logger.logToEventLog = true
}

// SetLevel sets the minimum severity level to log. Default is INFO.
func SetLevel(l LogLevel) {
	logger.level = l
}

func SetLogToConsole(b bool) {
	logger.logToStdout = b
}

func SetLogToEventLog(b bool) {
	logger.logToEventLog = b
}

func log(level LogLevel, printerID, jobID, format string, args ...interface{}) {
	if level > logger.level {
		return
	}

	var message string
	if format == "" {
		message = fmt.Sprint(args...)
	} else {
		message = fmt.Sprintf(format, args...)
	}

	if printerID != "" {
		message = fmt.Sprintf(logPrinterFormat, printerID, message)
	} else if jobID != "" {
		message = fmt.Sprintf(logJobFormat, jobID, message)
	}

	if logger.logToStdout {
		dateTime := time.Now().Format(dateTimeFormat)
		levelValue := stringByLevel[level]
		fmt.Println(dateTime, levelValue, message)
	}

	if logger.logToEventLog {
		if level == DEBUG || level == FATAL {
			// Windows Event Log only has three levels; these two extra information prepended.
			message = fmt.Sprintf("%s %s", stringByLevel[level], message)
		}

		// TODO
	}
}
