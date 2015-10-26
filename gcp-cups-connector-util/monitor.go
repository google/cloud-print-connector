/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/cups-connector/lib"
)

func monitorConnector(context *cli.Context) {
	config, filename, err := lib.GetConfig(context)
	if err != nil {
		panic(fmt.Sprintf("Failed to read config file: %s", err))
	}
	if filename == "" {
		fmt.Fprintln(os.Stderr, "No config file was found, so using defaults")
	}

	if _, err := os.Stat(config.MonitorSocketFilename); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		panic(fmt.Sprintf(
			"No connector is running, or the monitoring socket %s is mis-configured",
			config.MonitorSocketFilename))
	}

	timer := time.AfterFunc(context.Duration("monitor-timeout"), func() {
		panic(fmt.Sprintf("timeout after %s", context.Duration("monitor-timeout").String()))
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
