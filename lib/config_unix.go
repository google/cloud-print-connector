// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin

package lib

const defaultConfigFilename = "gcp-cups-connector.config.json"

type Config struct {
	// Associated with root account. XMPP credential.
	XMPPJID string `json:"xmpp_jid,omitempty"`

	// Associated with robot account. Used for acquiring OAuth access tokens.
	RobotRefreshToken string `json:"robot_refresh_token,omitempty"`

	// Associated with user account. Used for sharing GCP printers; may be omitted.
	UserRefreshToken string `json:"user_refresh_token,omitempty"`

	// Scope (user, group, domain) to share printers with.
	ShareScope string `json:"share_scope,omitempty"`

	// User-chosen name of this proxy. Should be unique per Google user account.
	ProxyName string `json:"proxy_name,omitempty"`

	// XMPP server FQDN.
	XMPPServer string `json:"xmpp_server,omitempty"`

	// XMPP server port number.
	XMPPPort uint16 `json:"xmpp_port,omitempty"`

	// XMPP ping timeout (give up waiting after this time).
	// TODO: Rename with "gcp_" removed.
	XMPPPingTimeout string `json:"gcp_xmpp_ping_timeout,omitempty"`

	// XMPP ping interval (time between ping attempts).
	// TODO: Rename with "gcp_" removed.
	// TODO: Rename with "_default" removed.
	XMPPPingInterval string `json:"gcp_xmpp_ping_interval_default,omitempty"`

	// GCP API URL prefix.
	GCPBaseURL string `json:"gcp_base_url,omitempty"`

	// OAuth2 client ID (not unique per client).
	GCPOAuthClientID string `json:"gcp_oauth_client_id,omitempty"`

	// OAuth2 client secret (not unique per client).
	GCPOAuthClientSecret string `json:"gcp_oauth_client_secret,omitempty"`

	// OAuth2 auth URL.
	GCPOAuthAuthURL string `json:"gcp_oauth_auth_url,omitempty"`

	// OAuth2 token URL.
	GCPOAuthTokenURL string `json:"gcp_oauth_token_url,omitempty"`

	// Maximum quantity of jobs (data) to download concurrently.
	GCPMaxConcurrentDownloads uint `json:"gcp_max_concurrent_downloads,omitempty"`

	// CUPS job queue size.
	// TODO: rename without cups_ prefix
	NativeJobQueueSize uint `json:"cups_job_queue_size"`

	// Interval (eg 10s, 1m) between CUPS printer state polls.
	// TODO: rename without cups_ prefix
	NativePrinterPollInterval string `json:"cups_printer_poll_interval"`

	// Add the job ID to the beginning of the job title. Useful for debugging.
	PrefixJobIDToJobTitle bool `json:"prefix_job_id_to_job_title"`

	// Prefix for all GCP printers hosted by this connector.
	DisplayNamePrefix string `json:"display_name_prefix"`

	// Enable SNMP to augment native printer information.
	SNMPEnable bool `json:"snmp_enable"`

	// Community string to use.
	SNMPCommunity string `json:"snmp_community"`

	// Maximum quantity of open SNMP connections.
	SNMPMaxConnections uint `json:"snmp_max_connections"`

	// Ignore printers with native names.
	PrinterBlacklist []string `json:"printer_blacklist"`

	// Enable local discovery and printing.
	LocalPrintingEnable bool `json:"local_printing_enable"`

	// Enable cloud discovery and printing.
	CloudPrintingEnable bool `json:"cloud_printing_enable"`

	// Least severity to log.
	LogLevel string `json:"log_level"`

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
