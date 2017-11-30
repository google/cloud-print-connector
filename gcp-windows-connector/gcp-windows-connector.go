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

	"github.com/google/cloud-print-connector/fcm"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
	"github.com/google/cloud-print-connector/manager"
	"github.com/google/cloud-print-connector/notification"
	"github.com/google/cloud-print-connector/winspool"
	"github.com/google/cloud-print-connector/xmpp"
	"github.com/urfave/cli"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

func main() {
	app := cli.NewApp()
	app.Name = "gcp-windows-connector"
	app.Usage = lib.ConnectorName + " for Windows"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
	}
	app.Action = runService
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

func runService(context *cli.Context) error {
	interactive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to detect interactive session: %s", err), 1)
	}

	s := service{context, interactive}

	if interactive {
		err = debug.Run(lib.ConnectorName, &s)
	} else {
		err = svc.Run(lib.ConnectorName, &s)
	}
	if err != nil {
		err = cli.NewExitError(err.Error(), 1)
	}
	return err
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
	notifications := make(chan notification.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	var f *fcm.FCM
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
			config.GCPMaxConcurrentDownloads, jobs, config.FcmNotificationsEnable)
		if err != nil {
			log.Fatal(err)
			return false, 1
		}
		if useFcm {
			f, err = fcm.NewFCM(config.GCPOAuthClientID, config.ProxyName, config.FcmServerBindUrl, g.FcmSubscribe, notifications)
			if err != nil {
				log.Fatal(err)
				return false, 1
			}
			defer f.Quit()
		} else {
			x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
				xmppPingTimeout, xmppPingInterval, g.GetRobotAccessToken, notifications)
			if err != nil {
				log.Fatal(err)
				return false, 1
			}
			defer x.Quit()
		}
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
		config.NativeJobQueueSize, *config.CUPSJobFullUsername, config.ShareScope, jobs, notifications,
		useFcm)
	if err != nil {
		log.Fatal(err)
		return false, 1
	}
	defer pm.Quit()

	// Init FCM client after printers are registered
	if useFcm && config.CloudPrintingEnable {
		f.Init()
	}
	statusHandle := svc.StatusHandle()
	if statusHandle != 0 {
		err = ws.StartPrinterNotifications(statusHandle)
		if err != nil {
			log.Error(err)
		} else {
			log.Info("Successfully registered for device notifications.")
		}
	}

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

		case svc.DeviceEvent:
			log.Infof("Printers change notification received %d.", request.EventType)
			// Delay the action to let the OS finish the process or we might
			// not see the new printer. Even if we miss it eventually the timed updates
			// will pick it up.
			time.AfterFunc(time.Second*5, func() {
				pm.SyncPrinters(false)
			})

		default:
			log.Errorf("Received unsupported service command from service control manager: %d", request.Cmd)
		}
	}
}
