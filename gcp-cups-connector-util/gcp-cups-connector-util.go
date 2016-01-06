/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
)

func main() {
	// Suppress date/time prefix.
	log.SetFlags(0)

	app := cli.NewApp()
	app.Name = "gcp-cups-connector-util"
	app.Usage = "Google Cloud Print CUPS Connector utility tools"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
	}
	app.Commands = []cli.Command{
		cli.Command{
			Name:      "init",
			ShortName: "i",
			Usage:     "Creates a config file",
			Action:    initConfigFile,
			Flags:     initFlags,
		},
		cli.Command{
			Name:      "monitor",
			ShortName: "m",
			Usage:     "Read stats from a running connector",
			Action:    monitorConnector,
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "monitor-timeout",
					Usage: "wait for a monitor response no more than this long",
					Value: 10 * time.Second,
				},
			},
		},
		cli.Command{
			Name:   "delete-all-gcp-printers",
			Usage:  "Delete all printers associated with this connector",
			Action: deleteAllGCPPrinters,
		},
		cli.Command{
			Name:   "update-config-file",
			Usage:  "Add new options to config file after update",
			Action: updateConfigFile,
		},
		cli.Command{
			Name:   "delete-gcp-job",
			Usage:  "Deletes one GCP job",
			Action: deleteGCPJob,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "job-id",
				},
			},
		},
		cli.Command{
			Name:   "cancel-gcp-job",
			Usage:  "Cancels one GCP job",
			Action: cancelGCPJob,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "job-id",
				},
			},
		},
		cli.Command{
			Name:   "delete-all-gcp-printer-jobs",
			Usage:  "Delete all queued jobs associated with a printer",
			Action: deleteAllGCPPrinterJobs,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "printer-id",
				},
			},
		},
		cli.Command{
			Name:   "cancel-all-gcp-printer-jobs",
			Usage:  "Cancels all queued jobs associated with a printer",
			Action: cancelAllGCPPrinterJobs,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "printer-id",
				},
			},
		},
		cli.Command{
			Name:   "show-gcp-printer-status",
			Usage:  "Shows the current status of a printer and it's jobs",
			Action: showGCPPrinterStatus,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "printer-id",
				},
			},
		},
		cli.Command{
			Name:   "share-gcp-printer",
			Usage:  "Shares a printer with user or group",
			Action: shareGCPPrinter,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "printer-id",
					Usage: "Printer to share.",
				},
				cli.StringFlag{
					Name:  "email",
					Usage: "Group or user to share with.",
				},
				cli.StringFlag{
					Name:  "role",
					Value: "USER",
					Usage: "Role granted. user or manager.",
				},
				cli.BoolTFlag{
					Name:  "skip-notification",
					Usage: "Skip sending email notice. Defaults to true",
				},
			},
		},
		cli.Command{
			Name:   "unshare-gcp-printer",
			Usage:  "Removes user or group access to printer.",
			Action: unshareGCPPrinter,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "printer-id",
					Usage: "Printer to unshare.",
				},
				cli.StringFlag{
					Name:  "email",
					Usage: "Group or user to remove.",
				},
			},
		},
	}

	app.Run(os.Args)
}

// getConfig returns a config object
func getConfig(context *cli.Context) *lib.Config {
	config, _, err := lib.GetConfig(context)
	if err != nil {
		log.Fatalln(err)
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
		log.Fatalln(err)
	}
	return gcp
}

