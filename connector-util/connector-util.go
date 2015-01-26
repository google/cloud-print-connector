/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"cups-connector/gcp"
	"cups-connector/lib"
	"flag"
	"fmt"

	"github.com/golang/glog"
)

var (
	deleteAllGCPPrinters = flag.Bool(
		"delete-all-gcp-printers", false,
		"Delete all printers associated with this connector")
)

func main() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		fmt.Println(err)
		return
	}

	if *deleteAllGCPPrinters {
		gcp, err := gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.XMPPJID, config.RobotRefreshToken,
			config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID, config.GCPOAuthClientSecret,
			config.GCPOAuthAuthURL, config.GCPOAuthTokenURL, config.XMPPServer, config.XMPPPort)
		if err != nil {
			glog.Fatal(err)
		}

		printers, _, err := gcp.List()
		if err != nil {
			glog.Fatal(err)
		}

		ch := make(chan bool)
		for _, p := range printers {
			go func(gcpID, name string) {
				err := gcp.Delete(gcpID)
				if err != nil {
					fmt.Printf("Failed to delete %s \"%s\": %s\n", gcpID, name, err)
				} else {
					fmt.Printf("Deleted %s \"%s\" from GCP\n", gcpID, name)
				}
				ch <- true
			}(p.GCPID, p.Name)
		}

		for _ = range printers {
			<-ch
		}
	}
}
