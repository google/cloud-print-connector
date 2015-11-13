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
	"github.com/google/cups-connector/lib"
)

func TestTranslateTicket(t *testing.T) {
	printer := lib.Printer{}
	expected := map[string]string{}
	o, err := translateTicket(&printer, nil)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected %+v, got %+v", expected, o)
		t.Fail()
	}

	ticket := cdd.CloudJobTicket{}
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected %+v, got %+v", expected, o)
		t.Fail()
	}

	printer = lib.Printer{
		Description: &cdd.PrinterDescriptionSection{
			Color: &cdd.Color{
				Option: []cdd.ColorOption{
					cdd.ColorOption{
						VendorID: "zebra-stripes",
						Type:     cdd.ColorTypeCustomMonochrome,
					},
				},
				VendorKey: "ColorModel",
			},
			Duplex: &cdd.Duplex{
				Option: []cdd.DuplexOption{
					cdd.DuplexOption{
						Type:     cdd.DuplexNoDuplex,
						VendorID: "None",
					},
				},
				VendorKey: "Duplex",
			},
			PageOrientation: &cdd.PageOrientation{},
			Copies:          &cdd.Copies{},
			Margins:         &cdd.Margins{},
			DPI: &cdd.DPI{
				Option: []cdd.DPIOption{
					cdd.DPIOption{
						HorizontalDPI: 100,
						VerticalDPI:   100,
						VendorID:      "q",
					},
				},
			},
			FitToPage:    &cdd.FitToPage{},
			MediaSize:    &cdd.MediaSize{},
			Collate:      &cdd.Collate{},
			ReverseOrder: &cdd.ReverseOrder{},
		},
	}
	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"number-up", "a"},
			cdd.VendorTicketItem{"a:b/c:d/e", "f"},
		},
		Color:           &cdd.ColorTicketItem{VendorID: "zebra-stripes", Type: cdd.ColorTypeCustomMonochrome},
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
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
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

	printer.Description = &cdd.PrinterDescriptionSection{
		Color: &cdd.Color{
			Option: []cdd.ColorOption{
				cdd.ColorOption{
					VendorID: "color",
					Type:     cdd.ColorTypeStandardColor,
				},
			},
			VendorKey: "print-color-mode",
		},
		Duplex: &cdd.Duplex{
			Option: []cdd.DuplexOption{
				cdd.DuplexOption{
					Type:     cdd.DuplexLongEdge,
					VendorID: "Single",
				},
			},
			VendorKey: "KMDuplex",
		},
		PageOrientation: &cdd.PageOrientation{},
		DPI: &cdd.DPI{
			Option: []cdd.DPIOption{
				cdd.DPIOption{
					HorizontalDPI: 100,
					VerticalDPI:   100,
					VendorID:      "q",
				},
			},
		},
		MediaSize: &cdd.MediaSize{},
	}
	ticket.Print = cdd.PrintTicketSection{
		Color:           &cdd.ColorTicketItem{VendorID: "color", Type: cdd.ColorTypeStandardColor},
		Duplex:          &cdd.DuplexTicketItem{Type: cdd.DuplexLongEdge},
		PageOrientation: &cdd.PageOrientationTicketItem{Type: cdd.PageOrientationLandscape},
		DPI:             &cdd.DPITicketItem{100, 100, ""},
		MediaSize:       &cdd.MediaSizeTicketItem{100000, 100000, false, ""},
	}
	expected = map[string]string{
		"print-color-mode":      "color",
		"KMDuplex":              "Single",
		"orientation-requested": "4",
		"Resolution":            "q",
		"PageSize":              "Custom.283x283",
	}
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	printer.Description.Color = &cdd.Color{
		Option: []cdd.ColorOption{
			cdd.ColorOption{
				VendorID: "Gray600x600dpi",
				Type:     cdd.ColorTypeStandardColor,
			},
		},
		VendorKey: "CMAndResolution",
	}
	ticket.Print = cdd.PrintTicketSection{
		Color: &cdd.ColorTicketItem{VendorID: "Gray600x600dpi", Type: cdd.ColorTypeStandardColor},
	}
	expected = map[string]string{
		"CMAndResolution": "Gray600x600dpi",
	}
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	printer.Description.Color = &cdd.Color{
		Option: []cdd.ColorOption{
			cdd.ColorOption{
				VendorID: "Color",
				Type:     cdd.ColorTypeStandardColor,
			},
		},
		VendorKey: "SelectColor",
	}
	ticket.Print = cdd.PrintTicketSection{
		Color: &cdd.ColorTicketItem{VendorID: "Color"},
	}
	expected = map[string]string{
		"SelectColor": "Color",
	}
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		Color: &cdd.ColorTicketItem{Type: cdd.ColorTypeStandardColor},
	}
	o, err = translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}
}

func TestTranslateTicket_RicohLockedPrint(t *testing.T) {
	printer := lib.Printer{}
	ticket := cdd.CloudJobTicket{}
	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"JobType:LockedPrint/LockedPrintPassword", "1234"},
		},
	}
	expected := map[string]string{
		"JobType":             "LockedPrint",
		"LockedPrintPassword": "1234",
	}
	o, err := translateTicket(&printer, &ticket)
	if err != nil {
		t.Logf("did not expect error %s", err)
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"JobType:LockedPrint/LockedPrintPassword", ""},
		},
	}
	expected = map[string]string{}
	o, err = translateTicket(&printer, &ticket)
	if err == nil {
		t.Log("expected error")
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"JobType:LockedPrint/LockedPrintPassword", "123"},
		},
	}
	o, err = translateTicket(&printer, &ticket)
	if err == nil {
		t.Log("expected error")
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"JobType:LockedPrint/LockedPrintPassword", "12345"},
		},
	}
	o, err = translateTicket(&printer, &ticket)
	if err == nil {
		t.Log("expected error")
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}

	ticket.Print = cdd.PrintTicketSection{
		VendorTicketItem: []cdd.VendorTicketItem{
			cdd.VendorTicketItem{"JobType:LockedPrint/LockedPrintPassword", "1bc3"},
		},
	}
	o, err = translateTicket(&printer, &ticket)
	if err == nil {
		t.Log("expected error")
		t.Fail()
	}
	if !reflect.DeepEqual(o, expected) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, o)
		t.Fail()
	}
}
