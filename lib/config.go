/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

import (
	"encoding/json"
	"flag"
	"io/ioutil"
)

var (
	ConfigFilename = flag.String(
		"config-filename", "cups-connector.oauth.json", "Name of config file")
	DefaultPrinterAttributes = []string{
		"printer-name",
		"printer-info",
		"printer-is-accepting-jobs",
		"printer-location",
		"printer-make-and-model",
		"printer-state",
		"printer-state-reasons",
	}
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

	// Whether to copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName.
	CopyPrinterInfoToDisplayName bool `json:"copy_printer_info_to_display_name"`

	// Filename of unix socket for connector-check to talk to connector.
	MonitorSocketFilename string `json:"monitor_socket_filename"`
}

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
