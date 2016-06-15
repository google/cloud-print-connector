// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/google/cloud-print-connector/lib"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var windowsCommands = []cli.Command{
	cli.Command{
		Name:      "init",
		ShortName: "i",
		Usage:     "Create a config file",
		Action:    initConfigFile,
		Flags:     commonInitFlags,
	},
	cli.Command{
		Name:   "install-event-log",
		Usage:  "Install registry entries for the event log",
		Action: installEventLog,
	},
	cli.Command{
		Name:   "remove-event-log",
		Usage:  "Remove registry entries for the event log",
		Action: removeEventLog,
	},
	cli.Command{
		Name:   "create-service",
		Usage:  "Create a service in the local service control manager",
		Action: createService,
	},
	cli.Command{
		Name:   "delete-service",
		Usage:  "Delete an existing service in the local service control manager",
		Action: deleteService,
	},
	cli.Command{
		Name:   "start-service",
		Usage:  "Start the service in the local service control manager",
		Action: startService,
	},
	cli.Command{
		Name:   "stop-service",
		Usage:  "Stop the service in the local service control manager",
		Action: stopService,
	},
}

func installEventLog(c *cli.Context) {
	err := eventlog.InstallAsEventCreate(lib.ConnectorName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		fmt.Printf("Failed to install event log registry entries: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("Event log registry entries installed successfully")
}

func removeEventLog(c *cli.Context) {
	err := eventlog.Remove(lib.ConnectorName)
	if err != nil {
		fmt.Printf("Failed to remove event log registry entries: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("Event log registry entries removed successfully")
}

func createService(c *cli.Context) {
	exePath, err := filepath.Abs("gcp-windows-connector.exe")
	if err != nil {
		fmt.Printf("Failed to find the connector executable: %s\n", err)
		os.Exit(1)
	}

	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Failed to connect to service control manager: %s\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	config := mgr.Config{
		DisplayName:  lib.ConnectorName,
		Description:  "Shares printers with Google Cloud Print",
		Dependencies: []string{"Spooler"},
		StartType:    mgr.StartAutomatic,
	}
	service, err := m.CreateService(lib.ConnectorName, exePath, config)
	if err != nil {
		fmt.Printf("Failed to create service: %s\n", err)
		os.Exit(1)
	}
	defer service.Close()

	fmt.Println("Service created successfully")
}

func deleteService(c *cli.Context) {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Failed to connect to service control manager: %s\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	service, err := m.OpenService(lib.ConnectorName)
	if err != nil {
		fmt.Printf("Failed to open service: %s\n", err)
		os.Exit(1)
	}
	defer service.Close()

	err = service.Delete()
	if err != nil {
		fmt.Printf("Failed to delete service: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Service deleted successfully")
}

func startService(c *cli.Context) {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Failed to connect to service control manager: %s\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	service, err := m.OpenService(lib.ConnectorName)
	if err != nil {
		fmt.Printf("Failed to open service: %s\n", err)
		os.Exit(1)
	}
	defer service.Close()

	err = service.Start()
	if err != nil {
		fmt.Printf("Failed to start service: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Service started successfully")
}

func stopService(c *cli.Context) {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Failed to connect to service control manager: %s\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	service, err := m.OpenService(lib.ConnectorName)
	if err != nil {
		fmt.Printf("Failed to open service: %s\n", err)
		os.Exit(1)
	}
	defer service.Close()

	_, err = service.Control(svc.Stop)
	if err != nil {
		fmt.Printf("Failed to stop service: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service stopped successfully")
}

func main() {
	// Suppress date/time prefix.
	log.SetFlags(0)

	app := cli.NewApp()
	app.Name = "gcp-windows-connector-util"
	app.Usage = lib.ConnectorName + " for Windows utility tools"
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
		GCPBaseURL:                lib.DefaultConfig.GCPBaseURL,
		GCPOAuthClientID:          context.String("gcp-oauth-client-id"),
		GCPOAuthClientSecret:      context.String("gcp-oauth-client-secret"),
		GCPOAuthAuthURL:           lib.DefaultConfig.GCPOAuthAuthURL,
		GCPOAuthTokenURL:          lib.DefaultConfig.GCPOAuthTokenURL,
		GCPMaxConcurrentDownloads: uint(context.Int("gcp-max-concurrent-downloads")),

		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		CUPSJobFullUsername:       lib.PointerToBool(context.Bool("cups-job-full-username")),
		PrefixJobIDToJobTitle:     lib.PointerToBool(context.Bool("prefix-job-id-to-job-title")),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		LogLevel:                  context.String("log-level"),

		LocalPortLow:  uint16(context.Int("local-port-low")),
		LocalPortHigh: uint16(context.Int("local-port-high")),
	}
}

// createLocalConfig creates a config object that supports local mode.
func createLocalConfig(context *cli.Context) *lib.Config {
	return &lib.Config{
		LocalPrintingEnable: true,
		CloudPrintingEnable: false,

		NativeJobQueueSize:        uint(context.Int("native-job-queue-size")),
		NativePrinterPollInterval: context.String("native-printer-poll-interval"),
		CUPSJobFullUsername:       lib.PointerToBool(context.Bool("cups-job-full-username")),
		PrefixJobIDToJobTitle:     lib.PointerToBool(context.Bool("prefix-job-id-to-job-title")),
		DisplayNamePrefix:         context.String("display-name-prefix"),
		PrinterBlacklist:          lib.DefaultConfig.PrinterBlacklist,
		LogLevel:                  context.String("log-level"),

		LocalPortLow:  uint16(context.Int("local-port-low")),
		LocalPortHigh: uint16(context.Int("local-port-high")),
	}
}
