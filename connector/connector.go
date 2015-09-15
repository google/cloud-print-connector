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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/cups-connector/cups"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/manager"
	"github.com/google/cups-connector/monitor"
	"github.com/google/cups-connector/privet"
	"github.com/google/cups-connector/snmp"
	"github.com/google/cups-connector/xmpp"

	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	defer glog.Flush()
	glog.Error(lib.FullName)
	fmt.Println(lib.FullName)

	var config *lib.Config
	if lib.ConfigFileExists() {
		var err error
		config, err = lib.ConfigFromFile()
		if err != nil {
			glog.Fatal(err)
		}
	} else {
		config = &lib.DefaultConfig
		glog.Info("No config file was found, so using defaults")
	}

	if !config.CloudPrintingEnable && !config.LocalPrintingEnable {
		glog.Fatal("Cannot run connector with both local_printing_enable and cloud_printing_enable set to false")
	}

	if _, err := os.Stat(config.MonitorSocketFilename); !os.IsNotExist(err) {
		if err != nil {
			glog.Fatal(err)
		}
		glog.Fatalf(
			"A connector is already running, or the monitoring socket %s wasn't cleaned up properly",
			config.MonitorSocketFilename)
	}

	cupsConnectTimeout, err := time.ParseDuration(config.CUPSConnectTimeout)
	if err != nil {
		glog.Fatalf("Failed to parse cups connect timeout: %s", err)
	}

	gcpXMPPPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping timeout: %s", err)
	}
	gcpXMPPPingIntervalDefault, err := time.ParseDuration(config.XMPPPingIntervalDefault)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping interval default: %s", err)
	}

	jobs := make(chan *lib.Job, 10)
	xmppNotifications := make(chan xmpp.PrinterNotification, 5)

	var g *gcp.GoogleCloudPrint
	var x *xmpp.XMPP
	if config.CloudPrintingEnable {
		g, err = gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
			config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
			gcpXMPPPingIntervalDefault, config.GCPMaxConcurrentDownloads, jobs)
		if err != nil {
			glog.Fatal(err)
		}

		x, err = xmpp.NewXMPP(config.XMPPJID, config.ProxyName, config.XMPPServer, config.XMPPPort,
			gcpXMPPPingTimeout, gcpXMPPPingIntervalDefault, g.GetRobotAccessToken, xmppNotifications)
		if err != nil {
			glog.Fatal(err)
		}
		defer x.Quit()
	}

	c, err := cups.NewCUPS(config.CopyPrinterInfoToDisplayName, config.CUPSPrinterAttributes,
		config.CUPSMaxConnections, cupsConnectTimeout)
	if err != nil {
		glog.Fatal(err)
	}
	defer c.Quit()

	var s *snmp.SNMPManager
	if config.SNMPEnable {
		glog.Info("SNMP enabled")
		s, err = snmp.NewSNMPManager(config.SNMPCommunity, config.SNMPMaxConnections)
		if err != nil {
			glog.Fatal(err)
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
			glog.Fatal(err)
		}
		defer priv.Quit()
	}

	pm, err := manager.NewPrinterManager(c, g, priv, s, config.CUPSPrinterPollInterval,
		config.CUPSJobQueueSize, config.CUPSJobFullUsername, config.CUPSIgnoreRawPrinters,
		config.ShareScope, jobs, xmppNotifications)
	if err != nil {
		glog.Fatal(err)
	}
	defer pm.Quit()

	m, err := monitor.NewMonitor(c, g, pm, config.MonitorSocketFilename)
	if err != nil {
		glog.Fatal(err)
	}
	defer m.Quit()

	if config.CloudPrintingEnable {
		if config.LocalPrintingEnable {
			glog.Errorf("Ready to rock as proxy '%s' and in local mode", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s' and in local mode\n", config.ProxyName)
		} else {
			glog.Errorf("Ready to rock as proxy '%s'", config.ProxyName)
			fmt.Printf("Ready to rock as proxy '%s'\n", config.ProxyName)
		}
	} else {
		glog.Error("Ready to rock in local-only mode")
		fmt.Println("Ready to rock in local-only mode")
	}

	waitIndefinitely()

	glog.Error("Shutting down")
	fmt.Println("")
	fmt.Println("Shutting down")
}

// Blocks until Ctrl-C or SIGTERM.
func waitIndefinitely() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	go func() {
		// In case the process doesn't die very quickly, wait for a second termination request.
		<-ch
		fmt.Println("Second termination request received")
		os.Exit(1)
	}()
}
