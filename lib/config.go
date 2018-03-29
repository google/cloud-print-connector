/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"runtime"

	"github.com/urfave/cli"
)

const (
	ConnectorName = "Google Cloud Print Connector"

	// A website with user-friendly information.
	ConnectorHomeURL = "https://github.com/google/cloud-print-connector"

	GCPAPIVersion = "2.0"
)

var (
	ConfigFilenameFlag = cli.StringFlag{
		Name:  "config-filename",
		Usage: "Connector config filename",
		Value: defaultConfigFilename,
	}

	// To be populated by something like:
	// go install -ldflags "-X github.com/google/cloud-print-connector/lib.BuildDate=`date +%Y.%m.%d`"
	BuildDate = "DEV"

	ShortName = platformName + " Connector " + BuildDate + "-" + runtime.GOOS

	FullName = ConnectorName + " for " + platformName + " version " + BuildDate + "-" + runtime.GOOS
)

// PointerToBool converts a boolean value (constant) to a pointer-to-bool.
func PointerToBool(b bool) *bool {
	return &b
}

// GetConfig reads a Config object from the config file indicated by the config
// filename flag. If no such file exists, then DefaultConfig is returned.
func GetConfig(context *cli.Context) (*Config, string, error) {
	cf, exists := getConfigFilename(context)
	if !exists {
		return &DefaultConfig, "", nil
	}

	configRaw, err := ioutil.ReadFile(cf)
	if err != nil {
		return nil, "", err
	}

	config := new(Config)
	if err = json.Unmarshal(configRaw, config); err != nil {
		return nil, "", err
	}

	// Same config as a map so that we can detect missing keys.
	var configMap map[string]interface{}
	if err = json.Unmarshal(configRaw, &configMap); err != nil {
		return nil, "", err
	}

	b := config.Backfill(configMap)

	return b, cf, nil
}

// ToFile writes this Config object to the config file indicated by ConfigFile.
func (c *Config) ToFile(context *cli.Context) (string, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}

	cf, _ := getConfigFilename(context)
	if err = ioutil.WriteFile(cf, b, 0600); err != nil {
		return "", err
	}
	return cf, nil
}

func (c *Config) commonSparse(context *cli.Context) *Config {
	s := *c

	if s.XMPPServer == DefaultConfig.XMPPServer {
		s.XMPPServer = ""
	}
	if !context.IsSet("xmpp-port") &&
		s.XMPPPort == DefaultConfig.XMPPPort {
		s.XMPPPort = 0
	}
	if !context.IsSet("xmpp-ping-timeout") &&
		s.XMPPPingTimeout == DefaultConfig.XMPPPingTimeout {
		s.XMPPPingTimeout = ""
	}
	if !context.IsSet("xmpp-ping-interval") &&
		s.XMPPPingInterval == DefaultConfig.XMPPPingInterval {
		s.XMPPPingInterval = ""
	}
	if s.GCPBaseURL == DefaultConfig.GCPBaseURL {
		s.GCPBaseURL = ""
	}
	if s.FcmServerBindUrl == DefaultConfig.FcmServerBindUrl {
		s.FcmServerBindUrl = ""
	}
	if s.GCPOAuthClientID == DefaultConfig.GCPOAuthClientID {
		s.GCPOAuthClientID = ""
	}
	if s.GCPOAuthClientSecret == DefaultConfig.GCPOAuthClientSecret {
		s.GCPOAuthClientSecret = ""
	}
	if s.GCPOAuthAuthURL == DefaultConfig.GCPOAuthAuthURL {
		s.GCPOAuthAuthURL = ""
	}
	if s.GCPOAuthTokenURL == DefaultConfig.GCPOAuthTokenURL {
		s.GCPOAuthTokenURL = ""
	}
	if !context.IsSet("gcp-max-concurrent-downloads") &&
		s.GCPMaxConcurrentDownloads == DefaultConfig.GCPMaxConcurrentDownloads {
		s.GCPMaxConcurrentDownloads = 0
	}
	if !context.IsSet("native-job-queue-size") &&
		s.NativeJobQueueSize == DefaultConfig.NativeJobQueueSize {
		s.NativeJobQueueSize = 0
	}
	if !context.IsSet("native-printer-poll-interval") &&
		s.NativePrinterPollInterval == DefaultConfig.NativePrinterPollInterval {
		s.NativePrinterPollInterval = ""
	}
	if !context.IsSet("cups-job-full-username") &&
		reflect.DeepEqual(s.CUPSJobFullUsername, DefaultConfig.CUPSJobFullUsername) {
		s.CUPSJobFullUsername = nil
	}
	if !context.IsSet("prefix-job-id-to-job-title") &&
		reflect.DeepEqual(s.PrefixJobIDToJobTitle, DefaultConfig.PrefixJobIDToJobTitle) {
		s.PrefixJobIDToJobTitle = nil
	}
	if !context.IsSet("display-name-prefix") &&
		s.DisplayNamePrefix == DefaultConfig.DisplayNamePrefix {
		s.DisplayNamePrefix = ""
	}
	if !context.IsSet("local-port-low") &&
		s.LocalPortLow == DefaultConfig.LocalPortLow {
		s.LocalPortLow = 0
	}
	if !context.IsSet("local-port-high") &&
		s.LocalPortHigh == DefaultConfig.LocalPortHigh {
		s.LocalPortHigh = 0
	}

	return &s
}

