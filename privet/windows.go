// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package privet

import (
	"errors"
)

type zeroconf struct{}

func newZeroconf() (*zeroconf, error) {
	return nil, errors.New("Privet has not been implemented for Windows")
}

func (z *zeroconf) addPrinter(name string, port uint16, ty, note, url, id string, online bool) error {
	return nil
}

func (z *zeroconf) updatePrinterTXT(name, ty, note, url, id string, online bool) error {
	return nil
}

func (z *zeroconf) removePrinter(name string) error {
	return nil
}

func (z *zeroconf) quit() {}
