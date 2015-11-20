/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/lib"
)

var rVendorIDKeyValue = regexp.MustCompile(
	`^([^\` + internalKeySeparator + `]+)(?:` + internalKeySeparator + `(.+))?$`)

// translateTicket converts a CloudJobTicket to a map of options, suitable for a new CUPS print job.
func translateTicket(printer *lib.Printer, ticket *cdd.CloudJobTicket) (map[string]string, error) {
	if printer == nil || ticket == nil {
		return map[string]string{}, nil
	}

	m := map[string]string{}
	for _, vti := range ticket.Print.VendorTicketItem {
		if vti.ID == ricohPasswordVendorID {
			if !rRicohPasswordFormat.MatchString(vti.Value) {
				return map[string]string{}, errors.New("Invalid password format")
			}
		}

		for _, option := range strings.Split(vti.ID, internalValueSeparator) {
			var key, value string
			parts := rVendorIDKeyValue.FindStringSubmatch(option)
			if parts == nil || parts[2] == "" {
				key, value = option, vti.Value
			} else {
				key, value = parts[1], parts[2]
			}
			m[key] = value
		}
	}
	if ticket.Print.Color != nil && printer.Description.Color != nil {
		if ticket.Print.Color.VendorID != "" {
			m[printer.Description.Color.VendorKey] = ticket.Print.Color.VendorID
		} else {
			// The ticket doesn't provide the VendorID. Let's find it.
			for _, colorOption := range printer.Description.Color.Option {
				if ticket.Print.Color.Type == colorOption.Type {
					m[printer.Description.Color.VendorKey] = colorOption.VendorID
				}
			}
		}
	}
	if ticket.Print.Duplex != nil && printer.Description.Duplex != nil {
		for _, duplexOption := range printer.Description.Duplex.Option {
			if ticket.Print.Duplex.Type == duplexOption.Type {
				m[printer.Description.Duplex.VendorKey] = duplexOption.VendorID
			}
		}
	}
	if ticket.Print.PageOrientation != nil && printer.Description.PageOrientation != nil {
		if orientation, exists := orientationValueByType[ticket.Print.PageOrientation.Type]; exists {
			m[attrOrientationRequested] = orientation
		}
	}
	if ticket.Print.Copies != nil && printer.Description.Copies != nil {
		m[attrCopies] = strconv.FormatInt(int64(ticket.Print.Copies.Copies), 10)
	}
	if ticket.Print.Margins != nil && printer.Description.Margins != nil {
		m[attrMediaLeftMargin] = micronsToPoints(ticket.Print.Margins.LeftMicrons)
		m[attrMediaRightMargin] = micronsToPoints(ticket.Print.Margins.RightMicrons)
		m[attrMediaTopMargin] = micronsToPoints(ticket.Print.Margins.TopMicrons)
		m[attrMediaBottomMargin] = micronsToPoints(ticket.Print.Margins.BottomMicrons)
	}
	if ticket.Print.DPI != nil && printer.Description.DPI != nil {
		if ticket.Print.DPI.VendorID != "" {
			m[ppdResolution] = ticket.Print.DPI.VendorID
		} else {
			for _, dpiOption := range printer.Description.DPI.Option {
				if ticket.Print.DPI.HorizontalDPI == dpiOption.HorizontalDPI &&
					ticket.Print.DPI.VerticalDPI == dpiOption.VerticalDPI {
					m[ppdResolution] = dpiOption.VendorID
				}
			}
		}
	}
	if ticket.Print.FitToPage != nil && printer.Description.FitToPage != nil {
		switch ticket.Print.FitToPage.Type {
		case cdd.FitToPageFitToPage:
			m[attrFitToPage] = attrTrue
		case cdd.FitToPageNoFitting:
			m[attrFitToPage] = attrFalse
		}
	}
	if ticket.Print.MediaSize != nil && printer.Description.MediaSize != nil {
		if ticket.Print.MediaSize.VendorID != "" {
			m[ppdPageSize] = ticket.Print.MediaSize.VendorID
		} else {
			widthPoints := micronsToPoints(ticket.Print.MediaSize.WidthMicrons)
			heightPoints := micronsToPoints(ticket.Print.MediaSize.HeightMicrons)
			m[ppdPageSize] = fmt.Sprintf("Custom.%sx%s", widthPoints, heightPoints)
		}
	}
	if ticket.Print.Collate != nil && printer.Description.Collate != nil {
		if ticket.Print.Collate.Collate {
			m[attrCollate] = attrTrue
		} else {
			m[attrCollate] = attrFalse
		}
	}
	if ticket.Print.ReverseOrder != nil && printer.Description.ReverseOrder != nil {
		if ticket.Print.ReverseOrder.ReverseOrder {
			m[attrOutputOrder] = "reverse"
		} else {
			m[attrOutputOrder] = "normal"
		}
	}

	return m, nil
}

func micronsToPoints(microns int32) string {
	return strconv.Itoa(int(float32(microns)*72/25400 + 0.5))
}
