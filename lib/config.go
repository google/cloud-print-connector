/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

import (
	"encoding/json"
	"flag"
	"io/ioutil"
)

var (
	ConfigFilename = flag.String(
		"config-filename", "cups-connector.config.json", "Name of config file")
)

type Config struct {
	// Associated with root account. XMPP credential.
	XMPPJID string `json:"xmpp_jid"`

	// Associated with robot account. Used for acquiring OAuth access tokens.
	RobotRefreshToken string `json:"robot_refresh_token"`

	// Associated with user account. Used for sharing GCP printers; may be omitted.
	UserRefreshToken string `json:"user_refresh_token,omitempty"`

	// Scope (user, group, domain) to share printers with.
	ShareScope string `json:"share_scope,omitempty"`

	// User-chosen name of this proxy. Should be unique per Google user account.
	ProxyName string `json:"proxy_name"`

	// Maximum quantity of PDFs to download concurrently.
	GCPMaxConcurrentDownloads uint `json:"gcp_max_concurrent_downloads"`

	// CUPS job queue size.
	CUPSJobQueueSize uint `json:"cups_job_queue_size"`

	// Interval (eg 10s, 1m) between CUPS printer status polls.
	CUPSPrinterPollInterval string `json:"cups_printer_poll_interval"`

	// CUPS printer attributes to copy to GCP.
	CUPSPrinterAttributes []string `json:"cups_printer_attributes"`

	// Whether to use the full username (joe@example.com) in CUPS jobs.
	CUPSJobFullUsername bool `json:"cups_job_full_username"`

	// Whether to ignore printers with make/model 'Local Raw Printer'.
	CUPSIgnoreRawPrinters bool `json:"cups_ignore_raw_printers"`

	// Whether to copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName.
	CopyPrinterInfoToDisplayName bool `json:"copy_printer_info_to_display_name"`

	// Filename of unix socket for connector-check to talk to connector.
	MonitorSocketFilename string `json:"monitor_socket_filename"`

	// GCP API URL prefix.
	GCPBaseURL string `json:"gcp_base_url"`

	// XMPP server FQDN.
	XMPPServer string `json:"xmpp_server"`

	// XMPP server port number.
	XMPPPort uint16 `json:"xmpp_port"`

	// OAuth2 client ID (not unique per client).
	GCPOAuthClientID string `json:"gcp_oauth_client_id"`

	// OAuth2 client secret (not unique per client).
	GCPOAuthClientSecret string `json:"gcp_oauth_client_secret"`

	// OAuth2 auth URL.
	GCPOAuthAuthURL string `json:"gcp_oauth_auth_url"`

	// OAuth2 token URL.
	GCPOAuthTokenURL string `json:"gcp_oauth_token_url"`
}

// ConfigFromFile reads a Config object from the config file indicated by
// ConfigFile.
func ConfigFromFile() (*Config, error) {
	if !flag.Parsed() {
		flag.Parse()
	}

	b, err := ioutil.ReadFile(*ConfigFilename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err = json.Unmarshal(b, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ToFile writes this Config object to the config file indicated by ConfigFile.
func (c *Config) ToFile() error {
	if !flag.Parsed() {
		flag.Parse()
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(*ConfigFilename, b, 0600); err != nil {
		return err
	}

	return nil
}
