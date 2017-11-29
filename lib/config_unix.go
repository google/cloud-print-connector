// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin freebsd

package lib

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/urfave/cli"
	"launchpad.net/go-xdg/v0"
)

const (
	platformName = "CUPS"

	defaultConfigFilename = "gcp-cups-connector.config.json"
)

type Config struct {
	// Enable local discovery and printing.
	LocalPrintingEnable bool `json:"local_printing_enable"`

	// Enable cloud discovery and printing.
	CloudPrintingEnable bool `json:"cloud_printing_enable"`

	// Enable fcm notifications instead of xmpp notifications.
	FcmNotificationsEnable bool `json:"fcm_notifications_enable"`

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

	// FCM url client should listen on.
	FcmServerBindUrl string `json:"fcm_server_bind_url,omitempty"`

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

	// CUPS job queue size, must be greater than zero.
	// TODO: rename without cups_ prefix
	NativeJobQueueSize uint `json:"cups_job_queue_size,omitempty"`

	// Interval (eg 10s, 1m) between CUPS printer state polls.
	// TODO: rename without cups_ prefix
	NativePrinterPollInterval string `json:"cups_printer_poll_interval,omitempty"`

	// Use the full username (joe@example.com) in job.
	// TODO: rename without cups_ prefix
	CUPSJobFullUsername *bool `json:"cups_job_full_username,omitempty"`

	// Add the job ID to the beginning of the job title. Useful for debugging.
	PrefixJobIDToJobTitle *bool `json:"prefix_job_id_to_job_title,omitempty"`

	// Prefix for all GCP printers hosted by this connector.
	DisplayNamePrefix string `json:"display_name_prefix,omitempty"`

	// Ignore printers with native names.
	PrinterBlacklist []string `json:"printer_blacklist,omitempty"`

	// Allow printers with native names.
	PrinterWhitelist []string `json:"printer_whitelist,omitempty"`

	// Least severity to log.
	LogLevel string `json:"log_level"`

	// Local only: HTTP API port range, low.
	LocalPortLow uint16 `json:"local_port_low,omitempty"`

	// Local only: HTTP API port range, high.
	LocalPortHigh uint16 `json:"local_port_high,omitempty"`

	// CUPS only: Where to place log file.
	LogFileName string `json:"log_file_name"`

	// CUPS only: Maximum log file size.
	LogFileMaxMegabytes uint `json:"log_file_max_megabytes,omitempty"`

	// CUPS only: Maximum log file quantity.
	LogMaxFiles uint `json:"log_max_files,omitempty"`

	// CUPS only: Log to the systemd journal instead of to files?
	LogToJournal *bool `json:"log_to_journal,omitempty"`

	// CUPS only: Filename of unix socket for connector-check to talk to connector.
	MonitorSocketFilename string `json:"monitor_socket_filename,omitempty"`

	// CUPS only: Maximum quantity of open CUPS connections.
	CUPSMaxConnections uint `json:"cups_max_connections,omitempty"`

	// CUPS only: timeout for opening a new connection.
	CUPSConnectTimeout string `json:"cups_connect_timeout,omitempty"`

	// CUPS only: printer attributes to copy to GCP.
	CUPSPrinterAttributes []string `json:"cups_printer_attributes,omitempty"`

	// CUPS only: non-standard PPD options to add as GCP vendor capabilities.
	CUPSVendorPPDOptions []string `json:"cups_vendor_ppd_options,omitempty"`

	// CUPS only: ignore printers with make/model 'Local Raw Printer'.
	CUPSIgnoreRawPrinters *bool `json:"cups_ignore_raw_printers,omitempty"`

	// CUPS only: ignore printers with make/model 'Local Printer Class'.
	CUPSIgnoreClassPrinters *bool `json:"cups_ignore_class_printers,omitempty"`

	// CUPS only: copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName.
	// TODO: rename with cups_ prefix
	CUPSCopyPrinterInfoToDisplayName *bool `json:"copy_printer_info_to_display_name,omitempty"`
}

