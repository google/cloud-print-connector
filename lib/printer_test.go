/*
Copyright 2016 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import (
	"testing"
	"reflect"
)

func TestFilterBlacklistPrinters(t *testing.T) {
	printers := []Printer {
		{Name: "Stay1"},
		{Name: "Go1"},
		{Name: "Go2"},
		{Name: "Stay2"},
	}
	blacklist := map[string]interface{} {
		"Go1": "",
		"Go2": "",
	}
	correctFilteredPrinters := []Printer {
		{Name: "Stay1"},
		{Name: "Stay2"},
	}

	filteredPrinters := FilterBlacklistPrinters(printers, blacklist)

	if !reflect.DeepEqual(filteredPrinters, correctFilteredPrinters) {
		t.Fatalf("filtering result incorrect: %v", filteredPrinters)
	}
}

func TestFilterWhitelistPrinters(t *testing.T) {
	printers := []Printer {
		{Name: "Stay1"},
		{Name: "Go1"},
		{Name: "Go2"},
		{Name: "Stay2"},
	}
	whitelist := map[string]interface{} {
		"Stay1": "",
		"Stay2": "",
	}
	correctFilteredPrinters := []Printer {
		{Name: "Stay1"},
		{Name: "Stay2"},
	}

	filteredPrinters := FilterWhitelistPrinters(printers, whitelist)

	if !reflect.DeepEqual(filteredPrinters, correctFilteredPrinters) {
		t.Fatalf("filtering result incorrect: %v", filteredPrinters)
	}
}
