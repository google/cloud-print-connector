// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
	"github.com/google/cloud-print-connector/manager"
	"github.com/google/cloud-print-connector/winspool"
	"github.com/google/cloud-print-connector/xmpp"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

func main() {
	app := cli.NewApp()
	app.Name = "gcp-windows-connector"
	app.Usage = "Google Cloud Print Connector for Windows"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
	}
	app.Action = RunService
	app.Run(os.Args)
}

var (
	runningStatus = svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop,
	}
	stoppingStatus = svc.Status{
		State:   svc.StopPending,
		Accepts: svc.AcceptStop,
	}
)

type service struct {
	context     *cli.Context
	interactive bool
}

func RunService(context *cli.Context) error {
	interactive, err := svc.IsAnInteractiveSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to detect interactive session: %s\n", err)
		os.Exit(1)
	}

	s := service{context, interactive}

	if interactive {
		debug.Run(lib.ConnectorName, &s)
	} else {
		svc.Run(lib.ConnectorName, &s)
	}
        return nil
}

func (service *service) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	if service.interactive {
		if err := log.Start(true); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start event log: %s\n", err)
			return false, 1
		}
	} else {
		if err := log.Start(false); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start event log: %s\n", err)
			return false, 1
		}
	}
	defer log.Stop()

	config, configFilename, err := lib.GetConfig(service.context)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %s\n", err)
		return false, 1
	}

	logLevel, ok := log.LevelFromString(config.LogLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Log level %s is not recognized\n", config.LogLevel)
		return false, 1
	}
	log.SetLevel(logLevel)

	if configFilename == "" {
		log.Info("No config file was found, so using defaults")
	} else {
		log.Infof("Using config file %s", configFilename)
	}
	completeConfig, _ := json.MarshalIndent(config, "", " ")
	log.Debugf("Config: %s", string(completeConfig))

	log.Info(lib.FullName)

	if !config.CloudPrintingEnable && !config.LocalPrintingEnable {
		log.Fatal("Cannot run connector with both local_printing_enable and cloud_printing_enable set to false")
		return false, 1
	} else if config.LocalPrintingEnable {
		log.Fatal("Local printing has not been implemented in this version of the Windows connector.")
		return false, 1
	}

	jobs := make(chan *lib.Job, 10)
	xmppNotifications := make(chan xmpp.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	if config.CloudPrintingEnable {
		xmppPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
		if err != nil {
			log.Fatalf("Failed to parse xmpp ping timeout: %s", err)
			return false, 1
		}
		xmppPingInterval, err := time.ParseDuration(config.XMPPPingInterval)
		if err != nil {
			log.Fatalf("Failed to parse xmpp ping interval default: %s", err)
			return false, 1
		}

		g, err = gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
			config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
			config.GCPMaxConcurrentDownloads, jobs)
		if err != nil {
			log.Fatal(err)
			return false, 1
		}

		x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
			xmppPingTimeout, xmppPingInterval, g.GetRobotAccessToken, xmppNotifications)
		if err != nil {
			log.Fatal(err)
			return false, 1
		}
		defer x.Quit()
	}

	ws, err := winspool.NewWinSpool(*config.PrefixJobIDToJobTitle, config.DisplayNamePrefix, config.PrinterBlacklist, config.PrinterWhitelist)
	if err != nil {
		log.Fatal(err)
		return false, 1
	}

	nativePrinterPollInterval, err := time.ParseDuration(config.NativePrinterPollInterval)
	if err != nil {
		log.Fatalf("Failed to parse printer poll interval: %s", err)
		return false, 1
	}
	pm, err := manager.NewPrinterManager(ws, g, nil, nativePrinterPollInterval,
		config.NativeJobQueueSize, *config.CUPSJobFullUsername, config.ShareScope, jobs, xmppNotifications)
	if err != nil {
		log.Fatal(err)
		return false, 1
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

	s <- runningStatus
	for {
		request := <-r
		switch request.Cmd {
		case svc.Interrogate:
			s <- runningStatus

		case svc.Stop:
			s <- stoppingStatus
			log.Info("Shutting down")
			time.AfterFunc(time.Second*30, func() {
				log.Fatal("Failed to stop quickly; stopping forcefully")
				os.Exit(1)
			})

			return false, 0

		default:
			log.Errorf("Received unsupported service command from service control manager: %d", request.Cmd)
		}
	}
}
