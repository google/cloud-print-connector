// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package lib

import (
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"
)

const (
	platformName = "Windows"

	defaultConfigFilename = "gcp-windows-connector.config.json"
)

type Config struct {
	// Enable local discovery and printing.
	LocalPrintingEnable bool `json:"local_printing_enable"`

	// Enable cloud discovery and printing.
	CloudPrintingEnable bool `json:"cloud_printing_enable"`

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

	// Windows Spooler job queue size, must be greater than zero.
	// TODO: rename without cups_ prefix
	NativeJobQueueSize uint `json:"cups_job_queue_size,omitempty"`

	// Interval (eg 10s, 1m) between Windows Spooler printer state polls.
	// TODO: rename without cups_ prefix
	NativePrinterPollInterval string `json:"cups_printer_poll_interval,omitempty"`

	// Add the job ID to the beginning of the job title. Useful for debugging.
	PrefixJobIDToJobTitle *bool `json:"prefix_job_id_to_job_title,omitempty"`

	// Prefix for all GCP printers hosted by this connector.
	DisplayNamePrefix string `json:"display_name_prefix,omitempty"`

	// Ignore printers with native names.
	PrinterBlacklist []string `json:"printer_blacklist,omitempty"`

	// Least severity to log.
	LogLevel string `json:"log_level"`

	// Local only: HTTP API port range, low.
	LocalPortLow uint16 `json:"local_port_low,omitempty"`

	// Local only: HTTP API port range, high.
	LocalPortHigh uint16 `json:"local_port_high,omitempty"`
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
	PrefixJobIDToJobTitle:     PointerToBool(false),
	DisplayNamePrefix:         "",
	PrinterBlacklist: []string{
		"Fax",
		"CutePDF Writer",
		"Microsoft XPS Document Writer",
		"Google Cloud Printer",
	},
	LocalPrintingEnable: true,
	CloudPrintingEnable: false,
	LogLevel:            "INFO",

	LocalPortLow:  26000,
	LocalPortHigh: 26999,
}

// getConfigFilename gets the absolute filename of the config file specified by
// the ConfigFilename flag, and whether it exists.
//
// If the ConfigFilename exists, then it is returned as an absolute path.
// If neither of those exist, the absolute ConfigFilename is returned.
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

	// Check for config file on path relative to executable.
	exeFile := os.Args[0]
	exeDir := filepath.Dir(exeFile)
	absCF = filepath.Join(exeDir, cf)
	if _, err := os.Stat(absCF); err == nil {
		return absCF, true
	}

	// This is probably what the user expects if it wasn't found anywhere else.
	return absCF, false
}

// Backfill returns a copy of this config with all missing keys set to default values.
func (c *Config) Backfill(configMap map[string]interface{}) *Config {
	return c.commonBackfill(configMap)
}

// Sparse returns a copy of this config with obvious values removed.
func (c *Config) Sparse(context *cli.Context) *Config {
	return c.commonSparse(context)
}
