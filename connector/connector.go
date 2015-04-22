/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"cups-connector/cups"
	"cups-connector/gcp"
	"cups-connector/lib"
	"cups-connector/manager"
	"cups-connector/monitor"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	defer glog.Flush()
	glog.Error(lib.FullName)
	fmt.Println(lib.FullName)

	config, err := lib.ConfigFromFile()
	if err != nil {
		glog.Fatal(err)
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

	cups, err := cups.NewCUPS(config.CopyPrinterInfoToDisplayName, config.CUPSPrinterAttributes,
		config.CUPSMaxConnections, cupsConnectTimeout)
	if err != nil {
		glog.Fatal(err)
	}
	defer cups.Quit()

	gcpXMPPPingTimeout, err := time.ParseDuration(config.XMPPPingTimeout)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping timeout: %s", err)
	}
	gcpXMPPPingIntervalDefault, err := time.ParseDuration(config.XMPPPingIntervalDefault)
	if err != nil {
		glog.Fatalf("Failed to parse xmpp ping interval default: %s", err)
	}

	gcp, err := gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.XMPPJID, config.RobotRefreshToken,
		config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID, config.GCPOAuthClientSecret,
		config.GCPOAuthAuthURL, config.GCPOAuthTokenURL, config.XMPPServer, config.XMPPPort,
		gcpXMPPPingTimeout, gcpXMPPPingIntervalDefault)
	if err != nil {
		glog.Fatal(err)
	}
	defer gcp.Quit()

	if err := gcp.StartXMPP(); err != nil {
		glog.Fatal(err)
	}

	pm, err := manager.NewPrinterManager(cups, gcp, config.CUPSPrinterPollInterval,
		config.GCPMaxConcurrentDownloads, config.CUPSJobQueueSize, config.CUPSJobFullUsername,
		config.CUPSIgnoreRawPrinters, config.ShareScope)
	if err != nil {
		glog.Fatal(err)
	}
	defer pm.Quit()

	m, err := monitor.NewMonitor(cups, gcp, pm, config.MonitorSocketFilename)
	if err != nil {
		glog.Fatal(err)
	}
	defer m.Quit()

	glog.Errorf("Ready to rock as proxy '%s'\n", config.ProxyName)
	fmt.Printf("Ready to rock as proxy '%s'\n", config.ProxyName)

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