// DefaultConfig represents reasonable default values for Config fields.
// Omitted Config fields are omitted on purpose; they are unique per
// connector instance.
var DefaultConfig = Config{
	LocalPrintingEnable:    true,
	CloudPrintingEnable:    false,
	FcmNotificationsEnable: false,

	XMPPServer:                "talk.google.com",
	XMPPPort:                  443,
	XMPPPingTimeout:           "5s",
	XMPPPingInterval:          "2m",
	FcmServerBindUrl:          "https://fcm-stream.googleapis.com/fcm/connect/bind",
	GCPBaseURL:                "https://www.google.com/cloudprint/",
	GCPOAuthClientID:          "539833558011-35iq8btpgas80nrs3o7mv99hm95d4dv6.apps.googleusercontent.com",
	GCPOAuthClientSecret:      "V9BfPOvdiYuw12hDx5Y5nR0a",
	GCPOAuthAuthURL:           "https://accounts.google.com/o/oauth2/auth",
	GCPOAuthTokenURL:          "https://accounts.google.com/o/oauth2/token",
	GCPMaxConcurrentDownloads: 5,

	NativeJobQueueSize:        3,
	NativePrinterPollInterval: "1m",
	PrefixJobIDToJobTitle:     PointerToBool(false),
	DisplayNamePrefix:         "",
	PrinterBlacklist:          []string{},
	PrinterWhitelist:          []string{},
	LogLevel:                  "INFO",

	LocalPortLow:  26000,
	LocalPortHigh: 26999,

	LogFileName:         "/tmp/cloud-print-connector",
	LogFileMaxMegabytes: 1,
	LogMaxFiles:         3,
	LogToJournal:        PointerToBool(false),

	MonitorSocketFilename: "/tmp/cloud-print-connector-monitor.sock",

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
	CUPSJobFullUsername:              PointerToBool(false),
	CUPSIgnoreRawPrinters:            PointerToBool(true),
	CUPSIgnoreClassPrinters:          PointerToBool(true),
	CUPSCopyPrinterInfoToDisplayName: PointerToBool(true),
}

// getConfigFilename gets the absolute filename of the config file specified by
// the ConfigFilename flag, and whether it exists.
//
// If the (relative or absolute) ConfigFilename exists, then it is returned.
// If the ConfigFilename exists in a valid XDG path, then it is returned.
// If neither of those exist, the (relative or absolute) ConfigFilename is returned.
func getConfigFilename(context *cli.Context) (string, bool) {
	cf := context.GlobalString("config-filename")

	if filepath.IsAbs(cf) {
		// Absolute path specified; user knows what they want.
		_, err := os.Stat(cf)
		return cf, err == nil
	}

	absCF, err := filepath.Abs(cf)
	if err != nil {
		// syscall failure; treat as if file doesn't exist.
		return cf, false
	}
	if _, err := os.Stat(absCF); err == nil {
		// File exists on relative path.
		return absCF, true
	}

	if xdgCF, err := xdg.Config.Find(cf); err == nil {
		// File exists in an XDG directory.
		return xdgCF, true
	}

	// Default to relative path. This is probably what the user expects if
	// it wasn't found anywhere else.
	return absCF, false
}

