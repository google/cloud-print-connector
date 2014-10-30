/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"cups-connector/cups"
	"cups-connector/gcp"
	"cups-connector/lib"
	"cups-connector/manager"
	"cups-connector/monitor"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

func main() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		glog.Fatal(err)
	}

	if _, err := os.Stat(config.SocketFilename); !os.IsNotExist(err) {
		if err != nil {
			glog.Fatal(err)
		}
		glog.Errorf(
			"A connector is already running, or the monitoring socket %s wasn't cleaned up properly",
			config.SocketFilename)
	}

	cups, err := cups.NewCUPS(config.CopyPrinterInfoToDisplayName, config.CUPSPrinterAttributes)
	if err != nil {
		glog.Fatal(err)
	}

	gcp, err := gcp.NewGoogleCloudPrint(config.XMPPJID, config.RobotRefreshToken, config.UserRefreshToken, config.ShareScope, config.Proxy)
	if err != nil {
		glog.Fatal(err)
	}

	pm, err := manager.NewPrinterManager(cups, gcp, config.CUPSPollIntervalPrinter,
		config.GCPMaxConcurrentDownloads, config.CUPSQueueSize, config.CUPSJobFullUsername)
	if err != nil {
		glog.Fatal(err)
	}

	m, err := monitor.NewMonitor(cups, gcp, pm, config.SocketFilename)
	if err != nil {
		glog.Fatal(err)
	}

	fmt.Printf("Google Cloud Print CUPS Connector ready to rock as proxy '%s'\n", config.Proxy)

	waitIndefinitely()

	fmt.Println("")
	fmt.Println("shutting down normally")

	m.Quit()
	pm.Quit()
	cups.Quit()
	glog.Flush()
}

// Blocks until Ctrl-C or SIGTERM.
func waitIndefinitely() {
	// TODO(jacobmarble): Second signal forces quit.
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
