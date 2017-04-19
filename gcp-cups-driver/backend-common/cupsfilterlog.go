/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// The functions in cupsfilterlog write log message to STDERR, per CUPS filter(7) manpage:
// https://www.cups.org/documentation.php/man-filter.html
package common

import (
	"fmt"
	"os"
)

// logPrefix is a single word that appears at the first line of a log entry,
// as described in the filter(7) manpage.
type logPrefix string

const (
	PrefixAttr  logPrefix = "ATTR"
	PrefixPage  logPrefix = "PAGE"
	PrefixPPD   logPrefix = "PPD"
	PrefixState logPrefix = "STATE"
)

func Logf(prefix logPrefix, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: %s", prefix, fmt.Sprintf(format, args...))
}

// LogLevel represents the severity of a log message.
type LogLevel logPrefix

const (
	LevelEmerg   LogLevel = "EMERG"
	LevelAlert   LogLevel = "ALERT"
	LevelCrit    LogLevel = "CRIT"
	LevelError   LogLevel = "ERROR"
	LevelWarning LogLevel = "WARNING"
	LevelNotice  LogLevel = "NOTICE"
	LevelInfo    LogLevel = "INFO"
	LevelDebug   LogLevel = "DEBUG"
	LevelDebug2  LogLevel = "DEBUG2"
)

func Alert(format string, args ...interface{})   { Logf(logPrefix(LevelAlert), format, args...) }
func Crit(format string, args ...interface{})    { Logf(logPrefix(LevelCrit), format, args...) }
func Debug(format string, args ...interface{})   { Logf(logPrefix(LevelDebug), format, args...) }
func Debug2(format string, args ...interface{})  { Logf(logPrefix(LevelDebug2), format, args...) }
func Emerg(format string, args ...interface{})   { Logf(logPrefix(LevelEmerg), format, args...) }
func Error(format string, args ...interface{})   { Logf(logPrefix(LevelError), format, args...) }
func Info(format string, args ...interface{})    { Logf(logPrefix(LevelInfo), format, args...) }
func Notice(format string, args ...interface{})  { Logf(logPrefix(LevelNotice), format, args...) }
func Warning(format string, args ...interface{}) { Logf(logPrefix(LevelWarning), format, args...) }

// StateLevel represents the severity suffix described in RFC 2911 section 4.4.12.
type StateLevel logPrefix

const (
	StateReport  StateLevel = "report"
	StateWarning StateLevel = "warning"
	StateError   StateLevel = "error"
)

func AddStateReason(state StateLevel, reason string)    { Logf(PrefixState, "+%s-%s", reason, state) }
func RemoveStateReason(state StateLevel, reason string) { Logf(PrefixState, "-%s-%s", reason, state) }

func SetPPDKeywordValue(keyword string, value string) { Logf(PrefixPPD, "%s=%s", keyword, value) }
