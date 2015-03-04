/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"cups-connector/gcp"
	"cups-connector/lib"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/glog"
)

var (
	deleteAllGCPPrintersFlag = flag.Bool(
		"delete-all-gcp-printers", false,
		"Delete all printers associated with this connector")
	updateConfigFileFlag = flag.Bool(
		"update-config-file", false,
		"Add new options to config file after update")
)

func main() {
	flag.Parse()
	fmt.Println("Google Cloud Print CUPS Connector version", lib.GetBuildDate())

	if *deleteAllGCPPrintersFlag {
		deleteAllGCPPrinters()
	} else if *updateConfigFileFlag {
		updateConfigFile()
	} else {
		fmt.Println("no tool specified")
	}
}

// updateConfigFile opens the config file, adds any missing fields,
// writes the config file back.
func updateConfigFile() {
	// Config as parsed by the connector.
	config, err := lib.ConfigFromFile()
	if err != nil {
		panic(err)
	}

	// Same config in []byte format.
	configRaw, err := ioutil.ReadFile(*lib.ConfigFilename)
	if err != nil {
		panic(err)
	}

	// Same config in map format so that we can detect missing keys.
	var configMap map[string]interface{}
	if err = json.Unmarshal(configRaw, &configMap); err != nil {
		panic(err)
	}

	// No changes detected yet.
	dirty := false

	if _, exists := configMap["gcp_max_concurrent_downloads"]; !exists {
		dirty = true
		fmt.Println("Added gcp_max_concurrent_downloads")
		config.GCPMaxConcurrentDownloads = lib.DefaultConfig.GCPMaxConcurrentDownloads
	}
	if _, exists := configMap["cups_max_connections"]; !exists {
		dirty = true
		fmt.Println("Added cups_max_connections")
		config.CUPSMaxConnections = lib.DefaultConfig.CUPSMaxConnections
	}
	if _, exists := configMap["cups_connect_timeout"]; !exists {
		dirty = true
		fmt.Println("Added cups_connect_timeout")
		config.CUPSConnectTimeout = lib.DefaultConfig.CUPSConnectTimeout
	}
	if _, exists := configMap["cups_job_queue_size"]; !exists {
		dirty = true
		fmt.Println("Added cups_job_queue_size")
		config.CUPSJobQueueSize = lib.DefaultConfig.CUPSJobQueueSize
	}
	if _, exists := configMap["cups_printer_poll_interval"]; !exists {
		dirty = true
		fmt.Println("Added cups_printer_poll_interval")
		config.CUPSPrinterPollInterval = lib.DefaultConfig.CUPSPrinterPollInterval
	}
	if _, exists := configMap["cups_printer_attributes"]; !exists {
		dirty = true
		fmt.Println("Added cups_printer_attributes")
		config.CUPSPrinterAttributes = lib.DefaultConfig.CUPSPrinterAttributes
	}
	if _, exists := configMap["cups_job_full_username"]; !exists {
		dirty = true
		fmt.Println("Added cups_job_full_username")
		config.CUPSJobFullUsername = lib.DefaultConfig.CUPSJobFullUsername
	}
	if _, exists := configMap["cups_ignore_raw_printers"]; !exists {
		dirty = true
		fmt.Println("Added cups_ignore_raw_printers")
		config.CUPSIgnoreRawPrinters = lib.DefaultConfig.CUPSIgnoreRawPrinters
	}
	if _, exists := configMap["copy_printer_info_to_display_name"]; !exists {
		dirty = true
		fmt.Println("Added copy_printer_info_to_display_name")
		config.CopyPrinterInfoToDisplayName = lib.DefaultConfig.CopyPrinterInfoToDisplayName
	}
	if _, exists := configMap["monitor_socket_filename"]; !exists {
		dirty = true
		fmt.Println("Added monitor_socket_filename")
		config.MonitorSocketFilename = lib.DefaultConfig.MonitorSocketFilename
	}
	if _, exists := configMap["gcp_base_url"]; !exists {
		dirty = true
		fmt.Println("Added gcp_base_url")
		config.GCPBaseURL = lib.DefaultConfig.GCPBaseURL
	}
	if _, exists := configMap["xmpp_server"]; !exists {
		dirty = true
		fmt.Println("Added xmpp_server")
		config.XMPPServer = lib.DefaultConfig.XMPPServer
	}
	if _, exists := configMap["xmpp_port"]; !exists {
		dirty = true
		fmt.Println("Added xmpp_port")
		config.XMPPPort = lib.DefaultConfig.XMPPPort
	}
	if _, exists := configMap["gcp_xmpp_ping_timeout"]; !exists {
		dirty = true
		fmt.Println("Added gcp_xmpp_ping_timeout")
		config.XMPPPingTimeout = lib.DefaultConfig.XMPPPingTimeout
	}
	if _, exists := configMap["gcp_xmpp_ping_interval_default"]; !exists {
		dirty = true
		fmt.Println("Added gcp_xmpp_ping_interval_default")
		config.XMPPPingIntervalDefault = lib.DefaultConfig.XMPPPingIntervalDefault
	}
	if _, exists := configMap["gcp_oauth_client_id"]; !exists {
		dirty = true
		fmt.Println("Added gcp_oauth_client_id")
		config.GCPOAuthClientID = lib.DefaultConfig.GCPOAuthClientID
	}
	if _, exists := configMap["gcp_oauth_client_secret"]; !exists {
		dirty = true
		fmt.Println("Added gcp_oauth_client_secret")
		config.GCPOAuthClientSecret = lib.DefaultConfig.GCPOAuthClientSecret
	}
	if _, exists := configMap["gcp_oauth_auth_url"]; !exists {
		dirty = true
		fmt.Println("Added gcp_oauth_auth_url")
		config.GCPOAuthAuthURL = lib.DefaultConfig.GCPOAuthAuthURL
	}
	if _, exists := configMap["gcp_oauth_token_url"]; !exists {
		dirty = true
		fmt.Println("Added gcp_oauth_token_url")
		config.GCPOAuthTokenURL = lib.DefaultConfig.GCPOAuthTokenURL
	}

	if dirty {
		config.ToFile()
		fmt.Printf("Wrote %s\n", *lib.ConfigFilename)
	} else {
		fmt.Println("Nothing to update")
	}
}

// deleteAllGCPPrinters finds all GCP printers associated with this
// connector, deletes them from GCP.
func deleteAllGCPPrinters() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		panic(err)
	}

	gcpXMPPPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping timeout: %s", err)
	}
	gcpXMPPPingIntervalDefault, err := time.ParseDuration(config.XMPPPingIntervalDefault)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping interval default: %s", err)
	}

	gcp, err := gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.XMPPJID, config.RobotRefreshToken,
		config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID, config.GCPOAuthClientSecret,
		config.GCPOAuthAuthURL, config.GCPOAuthTokenURL, config.XMPPServer, config.XMPPPort,
		gcpXMPPPingTimeout, gcpXMPPPingIntervalDefault)
	if err != nil {
		glog.Fatal(err)
	}

	printers, _, _, err := gcp.List()
	if err != nil {
		glog.Fatal(err)
	}

	ch := make(chan bool)
	for _, p := range printers {
		go func(gcpID, name string) {
			err := gcp.Delete(gcpID)
			if err != nil {
				fmt.Printf("Failed to delete %s \"%s\": %s\n", gcpID, name, err)
			} else {
				fmt.Printf("Deleted %s \"%s\" from GCP\n", gcpID, name)
			}
			ch <- true
		}(p.GCPID, p.Name)
	}

	for _ = range printers {
		<-ch
	}
}
