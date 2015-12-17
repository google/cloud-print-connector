// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/cups-connector/lib"
)

var unixInitFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "log-file-name",
		Usage: "Log file name, full path",
		Value: lib.DefaultConfig.LogFileName,
	},
	cli.IntFlag{
		Name:  "log-file-max-megabytes",
		Usage: "Log file max size, in megabytes",
		Value: int(lib.DefaultConfig.LogFileMaxMegabytes),
	},
	cli.IntFlag{
		Name:  "log-max-files",
		Usage: "Maximum log file quantity before rollover",
		Value: int(lib.DefaultConfig.LogMaxFiles),
	},
	cli.BoolFlag{
		Name:  "log-to-journal",
		Usage: "Log to the systemd journal (if available) instead of to log-file-name",
	},
	cli.StringFlag{
		Name:  "monitor-socket-filename",
		Usage: "Filename of unix socket for connector-check to talk to connector",
		Value: lib.DefaultConfig.MonitorSocketFilename,
	},
	cli.IntFlag{
		Name:  "cups-max-connections",
		Usage: "Max connections to CUPS server",
		Value: int(lib.DefaultConfig.CUPSMaxConnections),
	},
	cli.StringFlag{
		Name:  "cups-connect-timeout",
		Usage: "CUPS timeout for opening a new connection",
		Value: lib.DefaultConfig.CUPSConnectTimeout,
	},
	cli.BoolFlag{
		Name:  "cups-job-full-username",
		Usage: "Whether to use the full username (joe@example.com) in CUPS jobs",
	},
	cli.BoolTFlag{
		Name:  "cups-ignore-raw-printers",
		Usage: "Whether to ignore CUPS raw printers",
	},
	cli.BoolTFlag{
		Name:  "cups-ignore-class-printers",
		Usage: "Whether to ignore CUPS class printers",
	},
	cli.BoolTFlag{
		Name:  "copy-printer-info-to-display-name",
		Usage: "Whether to copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName",
	},
}

var unixCommands = []cli.Command{
	cli.Command{
		Name:      "init",
		ShortName: "i",
		Usage:     "Creates a config file",
		Action:    initConfigFile,
		Flags:     append(commonInitFlags, unixInitFlags...),
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
}

func updateConfig(config *lib.Config, configMap map[string]interface{}) bool {
	dirty := commonUpdateConfig(config, configMap)

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
	if _, exists := configMap["log_to_journal"]; !exists {
		dirty = true
		fmt.Println("Added log_to_journal")
		config.LogToJournal = lib.DefaultConfig.LogToJournal
	}
	if _, exists := configMap["monitor_socket_filename"]; !exists {
		dirty = true
		fmt.Println("Added monitor_socket_filename")
		config.MonitorSocketFilename = lib.DefaultConfig.MonitorSocketFilename
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
	if _, exists := configMap["cups_ignore_class_printers"]; !exists {
		dirty = true
		fmt.Println("Added cups_ignore_class_printers")
		config.CUPSIgnoreClassPrinters = lib.DefaultConfig.CUPSIgnoreClassPrinters
	}
	if _, exists := configMap["copy_printer_info_to_display_name"]; !exists {
		dirty = true
		fmt.Println("Added copy_printer_info_to_display_name")
		config.CUPSCopyPrinterInfoToDisplayName = lib.DefaultConfig.CUPSCopyPrinterInfoToDisplayName
	}

	return dirty
}

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
	app.Commands = append(unixCommands, commonCommands...)

	app.Run(os.Args)
}

// createCloudConfig creates a config object that supports cloud and (optionally) local mode.
func createCloudConfig(context *cli.Context, xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName string, localEnable bool) *lib.Config {
	return &lib.Config{
		XMPPJID:                   xmppJID,
		RobotRefreshToken:         robotRefreshToken,
		UserRefreshToken:          userRefreshToken,
		ShareScope:                shareScope,
		ProxyName:                 proxyName,
		XMPPServer:                lib.DefaultConfig.XMPPServer,
		XMPPPort:                  uint16(context.Int("xmpp-port")),
		XMPPPingTimeout:           context.String("xmpp-ping-timeout"),
		XMPPPingInterval:          context.String("xmpp-ping-interval"),
		GCPBaseURL:                lib.DefaultConfig.GCPBaseURL,
		GCPOAuthClientID:          lib.DefaultConfig.GCPOAuthClientID,
		GCPOAuthClientSecret:      lib.DefaultConfig.GCPOAuthClientSecret,
		GCPOAuthAuthURL:           lib.DefaultConfig.GCPOAuthAuthURL,
		GCPOAuthTokenURL:          lib.DefaultConfig.GCPOAuthTokenURL,
		GCPMaxConcurrentDownloads: uint(context.Int("gcp-max-concurrent-downloads")),

		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		PrefixJobIDToJobTitle:     context.Bool("prefix-job-id-to-job-title"),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		SNMPEnable:                context.Bool("snmp-enable"),
		SNMPCommunity:             context.String("snmp-community"),
		SNMPMaxConnections:        uint(context.Int("snmp-max-connections")),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		LocalPrintingEnable:       localEnable,
		CloudPrintingEnable:       true,
		LogLevel:                  context.String("log-level"),

		LogFileName:                      context.String("log-file-name"),
		LogFileMaxMegabytes:              uint(context.Int("log-file-max-megabytes")),
		LogMaxFiles:                      uint(context.Int("log-max-files")),
		LogToJournal:                     context.Bool("log-to-journal"),
		MonitorSocketFilename:            context.String("monitor-socket-filename"),
		CUPSMaxConnections:               uint(context.Int("cups-max-connections")),
		CUPSConnectTimeout:               context.String("cups-connect-timeout"),
		CUPSPrinterAttributes:            lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:              context.Bool("cups-job-full-username"),
		CUPSIgnoreRawPrinters:            context.Bool("cups-ignore-raw-printers"),
		CUPSIgnoreClassPrinters:          context.Bool("cups-ignore-class-printers"),
		CUPSCopyPrinterInfoToDisplayName: context.Bool("cups-copy-printer-info-to-display-name"),
	}
}

// createLocalConfig creates a config object that supports local mode.
func createLocalConfig(context *cli.Context) *lib.Config {
	return &lib.Config{
		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		PrefixJobIDToJobTitle:     context.Bool("prefix-job-id-to-job-title"),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		SNMPEnable:                context.Bool("snmp-enable"),
		SNMPCommunity:             context.String("snmp-community"),
		SNMPMaxConnections:        uint(context.Int("snmp-max-connections")),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		LocalPrintingEnable:       true,
		CloudPrintingEnable:       false,
		LogLevel:                  context.String("log-level"),

		LogFileName:                      context.String("log-file-name"),
		LogFileMaxMegabytes:              uint(context.Int("log-file-max-megabytes")),
		LogMaxFiles:                      uint(context.Int("log-max-files")),
		LogToJournal:                     context.Bool("log-to-journal"),
		MonitorSocketFilename:            context.String("monitor-socket-filename"),
		CUPSMaxConnections:               uint(context.Int("cups-max-connections")),
		CUPSConnectTimeout:               context.String("cups-connect-timeout"),
		CUPSPrinterAttributes:            lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:              context.Bool("cups-job-full-username"),
		CUPSIgnoreRawPrinters:            context.Bool("cups-ignore-raw-printers"),
		CUPSIgnoreClassPrinters:          context.Bool("cups-ignore-class-printers"),
		CUPSCopyPrinterInfoToDisplayName: context.Bool("cups-copy-printer-info-to-display-name"),
	}
}
