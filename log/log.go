/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package log

import "strings"

// LogLevel represents a subset of the severity levels named by CUPS.
type LogLevel uint8

const (
	FATAL LogLevel = iota
	ERROR
	WARNING
	INFO
	DEBUG
)

var (
	stringByLevel = map[LogLevel]string{
		FATAL:   "FATAL",
		ERROR:   "ERROR",
		WARNING: "WARNING",
		INFO:    "INFO",
		DEBUG:   "DEBUG",
	}
	levelByString = map[string]LogLevel{
		"FATAL":   FATAL,
		"ERROR":   ERROR,
		"WARNING": WARNING,
		"INFO":    INFO,
		"DEBUG":   DEBUG,
	}
)

func LevelFromString(level string) (LogLevel, bool) {
	v, ok := levelByString[strings.ToUpper(level)]
	if !ok {
		return 0, false
	}
	return v, true
}

func Fatal(args ...interface{})                           { log(FATAL, "", "", "", args...) }
func Fatalf(format string, args ...interface{})           { log(FATAL, "", "", format, args...) }
func FatalJob(jobID string, args ...interface{})          { log(FATAL, "", jobID, "", args...) }
func FatalJobf(jobID, format string, args ...interface{}) { log(FATAL, "", jobID, format, args...) }
func FatalPrinter(printerID string, args ...interface{})  { log(FATAL, printerID, "", "", args...) }
func FatalPrinterf(printerID, format string, args ...interface{}) {
	log(FATAL, printerID, "", format, args...)
}

func Error(args ...interface{})                           { log(ERROR, "", "", "", args...) }
func Errorf(format string, args ...interface{})           { log(ERROR, "", "", format, args...) }
func ErrorJob(jobID string, args ...interface{})          { log(ERROR, "", jobID, "", args...) }
func ErrorJobf(jobID, format string, args ...interface{}) { log(ERROR, "", jobID, format, args...) }
func ErrorPrinter(printerID string, args ...interface{})  { log(ERROR, printerID, "", "", args...) }
func ErrorPrinterf(printerID, format string, args ...interface{}) {
	log(ERROR, printerID, "", format, args...)
}

func Warning(args ...interface{})                           { log(WARNING, "", "", "", args...) }
func Warningf(format string, args ...interface{})           { log(WARNING, "", "", format, args...) }
func WarningJob(jobID string, args ...interface{})          { log(WARNING, "", jobID, "", args...) }
func WarningJobf(jobID, format string, args ...interface{}) { log(WARNING, "", jobID, format, args...) }
func WarningPrinter(printerID string, args ...interface{})  { log(WARNING, printerID, "", "", args...) }
func WarningPrinterf(printerID, format string, args ...interface{}) {
	log(WARNING, printerID, "", format, args...)
}

func Info(args ...interface{})                           { log(INFO, "", "", "", args...) }
func Infof(format string, args ...interface{})           { log(INFO, "", "", format, args...) }
func InfoJob(jobID string, args ...interface{})          { log(INFO, "", jobID, "", args...) }
func InfoJobf(jobID, format string, args ...interface{}) { log(INFO, "", jobID, format, args...) }
func InfoPrinter(printerID string, args ...interface{})  { log(INFO, printerID, "", "", args...) }
func InfoPrinterf(printerID, format string, args ...interface{}) {
	log(INFO, printerID, "", format, args...)
}

func Debug(args ...interface{})                           { log(DEBUG, "", "", "", args...) }
func Debugf(format string, args ...interface{})           { log(DEBUG, "", "", format, args...) }
func DebugJob(jobID string, args ...interface{})          { log(DEBUG, "", jobID, "", args...) }
func DebugJobf(jobID, format string, args ...interface{}) { log(DEBUG, "", jobID, format, args...) }
func DebugPrinter(printerID string, args ...interface{})  { log(DEBUG, printerID, "", "", args...) }
func DebugPrinterf(printerID, format string, args ...interface{}) {
	log(DEBUG, printerID, "", format, args...)
}
