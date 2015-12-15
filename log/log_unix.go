// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin

// The log package logs to an io.Writer using the same log format that CUPS uses.
package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/coreos/go-systemd/journal"
)

const (
	logFormat        = "%c [%s] %s\n"
	logJobFormat     = "%c [%s] [Job %s] %s\n"
	logPrinterFormat = "%c [%s] [Printer %s] %s\n"

	dateTimeFormat = "02/Jan/2006:15:04:05 -0700"

	journalJobFormat     = "[Job %s] %s"
	journalPrinterFormat = "[Printer %s] %s"
)

var (
	levelToInitial = map[LogLevel]rune{
		FATAL:   'X', // "EMERG" in CUPS.
		ERROR:   'E',
		WARNING: 'W',
		INFO:    'I',
		DEBUG:   'D',
	}

	logger struct {
		writer         io.Writer
		level          LogLevel
		journalEnabled bool
	}
)

func (l LogLevel) priority() journal.Priority {
	switch l {
	case FATAL:
		return journal.PriCrit
	case ERROR:
		return journal.PriErr
	case WARNING:
		return journal.PriWarning
	case INFO:
		return journal.PriInfo
	case DEBUG:
		return journal.PriDebug
	default:
		return journal.PriDebug
	}
}

func init() {
	logger.writer = os.Stderr
	logger.level = INFO
}

// SetWriter sets the io.Writer to log to. Default is os.Stderr.
func SetWriter(w io.Writer) {
	logger.writer = w
}

// SetLevel sets the minimum severity level to log. Default is INFO.
func SetLevel(l LogLevel) {
	logger.level = l
}

// SetJournalEnabled enables or disables writing to the systemd journal. Default is false.
func SetJournalEnabled(b bool) {
	logger.journalEnabled = b
}

func log(level LogLevel, printerID, jobID, format string, args ...interface{}) {
	if level > logger.level {
		return
	}

	levelInitial := levelToInitial[level]
	dateTime := time.Now().Format(dateTimeFormat)
	var message string
	if format == "" {
		message = fmt.Sprint(args...)
	} else {
		message = fmt.Sprintf(format, args...)
	}

	journalVars := make(map[string]string)
	var journalMessage string
	if printerID != "" {
		fmt.Fprintf(logger.writer, logPrinterFormat, levelInitial, dateTime, printerID, message)
		journalVars["PRINTER_ID"] = printerID
		journalMessage = fmt.Sprintf(journalPrinterFormat, printerID, message)
	} else if jobID != "" {
		fmt.Fprintf(logger.writer, logJobFormat, levelInitial, dateTime, jobID, message)
		journalVars["JOB_ID"] = jobID
		journalMessage = fmt.Sprintf(journalJobFormat, jobID, message)
	} else {
		fmt.Fprintf(logger.writer, logFormat, levelInitial, dateTime, message)
		journalMessage = message
	}

	if logger.journalEnabled {
		pc := make([]uintptr, 1)
		runtime.Callers(3, pc)
		f := runtime.FuncForPC(pc[0])
		journalVars["CODE_FUNC"] = f.Name()
		file, line := f.FileLine(pc[0])
		journalVars["CODE_FILE"] = file
		journalVars["CODE_LINE"] = strconv.Itoa(line)
		journal.Send(journalMessage, level.priority(), journalVars)
	}
}
