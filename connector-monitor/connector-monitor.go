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

	if _, err := os.Stat(config.SocketFilename); err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		log.Fatal(fmt.Sprintf(
			"No connector is running, or the monitoring socket %s is mis-configured",
			config.SocketFilename))
	}

	conn, err := net.DialTimeout("unix", config.SocketFilename, time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	buf, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(string(buf))
}
