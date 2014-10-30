package main

import (
	"cups-connector/lib"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"time"
)

func main() {
	config, err := lib.ConfigFromFile()
	if err != nil {
		log.Fatal(err)
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
