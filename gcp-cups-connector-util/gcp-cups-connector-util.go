/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"

	"github.com/golang/glog"
)

var (
	initFlag = flag.Bool(
		"init", false,
		"Initialize a config file")
	monitorFlag = flag.Bool(
		"monitor", false,
		"Read stats from a running connector")
	deleteAllGCPPrintersFlag = flag.Bool(
		"delete-all-gcp-printers", false,
		"Delete all printers associated with this connector")
	updateConfigFileFlag = flag.Bool(
		"update-config-file", false,
		"Add new options to config file after update")
	deleteGCPJobFlag = flag.String(
		"delete-gcp-job", "",
		"Deletes one GCP job")
	cancelGCPJobFlag = flag.String(
		"cancel-gcp-job", "",
		"Cancels one GCP job")
	deleteAllGCPPrinterJobsFlag = flag.Bool(
		"delete-all-gcp-printer-jobs", false,
		"Delete all queued jobs associated with a printer")
	cancelAllGCPPrinterJobsFlag = flag.Bool(
		"cancel-all-gcp-printer-jobs", false,
		"Cancels all queued jobs associated with a printer")
	showGCPPrinterStatusFlag = flag.Bool(
		"show-gcp-printer-status", false,
		"Shows the current status of a printer and it's jobs")
	printerIdFlag = flag.String(
		"printer-id", "",
		"Specifies ID of printer to use")
)

func main() {
	flag.Parse()
	fmt.Println(lib.FullName)

	if *initFlag {
		initConfigFile()
	} else if *monitorFlag {
		monitorConnector()
	} else if *deleteAllGCPPrintersFlag {
		deleteAllGCPPrinters()
	} else if *updateConfigFileFlag {
		updateConfigFile()
	} else if *deleteGCPJobFlag != "" {
		deleteGCPJob()
	} else if *cancelGCPJobFlag != "" {
		cancelGCPJob()
	} else if *deleteAllGCPPrinterJobsFlag {
		if *printerIdFlag == "" {
			fmt.Println("-printer-id is required.")
		} else {
			deleteAllGCPPrinterJobs()
		}
	} else if *cancelAllGCPPrinterJobsFlag {
		if *printerIdFlag == "" {
			fmt.Println("-printer-id is required.")
		} else {
			cancelAllGCPPrinterJobs()
		}
	} else if *showGCPPrinterStatusFlag {
		if *printerIdFlag == "" {
			fmt.Println("-printer-id is required.")
		} else {
			showGCPPrinterStatus()
		}
	} else {
		fmt.Println("no tool specified")
	}
}

// getConfig returns a config object
func getConfig() *lib.Config {
	config, _, err := lib.GetConfig()
	if err != nil {
		panic(err)
	}
	return config
}

// getGCP returns a GoogleCloudPrint object
func getGCP(config *lib.Config) *gcp.GoogleCloudPrint {
	gcp, err := gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
		config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
		config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
		0, nil)
	if err != nil {
		panic(err)
	}
	return gcp
}

// updateConfigFile opens the config file, adds any missing fields,
// writes the config file back.
func updateConfigFile() {
	config, configFilename, err := lib.GetConfig()
	if err != nil {
		panic(err)
	}

	// Same config in []byte format.
	configRaw, err := ioutil.ReadFile(configFilename)
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
	} else {
		// Make sure all required attributes are present.
		s := make(map[string]struct{}, len(config.CUPSPrinterAttributes))
		for _, a := range config.CUPSPrinterAttributes {
			s[a] = struct{}{}
		}
		for _, a := range lib.DefaultConfig.CUPSPrinterAttributes {
			if _, exists := s[a]; !exists {
				dirty = true
				fmt.Printf("Added %s to cups_printer_attributes\n", a)
				config.CUPSPrinterAttributes = append(config.CUPSPrinterAttributes, a)
			}
		}
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
	if _, exists := configMap["snmp_enable"]; !exists {
		dirty = true
		fmt.Println("Added snmp_enable")
		config.SNMPEnable = lib.DefaultConfig.SNMPEnable
	}
	if _, exists := configMap["snmp_community"]; !exists {
		dirty = true
		fmt.Println("Added snmp_community")
		config.SNMPCommunity = lib.DefaultConfig.SNMPCommunity
	}
	if _, exists := configMap["snmp_max_connections"]; !exists {
		dirty = true
		fmt.Println("Added snmp_max_connections")
		config.SNMPMaxConnections = lib.DefaultConfig.SNMPMaxConnections
	}
	if _, exists := configMap["local_printing_enable"]; !exists {
		dirty = true
		fmt.Println("Added local_printing_enable")
		config.LocalPrintingEnable = lib.DefaultConfig.LocalPrintingEnable
	}
	if _, exists := configMap["cloud_printing_enable"]; !exists {
		dirty = true
		_, robot_token_exists := configMap["robot_refresh_token"]
		fmt.Println("Added cloud_printing_enable")
		if robot_token_exists {
			config.CloudPrintingEnable = true
		} else {
			config.CloudPrintingEnable = lib.DefaultConfig.CloudPrintingEnable
		}
	}

	if dirty {
		config.ToFile()
		fmt.Printf("Wrote %s\n", configFilename)
	} else {
		fmt.Println("Nothing to update")
	}
}

