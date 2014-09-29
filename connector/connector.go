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

	cups := cups.NewCUPSDefault()

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
