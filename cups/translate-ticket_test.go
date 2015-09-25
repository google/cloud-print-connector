/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/cups-connector/cdd"
)

func TestTranslateTicket(t *testing.T) {
	expected := map[string]string{}
	o := translateTicket(nil)
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected %+v, got %+v", expected, o)
		t.Fail()
	}

	ticket := cdd.CloudJobTicket{}
	o = translateTicket(&ticket)
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected %+v, got %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"number-up", "a"},
			cdd.VendorTicketItem{"a:b/c:d/e", "f"},
		},
		Color:           &cdd.ColorTicketItem{VendorID: "ColorModel:zebra-stripes", Type: cdd.ColorTypeCustomMonochrome},
		Duplex:          &cdd.DuplexTicketItem{Type: cdd.DuplexNoDuplex},
		PageOrientation: &cdd.PageOrientationTicketItem{Type: cdd.PageOrientationAuto},
		Copies:          &cdd.CopiesTicketItem{Copies: 2},
		Margins:         &cdd.MarginsTicketItem{100000, 100000, 100000, 100000},
		DPI:             &cdd.DPITicketItem{100, 100, "q"},
		FitToPage:       &cdd.FitToPageTicketItem{cdd.FitToPageNoFitting},
		MediaSize:       &cdd.MediaSizeTicketItem{100000, 100000, false, "r"},
		Collate:         &cdd.CollateTicketItem{false},
		ReverseOrder:    &cdd.ReverseOrderTicketItem{false},
	}
	expected = map[string]string{
		"number-up":           "a",
		"a":                   "b",
		"c":                   "d",
		"e":                   "f",
		"ColorModel":          "zebra-stripes",
		"Duplex":              "None",
		"copies":              "2",
		"media-left-margin":   micronsToPoints(100000),
		"media-right-margin":  micronsToPoints(100000),
		"media-top-margin":    micronsToPoints(100000),
		"media-bottom-margin": micronsToPoints(100000),
		"Resolution":          "q",
		"fit-to-page":         "false",
		"PageSize":            "r",
		"collate":             "false",
		"outputorder":         "normal",
	}
	o = translateTicket(&ticket)
	if !reflect.DeepEqual(o, expected) {
		eSorted := make([]string, 0, len(expected))
		for k := range expected {
			eSorted = append(eSorted, fmt.Sprintf("%s:%s", k, expected[k]))
		}
		sort.Strings(eSorted)
		oSorted := make([]string, 0, len(o))
		for k := range o {
			oSorted = append(oSorted, fmt.Sprintf("%s:%s", k, o[k]))
		}
		sort.Strings(oSorted)
		t.Logf("expected\n %+v\ngot\n %+v", strings.Join(eSorted, ","), strings.Join(oSorted, ","))
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		Color:           &cdd.ColorTicketItem{VendorID: "print-color-mode:color", Type: cdd.ColorTypeStandardColor},
		PageOrientation: &cdd.PageOrientationTicketItem{Type: cdd.PageOrientationLandscape},
		DPI:             &cdd.DPITicketItem{100, 100, ""},
		MediaSize:       &cdd.MediaSizeTicketItem{100000, 100000, false, ""},
	}
	expected = map[string]string{
		"print-color-mode":      "color",
		"orientation-requested": "4",
		"Resolution":            "100x100dpi",
		"PageSize":              "Custom.283x283",
	}
	o = translateTicket(&ticket)
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		Color: &cdd.ColorTicketItem{VendorID: "CMAndResolution:Gray600x600dpi", Type: cdd.ColorTypeStandardColor},
	}
	expected = map[string]string{
		"CMAndResolution": "Gray600x600dpi",
	}
	o = translateTicket(&ticket)
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}
}
