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
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/google/cups-connector/lib"
)

var monitorTimeoutFlag = flag.Duration(
	"monitor-timeout", time.Second*10,
	"wait for a monitor response for this long")

func monitorConnector() {
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

	if _, err := os.Stat(config.MonitorSocketFilename); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		panic(fmt.Sprintf(
			"No connector is running, or the monitoring socket %s is mis-configured",
			config.MonitorSocketFilename))
	}

	timer := time.AfterFunc(*monitorTimeoutFlag, func() {
		panic(fmt.Sprintf("timeout after %s", monitorTimeoutFlag.String()))
	})

	conn, err := net.DialTimeout("unix", config.MonitorSocketFilename, time.Second)
	if err != nil {
		panic(fmt.Sprintf(
			"No connector is running, or it is not listening to socket %s",
			config.MonitorSocketFilename))
	}
	defer conn.Close()

	buf, err := ioutil.ReadAll(conn)
	if err != nil {
		panic(err)
	}

	timer.Stop()

	fmt.Printf(string(buf))
}
