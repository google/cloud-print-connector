// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin freebsd

package main

import (
	"os"
	"time"

	"github.com/google/cloud-print-connector/lib"
	"github.com/urfave/cli"
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

func main() {
	app := cli.NewApp()
	app.Name = "gcp-connector-util"
	app.Usage = lib.ConnectorName + " for CUPS utility tools"
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
		LocalPrintingEnable: localEnable,
		CloudPrintingEnable: true,

		XMPPJID:                   xmppJID,
		RobotRefreshToken:         robotRefreshToken,
		UserRefreshToken:          userRefreshToken,
		ShareScope:                shareScope,
		ProxyName:                 proxyName,
		XMPPServer:                lib.DefaultConfig.XMPPServer,
		XMPPPort:                  uint16(context.Int("xmpp-port")),
		XMPPPingTimeout:           context.String("xmpp-ping-timeout"),
		XMPPPingInterval:          context.String("xmpp-ping-interval"),
		GCPBaseURL:                context.String("gcp-base-url"),
		GCPOAuthClientID:          context.String("gcp-oauth-client-id"),
		GCPOAuthClientSecret:      context.String("gcp-oauth-client-secret"),
		GCPOAuthAuthURL:           context.String("gcp-oauth-auth-url"),
		GCPOAuthTokenURL:          context.String("gcp-oauth-token-url"),
		GCPMaxConcurrentDownloads: uint(context.Int("gcp-max-concurrent-downloads")),

		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		PrefixJobIDToJobTitle:     lib.PointerToBool(context.Bool("prefix-job-id-to-job-title")),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		PrinterWhitelist:          lib.DefaultConfig.PrinterWhitelist,
		LogLevel:                  context.String("log-level"),

		LocalPortLow:  uint16(context.Int("local-port-low")),
		LocalPortHigh: uint16(context.Int("local-port-high")),

		LogFileName:                      context.String("log-file-name"),
		LogFileMaxMegabytes:              uint(context.Int("log-file-max-megabytes")),
		LogMaxFiles:                      uint(context.Int("log-max-files")),
		LogToJournal:                     lib.PointerToBool(context.Bool("log-to-journal")),
		MonitorSocketFilename:            context.String("monitor-socket-filename"),
		CUPSMaxConnections:               uint(context.Int("cups-max-connections")),
		CUPSConnectTimeout:               context.String("cups-connect-timeout"),
		CUPSPrinterAttributes:            lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:              lib.PointerToBool(context.Bool("cups-job-full-username")),
		CUPSIgnoreRawPrinters:            lib.PointerToBool(context.Bool("cups-ignore-raw-printers")),
		CUPSIgnoreClassPrinters:          lib.PointerToBool(context.Bool("cups-ignore-class-printers")),
		CUPSCopyPrinterInfoToDisplayName: lib.PointerToBool(context.Bool("copy-printer-info-to-display-name")),
	}
}

// createLocalConfig creates a config object that supports local mode.
func createLocalConfig(context *cli.Context) *lib.Config {
	return &lib.Config{
		LocalPrintingEnable: true,
		CloudPrintingEnable: false,

		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		PrefixJobIDToJobTitle:     lib.PointerToBool(context.Bool("prefix-job-id-to-job-title")),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		PrinterWhitelist:          lib.DefaultConfig.PrinterWhitelist,
		LogLevel:                  context.String("log-level"),

		LocalPortLow:  uint16(context.Int("local-port-low")),
		LocalPortHigh: uint16(context.Int("local-port-high")),

		LogFileName:                      context.String("log-file-name"),
		LogFileMaxMegabytes:              uint(context.Int("log-file-max-megabytes")),
		LogMaxFiles:                      uint(context.Int("log-max-files")),
		LogToJournal:                     lib.PointerToBool(context.Bool("log-to-journal")),
		MonitorSocketFilename:            context.String("monitor-socket-filename"),
		CUPSMaxConnections:               uint(context.Int("cups-max-connections")),
		CUPSConnectTimeout:               context.String("cups-connect-timeout"),
		CUPSPrinterAttributes:            lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:              lib.PointerToBool(context.Bool("cups-job-full-username")),
		CUPSIgnoreRawPrinters:            lib.PointerToBool(context.Bool("cups-ignore-raw-printers")),
		CUPSIgnoreClassPrinters:          lib.PointerToBool(context.Bool("cups-ignore-class-printers")),
		CUPSCopyPrinterInfoToDisplayName: lib.PointerToBool(context.Bool("copy-printer-info-to-display-name")),
	}
}
