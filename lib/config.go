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
	ConfigFilename = flag.String("config-filename", "cpc.oauth.json", "Name of config file")
)

type Config struct {
	// Associated with robot account. Used for acquiring OAuth access tokens.
	RefreshToken string `json:"refresh_token,omitempty"`

	// Associated with root account. XMPP credential.
	XMPPJID string `json:"xmpp_jid"`

	// User-chosen name of this proxy. Should be unique per Google user account.
	Proxy string `json:"proxy"`

	// Maximum quantity of PDFs to fetch concurrently.
	MaxConcurrentFetch uint `json:"max_concurrent_fetch"`

	// CUPS job queue size.
	CUPSQueueSize uint `json:"cups_queue_size"`

	// Interval, in seconds, between CUPS printer status polls.
	CUPSPollIntervalPrinter uint `json:"cups_poll_interval_printer"`

	// Maximum interval, in seconds, between CUPS job status polls.
	CUPSPollIntervalJob uint `json:"cups_poll_interval_job"`

	// Copy CUPS printer-info attribute to GCP defaultDisplayName field.
	CopyPrinterInfoToDisplayName bool `json:"copy_printer_info"`
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
