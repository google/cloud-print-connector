/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package monitor

import (
	"fmt"
	"net"

	"github.com/google/cups-connector/cups"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/log"
	"github.com/google/cups-connector/manager"
	"github.com/google/cups-connector/privet"
)

const monitorFormat = `cups-printers=%d
cups-raw-printers=%d
gcp-printers=%d
local-printers=%d
cups-conn-qty=%d
cups-conn-max-qty=%d
jobs-done=%d
jobs-error=%d
jobs-in-progress=%d
`

type Monitor struct {
	cups         *cups.CUPS
	gcp          *gcp.GoogleCloudPrint
	p            *privet.Privet
	pm           *manager.PrinterManager
	listenerQuit chan bool
}

func NewMonitor(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, p *privet.Privet, pm *manager.PrinterManager, socketFilename string) (*Monitor, error) {
	m := Monitor{cups, gcp, p, pm, make(chan bool)}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{socketFilename, "unix"})
	if err != nil {
		return nil, err
	}

	go m.listen(listener)

	return &m, nil
}

func (m *Monitor) listen(listener net.Listener) {
	ch := make(chan net.Conn)
	quitReq := make(chan bool, 1)
	quitAck := make(chan bool)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-quitReq:
					quitAck <- true
					return
				}
				log.Errorf("Error listening to monitor socket: %s", err)
			} else {
				ch <- conn
			}
		}
	}()

	for {
		select {
		case conn := <-ch:
			log.Info("Received monitor request")
			stats, err := m.getStats()
			if err != nil {
				log.Warningf("Monitor request failed: %s", err)
				conn.Write([]byte("error"))
			} else {
				conn.Write([]byte(stats))
			}
			conn.Close()

		case <-m.listenerQuit:
			quitReq <- true
			listener.Close()
			<-quitAck
			m.listenerQuit <- true
			return
		}
	}
}

func (m *Monitor) Quit() {
	m.listenerQuit <- true
	<-m.listenerQuit
}

func (m *Monitor) getStats() (string, error) {
	var cupsPrinterQuantity, rawPrinterQuantity, gcpPrinterQuantity, privetPrinterQuantity int

	if cupsPrinters, err := m.cups.GetPrinters(); err != nil {
		return "", err
	} else {
		cupsPrinterQuantity = len(cupsPrinters)
		_, rawPrinters := lib.FilterRawPrinters(cupsPrinters)
		rawPrinterQuantity = len(rawPrinters)
	}

	cupsConnOpen := m.cups.ConnQtyOpen()
	cupsConnMax := m.cups.ConnQtyMax()

	if m.gcp != nil {
		if gcpPrinters, err := m.gcp.List(); err != nil {
			return "", err
		} else {
			gcpPrinterQuantity = len(gcpPrinters)
		}
	}

	if m.p != nil {
		privetPrinterQuantity = m.p.Size()
	}

	jobsDone, jobsError, jobsProcessing, err := m.pm.GetJobStats()
	if err != nil {
		return "", err
	}

	stats := fmt.Sprintf(
		monitorFormat,
		cupsPrinterQuantity, rawPrinterQuantity, gcpPrinterQuantity, privetPrinterQuantity,
		cupsConnOpen, cupsConnMax,
		jobsDone, jobsError, jobsProcessing)

	return stats, nil
}
