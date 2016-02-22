// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/log"
	"github.com/google/cups-connector/manager"
	"github.com/google/cups-connector/winspool"
	"github.com/google/cups-connector/xmpp"
	"golang.org/x/sys/windows/svc"
)

// TODO: Windows Service.
// TODO: Event Log.

func main() {
	app := cli.NewApp()
	app.Name = "gcp-windows-connector"
	app.Usage = "Google Cloud Print Connector for Windows"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
	}
	app.Action = func(context *cli.Context) {
		os.Exit(connector(context))
	}
	app.RunAndExitOnError()
}

func connector(context *cli.Context) int {
	if interactive, err := svc.IsAnInteractiveSession(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to detect interactive session: %s\n", err)
		return 1

	} else if interactive {
		err = log.Start(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start event log: %s\n", err)
			return 1
		}

	} else {
		err = log.Start(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start event log: %s\n", err)
			return 1
		}
	}
	defer log.Stop()

	config, configFilename, err := lib.GetConfig(context)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %s\n", err)
		return 1
	}

	logLevel, ok := log.LevelFromString(config.LogLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Log level %s is not recognized\n", config.LogLevel)
		return 1
	}
	log.SetLevel(logLevel)

	if configFilename == "" {
		log.Info("No config file was found, so using defaults")
	}

	log.Info(lib.FullName)

	if !config.CloudPrintingEnable && !config.LocalPrintingEnable {
		log.Fatal("Cannot run connector with both local_printing_enable and cloud_printing_enable set to false")
		return 1
	} else if config.LocalPrintingEnable {
		log.Fatal("Local printing has not been implemented in this version of the Windows connector.")
		return 1
	}

	jobs := make(chan *lib.Job, 10)
	xmppNotifications := make(chan xmpp.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	if config.CloudPrintingEnable {
		xmppPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
		if err != nil {
			log.Fatalf("Failed to parse xmpp ping timeout: %s", err)
			return 1
		}
		xmppPingInterval, err := time.ParseDuration(config.XMPPPingInterval)
		if err != nil {
			log.Fatalf("Failed to parse xmpp ping interval default: %s", err)
			return 1
		}

		g, err = gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
			config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
			config.GCPMaxConcurrentDownloads, jobs)
		if err != nil {
			log.Fatal(err)
			return 1
		}

		x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
			xmppPingTimeout, xmppPingInterval, g.GetRobotAccessToken, xmppNotifications)
		if err != nil {
			log.Fatal(err)
			return 1
		}
		defer x.Quit()
	}

	ws, err := winspool.NewWinSpool(config.PrefixJobIDToJobTitle, config.DisplayNamePrefix, config.PrinterBlacklist)
	if err != nil {
		log.Fatal(err)
		return 1
	}

	nativePrinterPollInterval, err := time.ParseDuration(config.NativePrinterPollInterval)
	if err != nil {
		log.Fatalf("Failed to parse printer poll interval: %s", err)
		return 1
	}
	pm, err := manager.NewPrinterManager(ws, g, nil, nil, nativePrinterPollInterval,
		config.NativeJobQueueSize, false, false, config.ShareScope, jobs, xmppNotifications)
	if err != nil {
		log.Fatal(err)
		return 1
	}
	defer pm.Quit()

	if config.CloudPrintingEnable {
		if config.LocalPrintingEnable {
			log.Infof("Ready to rock as proxy '%s' and in local mode", config.ProxyName)
		} else {
			log.Infof("Ready to rock as proxy '%s'", config.ProxyName)
		}
	} else {
		log.Info("Ready to rock in local-only mode")
	}

	waitIndefinitely()

	log.Info("Shutting down")

	return 0
}

// Blocks until Ctrl-C or SIGTERM.
func waitIndefinitely() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	go func() {
		// In case the process doesn't die quickly, wait for a second termination request.
		<-ch
		log.Info("Second termination request received")
		os.Exit(1)
	}()
}