// deleteAllGCPPrinters finds all GCP printers associated with this
// connector, deletes them from GCP.
func deleteAllGCPPrinters() {
	config := getConfig()
	gcp := getGCP(config)

	printers, err := gcp.List()
	if err != nil {
		glog.Fatal(err)
	}

	ch := make(chan bool)
	for gcpID, name := range printers {
		go func(gcpID, name string) {
			err := gcp.Delete(gcpID)
			if err != nil {
				fmt.Printf("Failed to delete %s \"%s\": %s\n", gcpID, name, err)
			} else {
				fmt.Printf("Deleted %s \"%s\" from GCP\n", gcpID, name)
			}
			ch <- true
		}(gcpID, name)
	}

	for _ = range printers {
		<-ch
	}
}

// deleteGCPJob deletes one GCP job
func deleteGCPJob() {
	config := getConfig()
	gcp := getGCP(config)

	err := gcp.DeleteJob(*deleteGCPJobFlag)
	if err != nil {
		fmt.Printf("Failed to delete GCP job %s: %s\n", *deleteGCPJobFlag, err)
	} else {
		fmt.Printf("Deleted GCP job %s\n", *deleteGCPJobFlag)
	}
}

// cancelGCPJob cancels one GCP job
func cancelGCPJob() {
	config := getConfig()
	gcp := getGCP(config)

	cancelState := cdd.PrintJobStateDiff{
		State: &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		},
	}

	err := gcp.Control(*cancelGCPJobFlag, cancelState)
	if err != nil {
		fmt.Printf("Failed to cancel GCP job %s: %s\n", *cancelGCPJobFlag, err)
	} else {
		fmt.Printf("Canceled GCP job %s\n", *cancelGCPJobFlag)
	}
}

// deleteAllGCPPrinterJobs finds all GCP printer jobs associated with a
// a given printer id and deletes them.
func deleteAllGCPPrinterJobs() {
	config := getConfig()
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(*printerIdFlag)
	if err != nil {
		glog.Fatal(err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No queued jobs\n")
	}

	ch := make(chan bool)
	for _, job := range jobs {
		go func(gcpJobID string) {
			err := gcp.DeleteJob(gcpJobID)
			if err != nil {
				fmt.Printf("Failed to delete GCP job %s: %s\n", gcpJobID, err)
			} else {
				fmt.Printf("Deleted GCP job %s\n", gcpJobID)
			}
			ch <- true
		}(job.GCPJobID)
	}

	for _ = range jobs {
		<-ch
	}
}

// cancelAllGCPPrinterJobs finds all GCP printer jobs associated with a
// a given printer id and cancels them.
func cancelAllGCPPrinterJobs() {
	config := getConfig()
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(*printerIdFlag)
	if err != nil {
		glog.Fatal(err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No queued jobs\n")
	}

	cancelState := cdd.PrintJobStateDiff{
		State: &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		},
	}

	ch := make(chan bool)
	for _, job := range jobs {
		go func(gcpJobID string) {
			err := gcp.Control(gcpJobID, cancelState)
			if err != nil {
				fmt.Printf("Failed to cancel GCP job %s: %s\n", gcpJobID, err)
			} else {
				fmt.Printf("Cancelled GCP job %s\n", gcpJobID)
			}
			ch <- true
		}(job.GCPJobID)
	}

	for _ = range jobs {
		<-ch
	}
}

// showGCPPrinterStatus shows the current status of a GCP printer and it's jobs
func showGCPPrinterStatus() {
	config := getConfig()
	gcp := getGCP(config)

	printer, _, err := gcp.Printer(*printerIdFlag)
	if err != nil {
		glog.Fatal(err)
	}

	fmt.Println("Name:", printer.DefaultDisplayName)
	fmt.Println("State:", printer.State.State)

	jobs, err := gcp.Jobs(*printerIdFlag)
	if err != nil {
		glog.Fatal(err)
	}

	// Only init common states. Unusual states like DRAFT will only be shown
	// if there are jobs in that state.
	jobStateCounts := map[string]int{
		"DONE":        0,
		"ABORTED":     0,
		"QUEUED":      0,
		"STOPPED":     0,
		"IN_PROGRESS": 0,
	}

	for _, job := range jobs {
		jobState := string(job.SemanticState.State.Type)
		jobStateCounts[jobState]++
	}

	fmt.Println("Printer jobs:")
	for state, count := range jobStateCounts {
		fmt.Println(" ", state, ":", count)
	}
}
