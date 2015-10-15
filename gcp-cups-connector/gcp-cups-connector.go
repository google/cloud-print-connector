/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/golang/glog"
	"github.com/google/cups-connector/cups"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/log"
	"github.com/google/cups-connector/manager"
	"github.com/google/cups-connector/monitor"
	"github.com/google/cups-connector/privet"
	"github.com/google/cups-connector/snmp"
	"github.com/google/cups-connector/xmpp"
)

var logToConsoleFlag = flag.Bool("log-to-console", false, "Log to STDERR, in addition to file")

func main() {
	os.Exit(connector())
}

func connector() int {
	flag.Parse()

	config, configFilename, err := lib.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %s", err)
		return 1
	}

	var logWriter io.Writer = &lumberjack.Logger{
		Filename:   config.LogFileName,
		MaxSize:    int(config.LogFileMaxMegabytes),
		MaxBackups: int(config.LogMaxFiles),
		LocalTime:  true,
	}
	if *logToConsoleFlag {
		logWriter = io.MultiWriter(logWriter, os.Stderr)
	}
	logLevel, ok := log.LevelFromString(config.LogLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Log level %s is not recognized", config.LogLevel)
		return 1
	}
	log.SetLevel(logLevel)
	log.SetWriter(logWriter)

	if configFilename == "" {
		log.Info("No config file was found, so using defaults")
	}

	log.Error(lib.FullName)
	fmt.Println(lib.FullName)

	if !config.CloudPrintingEnable && !config.LocalPrintingEnable {
		log.Error("Cannot run connector with both local_printing_enable and cloud_printing_enable set to false")
		return 1
	}

	if _, err := os.Stat(config.MonitorSocketFilename); !os.IsNotExist(err) {
		if err != nil {
			log.Errorf("Failed to stat monitor socket: %s", err)
		} else {
			log.Errorf(
				"A connector is already running, or the monitoring socket %s wasn't cleaned up properly",
				config.MonitorSocketFilename)
		}
		return 1
	}

	cupsConnectTimeout, err := time.ParseDuration(config.CUPSConnectTimeout)
	if err != nil {
		log.Errorf("Failed to parse cups connect timeout: %s", err)
		return 1
	}

	jobs := make(chan *lib.Job, 10)
	xmppNotifications := make(chan xmpp.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	if config.CloudPrintingEnable {
		gcpXMPPPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
		if err != nil {
			glog.Fatalf("Failed to parse xmpp ping timeout: %s", err)
		}
		gcpXMPPPingIntervalDefault, err := time.ParseDuration(config.XMPPPingIntervalDefault)
		if err != nil {
			glog.Fatalf("Failed to parse xmpp ping interval default: %s", err)
		}

		g, err = gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
			config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
			config.GCPMaxConcurrentDownloads, jobs)
		if err != nil {
			log.Error(err)
			return 1
		}

		x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
			gcpXMPPPingTimeout, gcpXMPPPingIntervalDefault, g.GetRobotAccessToken, xmppNotifications)
		if err != nil {
			log.Error(err)
			return 1
		}
		defer x.Quit()
	}

	c, err := cups.NewCUPS(config.CopyPrinterInfoToDisplayName, config.PrefixJobIDToJobTitle,
		config.DisplayNamePrefix, config.CUPSPrinterAttributes, config.CUPSMaxConnections,
		cupsConnectTimeout)
	if err != nil {
		log.Error(err)
		return 1
	}
	defer c.Quit()

	var s *snmp.SNMPManager
	if config.SNMPEnable {
		log.Info("SNMP enabled")
		s, err = snmp.NewSNMPManager(config.SNMPCommunity, config.SNMPMaxConnections)
		if err != nil {
			log.Error(err)
			return 1
		}
		defer s.Quit()
	}

	var priv *privet.Privet
	if config.LocalPrintingEnable {
		if g == nil {
			priv, err = privet.NewPrivet(jobs, config.GCPBaseURL, nil)
		} else {
			priv, err = privet.NewPrivet(jobs, config.GCPBaseURL, g.ProximityToken)
		}
		if err != nil {
			log.Error(err)
			return 1
		}
		defer priv.Quit()
	}

	pm, err := manager.NewPrinterManager(c, g, priv, s, config.CUPSPrinterPollInterval,
		config.CUPSJobQueueSize, config.CUPSJobFullUsername, config.CUPSIgnoreRawPrinters,
		config.ShareScope, jobs, xmppNotifications)
	if err != nil {
		log.Error(err)
		return 1
	}
	defer pm.Quit()

	m, err := monitor.NewMonitor(c, g, priv, pm, config.MonitorSocketFilename)
	if err != nil {
		log.Error(err)
		return 1
	}
	defer m.Quit()

	if config.CloudPrintingEnable {
		if config.LocalPrintingEnable {
			log.Errorf("Ready to rock as proxy '%s' and in local mode", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s' and in local mode\n", config.ProxyName)
		} else {
			log.Errorf("Ready to rock as proxy '%s'", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s'\n", config.ProxyName)
		}
	} else {
		log.Error("Ready to rock in local-only mode")
		fmt.Println("Ready to rock in local-only mode")
	}

	waitIndefinitely()

	log.Error("Shutting down")
	fmt.Println("")
	fmt.Println("Shutting down")

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
		fmt.Println("Second termination request received")
		os.Exit(1)
	}()
}
