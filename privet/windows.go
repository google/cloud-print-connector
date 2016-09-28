// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package privet

import (
  "fmt"
  "sync"
  "github.com/andrewtj/dnssd"
  "github.com/google/cloud-print-connector/log"
)

const serviceType = "_privet._tcp"

type zeroconf struct {
	printers map[string]dnssd.RegisterOp
	pMutex   sync.RWMutex // Protects printers.
}

func newZeroconf() (*zeroconf, error) {
	z := zeroconf{
		printers: make(map[string]dnssd.RegisterOp),
	}
	return &z, nil
}

func nullDNSSDRegisterCallback(_ *dnssd.RegisterOp, _ error, _ bool, _, _, _ string) {}

func (z *zeroconf) addPrinter(name string, port uint16, ty, note, url, id string, online bool) error {
  z.pMutex.RLock()
	if _, exists := z.printers[name]; exists {
		z.pMutex.RUnlock()
		return fmt.Errorf("Bonjour already has printer %s", name)
	}
	z.pMutex.RUnlock()

  op := dnssd.NewRegisterOp(name, serviceType, int(port), nullDNSSDRegisterCallback)
  op.SetTXTPair("ty", ty)
  op.SetTXTPair("note", note)
  op.SetTXTPair("url", url)
  op.SetTXTPair("type", "printer")
  op.SetTXTPair("id", id)
  var cs string
  if online {
		cs = "online"
  } else {
		cs = "offline"
	}
  op.SetTXTPair("cs", cs)
  err := op.Start()
	if err != nil {
		return err
	}
  z.pMutex.Lock()
	defer z.pMutex.Unlock()

	z.printers[name] = *op
	return nil
}

func (z *zeroconf) updatePrinterTXT(name, ty, note, url, id string, online bool) error {
  z.pMutex.RLock()
	defer z.pMutex.RUnlock()

	if op, exists := z.printers[name]; exists {
		op.SetTXTPair("ty", ty)
    op.SetTXTPair("note", note)
    op.SetTXTPair("url", url)
    op.SetTXTPair("type", "printer")
    op.SetTXTPair("id", id)
    var cs string
    if online {
		  cs = "online"
    } else {
		  cs = "offline"
	  }
    op.SetTXTPair("cs", cs)
	} else {
		return fmt.Errorf("Bonjour can't update printer %s that hasn't been added", name)
	}
	return nil
}

func (z *zeroconf) removePrinter(name string) error {
  z.pMutex.RLock()
	defer z.pMutex.RUnlock()

	if op, exists := z.printers[name]; exists {
		op.Stop()
    delete(z.printers, name)
    log.Info("Unregistered %s with bonjour", name)
	} else {
		return fmt.Errorf("Bonjour can't remove printer %s that hasn't been added", name)
	}

	return nil
}

func (z *zeroconf) quit() {
	z.pMutex.Lock()
	defer z.pMutex.Unlock()

	for name, op := range z.printers {
		op.Stop()
		delete(z.printers, name)
  }
}