// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin freebsd

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/journal"
	"github.com/google/cloud-print-connector/cups"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
	"github.com/google/cloud-print-connector/manager"
	"github.com/google/cloud-print-connector/monitor"
	"github.com/google/cloud-print-connector/privet"
	"github.com/google/cloud-print-connector/xmpp"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "gcp-cups-connector"
	app.Usage = lib.ConnectorName + " for CUPS"
	app.Version = lib.BuildDate
	app.Flags = []cli.Flag{
		lib.ConfigFilenameFlag,
		cli.BoolFlag{
			Name:  "log-to-console",
			Usage: "Log to STDERR, in addition to configured logging",
		},
	}
	app.Action = connector
	app.Run(os.Args)
}

func connector(context *cli.Context) error {
	config, configFilename, err := lib.GetConfig(context)
	if err != nil {
		return fmt.Errorf("Failed to read config file: %s", err)
	}

	logToJournal := *config.LogToJournal && journal.Enabled()
	logToConsole := context.Bool("log-to-console")

	if logToJournal {
		log.SetJournalEnabled(true)
		if logToConsole {
			log.SetWriter(os.Stderr)
		} else {
			log.SetWriter(ioutil.Discard)
		}
	} else {
		logFileMaxBytes := config.LogFileMaxMegabytes * 1024 * 1024
		var logWriter io.Writer
		logWriter, err = log.NewLogRoller(config.LogFileName, logFileMaxBytes, config.LogMaxFiles)
		if err != nil {
			return fmt.Errorf("Failed to start log roller: %s", err)
		}

		if logToConsole {
			logWriter = io.MultiWriter(logWriter, os.Stderr)
		}
		log.SetWriter(logWriter)
	}

	logLevel, ok := log.LevelFromString(config.LogLevel)
	if !ok {
		return fmt.Errorf("Log level %s is not recognized", config.LogLevel)
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
	fmt.Println(lib.FullName)

	if !config.CloudPrintingEnable && !config.LocalPrintingEnable {
		errStr := "Cannot run connector with both local_printing_enable and cloud_printing_enable set to false"
		log.Fatal(errStr)
		return errors.New(errStr)
	}

	if _, err := os.Stat(config.MonitorSocketFilename); !os.IsNotExist(err) {
		var errStr string
		if err != nil {
			errStr = fmt.Sprintf("Failed to stat monitor socket: %s", err)
		} else {
			errStr = fmt.Sprintf(
				"A connector is already running, or the monitoring socket %s wasn't cleaned up properly",
				config.MonitorSocketFilename)
		}
		log.Fatal(errStr)
		return errors.New(errStr)
	}

	jobs := make(chan *lib.Job, 10)
	xmppNotifications := make(chan xmpp.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	if config.CloudPrintingEnable {
		xmppPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
		if err != nil {
			errStr := fmt.Sprintf("Failed to parse xmpp ping timeout: %s", err)
			log.Fatal(errStr)
			return errors.New(errStr)
		}
		xmppPingInterval, err := time.ParseDuration(config.XMPPPingInterval)
		if err != nil {
			errStr := fmt.Sprintf("Failed to parse xmpp ping interval default: %s", err)
			log.Fatalf(errStr)
			return errors.New(errStr)
		}

		g, err = gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
			config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
			config.GCPMaxConcurrentDownloads, jobs)
		if err != nil {
			log.Fatal(err)
			return err
		}

		x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
			xmppPingTimeout, xmppPingInterval, g.GetRobotAccessToken, xmppNotifications)
		if err != nil {
			log.Fatal(err)
			return err
		}
		defer x.Quit()
	}

	cupsConnectTimeout, err := time.ParseDuration(config.CUPSConnectTimeout)
	if err != nil {
		errStr := fmt.Sprintf("Failed to parse CUPS connect timeout: %s", err)
		log.Fatalf(errStr)
		return errors.New(errStr)
	}
	c, err := cups.NewCUPS(*config.CUPSCopyPrinterInfoToDisplayName, *config.PrefixJobIDToJobTitle,
		config.DisplayNamePrefix, config.CUPSPrinterAttributes, config.CUPSVendorPPDOptions, config.CUPSMaxConnections,
		cupsConnectTimeout, config.PrinterBlacklist, config.PrinterWhitelist, *config.CUPSIgnoreRawPrinters,
		*config.CUPSIgnoreClassPrinters)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer c.Quit()

	var priv *privet.Privet
	if config.LocalPrintingEnable {
		if g == nil {
			priv, err = privet.NewPrivet(jobs, config.LocalPortLow, config.LocalPortHigh, config.GCPBaseURL, nil)
		} else {
			priv, err = privet.NewPrivet(jobs, config.LocalPortLow, config.LocalPortHigh, config.GCPBaseURL, g.ProximityToken)
		}
		if err != nil {
			log.Fatal(err)
			return err
		}
		defer priv.Quit()
	}

	nativePrinterPollInterval, err := time.ParseDuration(config.NativePrinterPollInterval)
	if err != nil {
		errStr := fmt.Sprintf("Failed to parse CUPS printer poll interval: %s", err)
		log.Fatal(errStr)
		return errors.New(errStr)
	}
	pm, err := manager.NewPrinterManager(c, g, priv, nativePrinterPollInterval,
		config.NativeJobQueueSize, *config.CUPSJobFullUsername, config.ShareScope,
		jobs, xmppNotifications)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer pm.Quit()

	m, err := monitor.NewMonitor(c, g, priv, pm, config.MonitorSocketFilename)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer m.Quit()

	if config.CloudPrintingEnable {
		if config.LocalPrintingEnable {
			log.Infof("Ready to rock as proxy '%s' and in local mode", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s' and in local mode\n", config.ProxyName)
		} else {
			log.Infof("Ready to rock as proxy '%s'", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s'\n", config.ProxyName)
		}
	} else {
		log.Info("Ready to rock in local-only mode")
		fmt.Println("Ready to rock in local-only mode")
	}

	waitIndefinitely()

	log.Info("Shutting down")
	fmt.Println("")
	fmt.Println("Shutting down")

	return nil
}

// Blocks until Ctrl-C or SIGTERM.
func waitIndefinitely() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	go func() {
		// In case the process doesn't die quickly, wait for a second termination request.
		<-ch
		fmt.Println("Second termination request received")
		os.Exit(1)
	}()
}
