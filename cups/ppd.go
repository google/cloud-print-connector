/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package cups

import (
	"regexp"
	"strings"
)

var (
	// Get manufacturer name from PPD.
	reManufacturer = regexp.MustCompile(`(?m)^\*Manufacturer:\s*"(.+)"\s*$`)
	// Get model name from PPD.
	reModel = regexp.MustCompile(`(?m)^\*ModelName:\s*"(.+)"\s*$`)
	// Source of data: PPD Spec 4.3, Table D.1.
	manTitleCaseLookup = map[string]string{
		"ADOBE":        "Adobe",
		"APPLE":        "Apple",
		"BULL":         "Bull",
		"CANON":        "Canon",
		"COMPAQ":       "Compaq",
		"DATAPRODUCTS": "Dataproducts",
		"KODAK":        "Kodak",
		"LEXMARK":      "Lexmark",
		"MANNESMANN":   "Mannesmann",
		"OCE":          "Oce",
		"OKI":          "Oki",
		"PANASONIC":    "Panasonic",
		"RICOH":        "Ricoh",
		"EPSON":        "Epson",
		"SEIKO":        "Seiko",
		"TEKTRONIX":    "Tektronix",
		"WANG":         "Wang",
		"XEROX":        "Xerox",
	}
)

// parseManufacturerAndModel finds the *Manufacturer and *ModelName values in a PPD string.
func parseManufacturerAndModel(ppd string) (string, string) {
	manufacturer := "Unknown"
	res := reManufacturer.FindStringSubmatch(ppd)
	if len(res) > 1 && res[1] != "" {
		manufacturer = res[1]
	}

	model := "Unknown"
	res = reModel.FindStringSubmatch(ppd)
	if len(res) > 1 && res[1] != "" {
		model = res[1]
	}

	if strings.HasPrefix(model, manufacturer) && len(model) > len(manufacturer) {
		// ModelName starts with Manufacturer (as it should).
		// Remove Manufacturer from ModelName.
		model = strings.TrimPrefix(model, manufacturer)
		model = strings.TrimSpace(model)
	}

	if m, exists := manTitleCaseLookup[manufacturer]; exists {
		manufacturer = m
	}

	return manufacturer, model
}
