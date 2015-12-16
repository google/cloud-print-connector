// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin

package lib

const defaultConfigFilename = "gcp-cups-connector.config.json"

type Config struct {
	commonConfig

	// CUPS only: Where to place log file.
	LogFileName string `json:"log_file_name"`

	// CUPS only: Maximum log file size.
	LogFileMaxMegabytes uint `json:"log_file_max_megabytes"`

	// CUPS only: Maximum log file quantity.
	LogMaxFiles uint `json:"log_max_files"`

	// CUPS only: Log to the systemd journal instead of to files?
	LogToJournal bool `json:"log_to_journal"`

	// CUPS only: Filename of unix socket for connector-check to talk to connector.
	MonitorSocketFilename string `json:"monitor_socket_filename"`

	// CUPS only: Maximum quantity of open CUPS connections.
	CUPSMaxConnections uint `json:"cups_max_connections,omitempty"`

	// CUPS only: timeout for opening a new connection.
	CUPSConnectTimeout string `json:"cups_connect_timeout,omitempty"`

	// CUPS only: printer attributes to copy to GCP.
	CUPSPrinterAttributes []string `json:"cups_printer_attributes,omitempty"`

	// CUPS only: use the full username (joe@example.com) in CUPS job.
	CUPSJobFullUsername bool `json:"cups_job_full_username,omitempty"`

	// CUPS only: ignore printers with make/model 'Local Raw Printer'.
	CUPSIgnoreRawPrinters bool `json:"cups_ignore_raw_printers"`

	// CUPS only: ignore printers with make/model 'Local Printer Class'.
	CUPSIgnoreClassPrinters bool `json:"cups_ignore_class_printers"`

	// CUPS only: copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName.
	// TODO: rename with cups_ prefix
	CUPSCopyPrinterInfoToDisplayName bool `json:"copy_printer_info_to_display_name,omitempty"`
}

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
