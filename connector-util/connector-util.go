/*
Copyright 2015 Google Inc. All rights reserved.

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
