// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/google/cups-connector/lib"
)

var windowsCommands = []cli.Command{
	cli.Command{
		Name:      "init",
		ShortName: "i",
		Usage:     "Creates a config file",
		Action:    initConfigFile,
		Flags:     commonInitFlags,
	},
}

func updateConfig(config *lib.Config, configMap map[string]interface{}) bool {
	return commonUpdateConfig(config, configMap)
}

func main() {
	// Suppress date/time prefix.
	log.SetFlags(0)

	app := cli.NewApp()
	app.Name = "gcp-windows-connector-util"
	app.Usage = "Google Cloud Print Windows Connector utility tools"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
	}
	app.Commands = append(windowsCommands, commonCommands...)

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
	}
}
