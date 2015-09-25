/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/cups-connector/cdd"
)

var rVendorIDKeyValue = regexp.MustCompile(
	`^([^\` + internalKeySeparator + `]+)(?:` + internalKeySeparator + `(.+))?$`)

// translateTicket converts a CloudJobTicket to a map of options, suitable for a new CUPS print job.
func translateTicket(ticket *cdd.CloudJobTicket) map[string]string {
	m := map[string]string{}
	if ticket == nil {
		return m
	}

	for _, vti := range ticket.Print.VendorTicketItem {
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
	if ticket.Print.Color != nil {
		// TODO: Lookup VendorID by Color.Type in CDD when ticket Color.VendorID is empty?
		parts := rVendorIDKeyValue.FindStringSubmatch(ticket.Print.Color.VendorID)
		if parts != nil && parts[2] != "" {
			m[parts[1]] = parts[2]
		}
	}
	if ticket.Print.Duplex != nil {
		if ppdValue, exists := duplexPPDByCDD[ticket.Print.Duplex.Type]; exists {
			m[ppdDuplex] = ppdValue
		}
	}
	if ticket.Print.PageOrientation != nil {
		if orientation, exists := orientationValueByType[ticket.Print.PageOrientation.Type]; exists {
			m[attrOrientationRequested] = orientation
		}
	}
	if ticket.Print.Copies != nil {
		m[attrCopies] = strconv.FormatInt(int64(ticket.Print.Copies.Copies), 10)
	}
	if ticket.Print.Margins != nil {
		m[attrMediaLeftMargin] = micronsToPoints(ticket.Print.Margins.LeftMicrons)
		m[attrMediaRightMargin] = micronsToPoints(ticket.Print.Margins.RightMicrons)
		m[attrMediaTopMargin] = micronsToPoints(ticket.Print.Margins.TopMicrons)
		m[attrMediaBottomMargin] = micronsToPoints(ticket.Print.Margins.BottomMicrons)
	}
	if ticket.Print.DPI != nil {
		if ticket.Print.DPI.VendorID != "" {
			m[ppdResolution] = ticket.Print.DPI.VendorID
		} else {
			// TODO: Lookup VendorID in CDD?
			m[ppdResolution] = fmt.Sprintf("%dx%ddpi",
				ticket.Print.DPI.HorizontalDPI, ticket.Print.DPI.VerticalDPI)
		}
	}
	if ticket.Print.FitToPage != nil {
		switch ticket.Print.FitToPage.Type {
		case cdd.FitToPageFitToPage:
			m[attrFitToPage] = attrTrue
		case cdd.FitToPageNoFitting:
			m[attrFitToPage] = attrFalse
		}
	}
	if ticket.Print.MediaSize != nil {
		if ticket.Print.MediaSize.VendorID != "" {
			m[ppdPageSize] = ticket.Print.MediaSize.VendorID
		} else {
			widthPoints := micronsToPoints(ticket.Print.MediaSize.WidthMicrons)
			heightPoints := micronsToPoints(ticket.Print.MediaSize.HeightMicrons)
			m[ppdPageSize] = fmt.Sprintf("Custom.%sx%s", widthPoints, heightPoints)
		}
	}
	if ticket.Print.Collate != nil {
		if ticket.Print.Collate.Collate {
			m[attrCollate] = attrTrue
		} else {
			m[attrCollate] = attrFalse
		}
	}
	if ticket.Print.ReverseOrder != nil {
		if ticket.Print.ReverseOrder.ReverseOrder {
			m[attrOutputOrder] = "reverse"
		} else {
			m[attrOutputOrder] = "normal"
		}
	}

	return m
}

func micronsToPoints(microns int32) string {
	return strconv.Itoa(int(float32(microns)*72/25400 + 0.5))
}
