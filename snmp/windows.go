// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package snmp

import (
	"errors"

	"github.com/google/cups-connector/lib"
)

type SNMPManager struct {}

func NewSNMPManager(community string, maxConnections uint) (*SNMPManager, error) {
	return nil, errors.New("SNMP has not been implemented for Windows")
}

func (s *SNMPManager) Quit() {
}

func (s *SNMPManager) AugmentPrinters(printers []lib.Printer) error {
	return nil
}
