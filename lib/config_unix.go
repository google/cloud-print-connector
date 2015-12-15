// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin

package lib

const defaultConfigFilename = "gcp-cups-connector.config.json"

// DefaultConfig represents reasonable default values for Config fields.
// Omitted Config fields are omitted on purpose; they are unique per
// connector instance.
var DefaultConfig = Config{
	XMPPServer:                "talk.google.com",
	XMPPPort:                  443,
	XMPPPingTimeout:           "5s",
	XMPPPingInterval:          "2m",
	GCPBaseURL:                "https://www.google.com/cloudprint/",
	GCPOAuthClientID:          "539833558011-35iq8btpgas80nrs3o7mv99hm95d4dv6.apps.googleusercontent.com",
	GCPOAuthClientSecret:      "V9BfPOvdiYuw12hDx5Y5nR0a",
	GCPOAuthAuthURL:           "https://accounts.google.com/o/oauth2/auth",
	GCPOAuthTokenURL:          "https://accounts.google.com/o/oauth2/token",
	GCPMaxConcurrentDownloads: 5,

	NativeJobQueueSize:        3,
	NativePrinterPollInterval: "1m",
	PrefixJobIDToJobTitle:     false,
	DisplayNamePrefix:         "",
	SNMPEnable:                false,
	SNMPCommunity:             "public",
	SNMPMaxConnections:        100,
	PrinterBlacklist:          []string{},
	LocalPrintingEnable:       true,
	CloudPrintingEnable:       false,
	LogLevel:                  "INFO",

	LogFileName:         "/tmp/cups-connector",
	LogFileMaxMegabytes: 1,
	LogMaxFiles:         3,
	LogToJournal:        false,

	MonitorSocketFilename: "/tmp/cups-connector-monitor.sock",

	CUPSMaxConnections: 50,
	CUPSConnectTimeout: "5s",
	CUPSPrinterAttributes: []string{
		"cups-version",
		"device-uri",
		"document-format-supported",
		"print-color-mode-default",
		"print-color-mode-supported",
		"printer-name",
		"printer-info",
		"printer-location",
		"printer-make-and-model",
		"printer-state",
		"printer-state-reasons",
		"printer-uuid",
		"marker-names",
		"marker-types",
		"marker-levels",
		"copies-default",
		"copies-supported",
		"number-up-default",
		"number-up-supported",
		"orientation-requested-default",
		"orientation-requested-supported",
		"pdf-versions-supported",
	},
	CUPSJobFullUsername:              false,
	CUPSIgnoreRawPrinters:            true,
	CUPSIgnoreClassPrinters:          true,
	CUPSCopyPrinterInfoToDisplayName: true,
}