func (c *Config) commonBackfill(configMap map[string]interface{}) *Config {
	b := *c

	if _, exists := configMap["xmpp_server"]; !exists {
		b.XMPPServer = DefaultConfig.XMPPServer
	}
	if _, exists := configMap["xmpp_port"]; !exists {
		b.XMPPPort = DefaultConfig.XMPPPort
	}
	if _, exists := configMap["gcp_xmpp_ping_timeout"]; !exists {
		b.XMPPPingTimeout = DefaultConfig.XMPPPingTimeout
	}
	if _, exists := configMap["gcp_xmpp_ping_interval_default"]; !exists {
		b.XMPPPingInterval = DefaultConfig.XMPPPingInterval
	}
	if _, exists := configMap["gcp_base_url"]; !exists {
		b.GCPBaseURL = DefaultConfig.GCPBaseURL
	}
	if _, exists := configMap["fcm_server_bind_url"]; !exists {
		b.FcmServerBindUrl = DefaultConfig.FcmServerBindUrl
	}
	if _, exists := configMap["gcp_oauth_client_id"]; !exists {
		b.GCPOAuthClientID = DefaultConfig.GCPOAuthClientID
	}
	if _, exists := configMap["gcp_oauth_client_secret"]; !exists {
		b.GCPOAuthClientSecret = DefaultConfig.GCPOAuthClientSecret
	}
	if _, exists := configMap["gcp_oauth_auth_url"]; !exists {
		b.GCPOAuthAuthURL = DefaultConfig.GCPOAuthAuthURL
	}
	if _, exists := configMap["gcp_oauth_token_url"]; !exists {
		b.GCPOAuthTokenURL = DefaultConfig.GCPOAuthTokenURL
	}
	if _, exists := configMap["gcp_max_concurrent_downloads"]; !exists {
		b.GCPMaxConcurrentDownloads = DefaultConfig.GCPMaxConcurrentDownloads
	}
	if _, exists := configMap["cups_job_queue_size"]; !exists {
		b.NativeJobQueueSize = DefaultConfig.NativeJobQueueSize
	}
	if _, exists := configMap["cups_printer_poll_interval"]; !exists {
		b.NativePrinterPollInterval = DefaultConfig.NativePrinterPollInterval
	}
	if _, exists := configMap["cups_job_full_username"]; !exists {
		b.CUPSJobFullUsername = DefaultConfig.CUPSJobFullUsername
	}
	if _, exists := configMap["prefix_job_id_to_job_title"]; !exists {
		b.PrefixJobIDToJobTitle = DefaultConfig.PrefixJobIDToJobTitle
	}
	if _, exists := configMap["display_name_prefix"]; !exists {
		b.DisplayNamePrefix = DefaultConfig.DisplayNamePrefix
	}
	if _, exists := configMap["printer_blacklist"]; !exists {
		b.PrinterBlacklist = DefaultConfig.PrinterBlacklist
	}
	if _, exists := configMap["printer_whitelist"]; !exists {
		b.PrinterWhitelist = DefaultConfig.PrinterWhitelist
	}
	if _, exists := configMap["local_printing_enable"]; !exists {
		b.LocalPrintingEnable = DefaultConfig.LocalPrintingEnable
	}
	if _, exists := configMap["cloud_printing_enable"]; !exists {
		b.CloudPrintingEnable = DefaultConfig.CloudPrintingEnable
	}
	if _, exists := configMap["fcm_notifications_enable"]; !exists {
		b.FcmNotificationsEnable = DefaultConfig.FcmNotificationsEnable
	}
	if _, exists := configMap["log_level"]; !exists {
		b.LogLevel = DefaultConfig.LogLevel
	}
	if _, exists := configMap["local_port_low"]; !exists {
		b.LocalPortLow = DefaultConfig.LocalPortLow
	}
	if _, exists := configMap["local_port_high"]; !exists {
		b.LocalPortHigh = DefaultConfig.LocalPortHigh
	}

	return &b
}