// updateConfigFile opens the config file, adds any missing fields,
// writes the config file back.
func updateConfigFile(context *cli.Context) {
	config, configFilename, err := lib.GetConfig(context)
	if err != nil {
		log.Fatalln(err)
	}
	if configFilename == "" {
		fmt.Println("Could not find a config file to update")
		return
	}

	// Same config in []byte format.
	configRaw, err := ioutil.ReadFile(configFilename)
	if err != nil {
		log.Fatalln(err)
	}

	// Same config in map format so that we can detect missing keys.
	var configMap map[string]interface{}
	if err = json.Unmarshal(configRaw, &configMap); err != nil {
		log.Fatalln(err)
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
	if _, exists := configMap["display_name_prefix"]; !exists {
		dirty = true
		fmt.Println("Added display_name_prefix")
		config.DisplayNamePrefix = lib.DefaultConfig.DisplayNamePrefix
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
		config.XMPPPingInterval = lib.DefaultConfig.XMPPPingInterval
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
	if _, exists := configMap["log_file_name"]; !exists {
		dirty = true
		fmt.Println("Added log_file_name")
		config.LogFileName = lib.DefaultConfig.LogFileName
	}
	if _, exists := configMap["log_file_max_megabytes"]; !exists {
		dirty = true
		fmt.Println("Added log_file_max_megabytes")
		config.LogFileMaxMegabytes = lib.DefaultConfig.LogFileMaxMegabytes
	}
	if _, exists := configMap["log_max_files"]; !exists {
		dirty = true
		fmt.Println("Added log_max_files")
		config.LogMaxFiles = lib.DefaultConfig.LogMaxFiles
	}
	if _, exists := configMap["log_level"]; !exists {
		dirty = true
		fmt.Println("Added log_level")
		config.LogLevel = lib.DefaultConfig.LogLevel
	}
	if _, exists := configMap["log_to_journal"]; !exists {
		dirty = true
		fmt.Println("Added log_to_journal")
		config.LogToJournal = lib.DefaultConfig.LogToJournal
	}
	if _, exists := configMap["printer_blacklist"]; !exists {
		dirty = true
		fmt.Println("Added printer_blacklist")
		config.PrinterBlacklist = lib.DefaultConfig.PrinterBlacklist
	}

	if dirty {
		config.ToFile(context)
		fmt.Printf("Wrote %s\n", configFilename)
	} else {
		fmt.Println("Nothing to update")
	}
}

// deleteAllGCPPrinters finds all GCP printers associated with this
// connector, deletes them from GCP.
func deleteAllGCPPrinters(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	printers, err := gcp.List()
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	for gcpID, name := range printers {
		wg.Add(1)
		go func(gcpID, name string) {
			err := gcp.Delete(gcpID)
			if err != nil {
				fmt.Printf("Failed to delete %s \"%s\": %s\n", gcpID, name, err)
			} else {
				fmt.Printf("Deleted %s \"%s\" from GCP\n", gcpID, name)
			}
			wg.Done()
		}(gcpID, name)
	}
	wg.Wait()
}

// deleteGCPJob deletes one GCP job
func deleteGCPJob(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	err := gcp.DeleteJob(context.String("job-id"))
	if err != nil {
		fmt.Printf("Failed to delete GCP job %s: %s\n", context.String("job-id"), err)
	} else {
		fmt.Printf("Deleted GCP job %s\n", context.String("job-id"))
	}
}

// cancelGCPJob cancels one GCP job
func cancelGCPJob(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	cancelState := cdd.PrintJobStateDiff{
		State: &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		},
	}

	err := gcp.Control(context.String("job-id"), cancelState)
	if err != nil {
		fmt.Printf("Failed to cancel GCP job %s: %s\n", context.String("job-id"), err)
	} else {
		fmt.Printf("Canceled GCP job %s\n", context.String("job-id"))
	}
}

// deleteAllGCPPrinterJobs finds all GCP printer jobs associated with a
// a given printer id and deletes them.
func deleteAllGCPPrinterJobs(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
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
func cancelAllGCPPrinterJobs(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
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
func showGCPPrinterStatus(context *cli.Context) {
	config := getConfig(context)
	gcp := getGCP(config)

	printer, _, err := gcp.Printer(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Name:", printer.DefaultDisplayName)
	fmt.Println("State:", printer.State.State)

	jobs, err := gcp.Jobs(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
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

// shareGCPPrinter shares a GCP printer
func shareGCPPrinter(context *cli.Context) {
	config := getConfig(context)
	gcpConn := getGCP(config)

	var role gcp.Role
	switch strings.ToUpper(context.String("role")) {
	case "USER":
		role = gcp.User
	case "MANAGER":
		role = gcp.Manager
	default:
		fmt.Println("role should be user or manager.")
		return
	}

	err := gcpConn.Share(context.String("printer-id"), context.String("email"),
		role, context.Bool("skip-notification"))
	if err != nil {
		fmt.Printf("Failed to share GCP printer %s with %s: %s\n", context.String("printer-id"), context.String("email"), err)
	} else {
		fmt.Printf("Shared GCP printer %s with %s\n", context.String("printer-id"), context.String("email"))
	}
}

// unshareGCPPrinter unshares a GCP printer.
func unshareGCPPrinter(context *cli.Context) {
	config := getConfig(context)
	gcpConn := getGCP(config)

	err := gcpConn.Unshare(context.String("printer-id"), context.String("email"))
	if err != nil {
		fmt.Printf("Failed to unshare GCP printer %s with %s: %s\n", context.String("printer-id"), context.String("email"), err)
	} else {
		fmt.Printf("Unshared GCP printer %s with %s\n", context.String("printer-id"), context.String("email"))
	}
}
