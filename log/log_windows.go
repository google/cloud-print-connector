// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

// The log package logs to the Windows Event Log, or stdout.
package log

import (
	"fmt"

	"github.com/google/cloud-print-connector/lib"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const (
	logJobFormat     = "[Job %s] %s"
	logPrinterFormat = "[Printer %s] %s"

	dateTimeFormat = "2006-Jan-02 15:04:05"
)

var logger struct {
	level LogLevel
	elog  debug.Log
}

func init() {
	logger.level = INFO
}

// SetLevel sets the minimum severity level to log. Default is INFO.
func SetLevel(l LogLevel) {
	logger.level = l
}

func Start(logToConsole bool) error {
	if logToConsole {
		logger.elog = debug.New(lib.ConnectorName)

	} else {
		l, err := eventlog.Open(lib.ConnectorName)
		if err != nil {
			return err
		}
		logger.elog = l
	}

	return nil
}

func Stop() {
	err := logger.elog.Close()
	if err != nil {
		panic("Failed to close log")
	}
}

func log(level LogLevel, printerID, jobID, format string, args ...interface{}) {
	if logger.elog == nil {
		panic("Attempted to log without first calling Start()")
	}

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

	if level == DEBUG || level == FATAL {
		// Windows Event Log only has three levels; these two extra information prepended.
		message = fmt.Sprintf("%s %s", stringByLevel[level], message)
	}

	switch level {
	case FATAL, ERROR:
		logger.elog.Error(1, message)
	case WARNING:
		logger.elog.Warning(2, message)
	case INFO, DEBUG:
		logger.elog.Info(3, message)
	}
}
