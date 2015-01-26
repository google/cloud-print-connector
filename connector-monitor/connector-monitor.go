/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"cups-connector/lib"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(config.MonitorSocketFilename); err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		log.Fatal(fmt.Errorf(
			"No connector is running, or the monitoring socket %s is mis-configured",
			config.MonitorSocketFilename))
	}

	timer := time.AfterFunc(time.Second*3, func() {
		log.Fatal("timeout")
		return
	})

	conn, err := net.DialTimeout("unix", config.MonitorSocketFilename, time.Second)
	if err != nil {
		log.Fatal(fmt.Errorf(
			"No connector is running, or it is not listening to socket %s",
			config.MonitorSocketFilename))
	}
	defer conn.Close()

	buf, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Fatal(err)
	}

	timer.Stop()

	fmt.Printf(string(buf))
}
