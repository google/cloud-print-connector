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
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		log.Fatal(err)
	}

	cups, err := cups.NewCUPS(config.CopyPrinterInfoToDisplayName)
	if err != nil {
		log.Fatal(err)
	}

	gcp, err := gcp.NewGoogleCloudPrint(config.RefreshToken, config.XMPPJID, config.Proxy)
	if err != nil {
		log.Fatal(err)
	}

	pm, err := manager.NewPrinterManager(cups, gcp, config.CUPSPollIntervalPrinter, config.CUPSPollIntervalJob)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Google Cloud Print CUPS Connector ready to rock as proxy '%s'\n", config.Proxy)

	waitIndefinitely()

	fmt.Println("")
	fmt.Println("shutting down normally")

	pm.Quit()
	cups.Quit()
}

// Blocks until Ctrl-C or SIGTERM.
func waitIndefinitely() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