// Backfill returns a copy of this config with all missing keys set to default values.
func (c *Config) Backfill(configMap map[string]interface{}) *Config {
	b := *c.commonBackfill(configMap)

	if _, exists := configMap["log_file_name"]; !exists {
		b.LogFileName = DefaultConfig.LogFileName
	}
	if _, exists := configMap["log_file_max_megabytes"]; !exists {
		b.LogFileMaxMegabytes = DefaultConfig.LogFileMaxMegabytes
	}
	if _, exists := configMap["log_max_files"]; !exists {
		b.LogMaxFiles = DefaultConfig.LogMaxFiles
	}
	if _, exists := configMap["log_to_journal"]; !exists {
		b.LogToJournal = DefaultConfig.LogToJournal
	}
	if _, exists := configMap["monitor_socket_filename"]; !exists {
		b.MonitorSocketFilename = DefaultConfig.MonitorSocketFilename
	}
	if _, exists := configMap["cups_max_connections"]; !exists {
		b.CUPSMaxConnections = DefaultConfig.CUPSMaxConnections
	}
	if _, exists := configMap["cups_connect_timeout"]; !exists {
		b.CUPSConnectTimeout = DefaultConfig.CUPSConnectTimeout
	}
	if _, exists := configMap["cups_printer_attributes"]; !exists {
		b.CUPSPrinterAttributes = DefaultConfig.CUPSPrinterAttributes
	} else {
		// Make sure all required attributes are present.
		s := make(map[string]struct{}, len(b.CUPSPrinterAttributes))
		for _, a := range b.CUPSPrinterAttributes {
			s[a] = struct{}{}
		}
		for _, a := range DefaultConfig.CUPSPrinterAttributes {
			if _, exists := s[a]; !exists {
				b.CUPSPrinterAttributes = append(b.CUPSPrinterAttributes, a)
			}
		}
	}
	if _, exists := configMap["cups_job_full_username"]; !exists {
		b.CUPSJobFullUsername = DefaultConfig.CUPSJobFullUsername
	}
	if _, exists := configMap["cups_ignore_raw_printers"]; !exists {
		b.CUPSIgnoreRawPrinters = DefaultConfig.CUPSIgnoreRawPrinters
	}
	if _, exists := configMap["cups_ignore_class_printers"]; !exists {
		b.CUPSIgnoreClassPrinters = DefaultConfig.CUPSIgnoreClassPrinters
	}
	if _, exists := configMap["copy_printer_info_to_display_name"]; !exists {
		b.CUPSCopyPrinterInfoToDisplayName = DefaultConfig.CUPSCopyPrinterInfoToDisplayName
	}

	return &b
}

// Sparse returns a copy of this config with obvious values removed.
func (c *Config) Sparse(context *cli.Context) *Config {
	s := *c.commonSparse(context)

	if !context.IsSet("log-file-max-megabytes") &&
		s.LogFileMaxMegabytes == DefaultConfig.LogFileMaxMegabytes {
		s.LogFileMaxMegabytes = 0
	}
	if !context.IsSet("log-max-files") &&
		s.LogMaxFiles == DefaultConfig.LogMaxFiles {
		s.LogMaxFiles = 0
	}
	if !context.IsSet("log-to-journal") &&
		reflect.DeepEqual(s.LogToJournal, DefaultConfig.LogToJournal) {
		s.LogToJournal = nil
	}
	if !context.IsSet("monitor-socket-filename") &&
		s.MonitorSocketFilename == DefaultConfig.MonitorSocketFilename {
		s.MonitorSocketFilename = ""
	}
	if !context.IsSet("cups-max-connections") &&
		s.CUPSMaxConnections == DefaultConfig.CUPSMaxConnections {
		s.CUPSMaxConnections = 0
	}
	if !context.IsSet("cups-connect-timeout") &&
		s.CUPSConnectTimeout == DefaultConfig.CUPSConnectTimeout {
		s.CUPSConnectTimeout = ""
	}
	if reflect.DeepEqual(s.CUPSPrinterAttributes, DefaultConfig.CUPSPrinterAttributes) {
		s.CUPSPrinterAttributes = nil
	}
	if !context.IsSet("cups-job-full-username") &&
		reflect.DeepEqual(s.CUPSJobFullUsername, DefaultConfig.CUPSJobFullUsername) {
		s.CUPSJobFullUsername = nil
	}
	if !context.IsSet("cups-ignore-raw-printers") &&
		reflect.DeepEqual(s.CUPSIgnoreRawPrinters, DefaultConfig.CUPSIgnoreRawPrinters) {
		s.CUPSIgnoreRawPrinters = nil
	}
	if !context.IsSet("cups-ignore-class-printers") &&
		reflect.DeepEqual(s.CUPSIgnoreClassPrinters, DefaultConfig.CUPSIgnoreClassPrinters) {
		s.CUPSIgnoreClassPrinters = nil
	}
	if !context.IsSet("copy-printer-info-to-display-name") &&
		reflect.DeepEqual(s.CUPSCopyPrinterInfoToDisplayName, DefaultConfig.CUPSCopyPrinterInfoToDisplayName) {
		s.CUPSCopyPrinterInfoToDisplayName = nil
	}

	return &s
}
