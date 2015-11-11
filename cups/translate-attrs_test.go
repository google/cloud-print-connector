/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/log"
)

func TestGetUUID(t *testing.T) {
	u := getUUID(nil)
	if u != "" {
		t.Logf("expected empty string, got %s", u)
		t.Fail()
	}

	pt := map[string][]string{}
	u = getUUID(pt)
	if u != "" {
		t.Logf("expected empty string, got %s", u)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterUUID: []string{"123"}}
	expected := "123"
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterUUID: []string{"abc:123"}}
	expected = "abc:123"
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterUUID: []string{"urn:123"}}
	expected = "123"
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterUUID: []string{"uuid:123"}}
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterUUID: []string{"urn:uuid:123"}}
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{
		attrPrinterUUID: []string{"urn:uuid:123"},
		attrPrinterName: []string{"my-name"},
	}
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}

	pt = map[string][]string{
		attrPrinterName: []string{"my-name"},
	}
	expected = "my-name"
	u = getUUID(pt)
	if u != expected {
		t.Logf("expected %s, got %s", expected, u)
		t.Fail()
	}
}

func TestGetState(t *testing.T) {
	state := getState(nil)
	if cdd.CloudDeviceStateIdle != state {
		t.Logf("expected %+v, got %+v", cdd.CloudDeviceStateIdle, state)
		t.Fail()
	}

	pt := map[string][]string{}
	state = getState(pt)
	if cdd.CloudDeviceStateIdle != state {
		t.Logf("expected %+v, got %+v", cdd.CloudDeviceStateIdle, state)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterState: []string{"1"}}
	state = getState(pt)
	if cdd.CloudDeviceStateIdle != state {
		t.Logf("expected %+v, got %+v", cdd.CloudDeviceStateIdle, state)
		t.Fail()
	}

	pt = map[string][]string{attrPrinterState: []string{"4"}}
	state = getState(pt)
	if cdd.CloudDeviceStateProcessing != state {
		t.Logf("expected %+v, got %+v", cdd.CloudDeviceStateProcessing, state)
		t.Fail()
	}
}

func TestGetVendorState(t *testing.T) {
	vs := getVendorState(nil)
	if nil != vs {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	vs = getVendorState(pt)
	if nil != vs {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		attrPrinterStateReasons: []string{"broken-arrow", "peanut-butter-jam-warning"},
	}
	expected := &cdd.VendorState{
		Item: []cdd.VendorStateItem{
			cdd.VendorStateItem{
				DescriptionLocalized: cdd.NewLocalizedString("broken-arrow"),
				State:                cdd.VendorStateError,
			},
			cdd.VendorStateItem{
				DescriptionLocalized: cdd.NewLocalizedString("peanut-butter-jam-warning"),
				State:                cdd.VendorStateWarning,
			},
		},
	}
	vs = getVendorState(pt)
	if !reflect.DeepEqual(expected, vs) {
		t.Logf("expected\n %+v\ngot\n %+v", expected, vs)
		t.Fail()
	}
}

func TestConvertSupportedContentType(t *testing.T) {
	sct := convertSupportedContentType(nil)
	if sct != nil {
		t.Logf("expected nil, got %+v", sct)
		t.Fail()
	}

	pt := map[string][]string{}
	sct = convertSupportedContentType(pt)
	if sct != nil {
		t.Logf("expected nil, got %+v", sct)
		t.Fail()
	}

	pt = map[string][]string{
		attrDocumentFormatSupported: []string{
			"image/png", "image/pwg-raster", "application/octet-stream",
			"application/pdf", "application/postscript"},
		"pdf-versions-supported": []string{"adobe-1.3", "adobe-1.4", "adobe-1.6", "dingbat"},
	}
	expected := &[]cdd.SupportedContentType{
		cdd.SupportedContentType{
			ContentType: "application/pdf",
			MinVersion:  "1.3",
			MaxVersion:  "1.6",
		},
		cdd.SupportedContentType{ContentType: "application/postscript"},
		cdd.SupportedContentType{ContentType: "image/png"},
	}
	sct = convertSupportedContentType(pt)
	if !reflect.DeepEqual(*expected, *sct) {
		t.Logf("expected\n %+v\ngot\n %+v", *expected, *sct)
		t.Fail()
	}

	pt = map[string][]string{
		attrDocumentFormatSupported: []string{"image/png"},
	}
	expected = &[]cdd.SupportedContentType{
		cdd.SupportedContentType{ContentType: "application/pdf"},
		cdd.SupportedContentType{ContentType: "application/postscript"},
		cdd.SupportedContentType{ContentType: "image/png"},
	}
	sct = convertSupportedContentType(pt)
	if !reflect.DeepEqual(*expected, *sct) {
		t.Logf("expected %+v, got %+v", *expected, *sct)
		t.Fail()
	}
}

func TestConvertMarkers(t *testing.T) {
	log.SetLevel(log.ERROR)

	m, ms := convertMarkers(nil)
	if m != nil {
		t.Logf("expected nil")
		t.Fail()
	}
	if ms != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	m, ms = convertMarkers(pt)
	if m != nil {
		t.Logf("expected nil")
		t.Fail()
	}
	if ms != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		attrMarkerNames:  []string{"black", "black", "black"},
		attrMarkerTypes:  []string{"toner", "toner", "ink"},
		attrMarkerLevels: []string{"10", "11", "12"},
	}
	m, ms = convertMarkers(pt)
	if m != nil {
		t.Logf("expected nil")
		t.Fail()
	}
	if ms != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		attrMarkerNames:  []string{"black", "color"},
		attrMarkerTypes:  []string{"toner", "toner", "ink"},
		attrMarkerLevels: []string{"10", "11", "12"},
	}
	m, ms = convertMarkers(pt)
	if m != nil {
		t.Logf("expected nil")
		t.Fail()
	}
	if ms != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		attrMarkerNames:  []string{"black", "color", "rainbow"},
		attrMarkerTypes:  []string{"toner", "toner"},
		attrMarkerLevels: []string{"10", "11", "12"},
	}
	m, ms = convertMarkers(pt)
	if m != nil {
		t.Logf("expected nil")
		t.Fail()
	}
	if ms != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		attrMarkerNames:  []string{"black", " Reorder Part #12345", "color", "rainbow", "zebra", "pony"},
		attrMarkerTypes:  []string{"toner", "toner", "ink", "staples", "water", " Reorder H2O"},
		attrMarkerLevels: []string{"10", "11", "12", "208", "13"},
	}
	mExpected := &[]cdd.Marker{
		cdd.Marker{
			VendorID: "black, Reorder Part #12345",
			Type:     cdd.MarkerToner,
			Color:    &cdd.MarkerColor{Type: cdd.MarkerColorBlack},
		},
		cdd.Marker{
			VendorID: "color",
			Type:     cdd.MarkerToner,
			Color:    &cdd.MarkerColor{Type: cdd.MarkerColorColor},
		},
		cdd.Marker{
			VendorID: "rainbow",
			Type:     cdd.MarkerInk,
			Color: &cdd.MarkerColor{
				Type: cdd.MarkerColorCustom,
				CustomDisplayNameLocalized: cdd.NewLocalizedString("rainbow"),
			},
		},
		cdd.Marker{
			VendorID: "zebra",
			Type:     cdd.MarkerStaples,
		},
	}
	ten, eleven, twelve, eighty := int32(10), int32(11), int32(12), int32(80)
	msExpected := &cdd.MarkerState{
		Item: []cdd.MarkerStateItem{
			cdd.MarkerStateItem{
				VendorID:     "black, Reorder Part #12345",
				State:        cdd.MarkerStateExhausted,
				LevelPercent: &ten,
			},
			cdd.MarkerStateItem{
				VendorID:     "color",
				State:        cdd.MarkerStateOK,
				LevelPercent: &eleven,
			},
			cdd.MarkerStateItem{
				VendorID:     "rainbow",
				State:        cdd.MarkerStateOK,
				LevelPercent: &twelve,
			},
			cdd.MarkerStateItem{
				VendorID:     "zebra",
				State:        cdd.MarkerStateOK,
				LevelPercent: &eighty,
			},
		},
	}
	m, ms = convertMarkers(pt)
	if !reflect.DeepEqual(mExpected, m) {
		e, _ := json.Marshal(mExpected)
		f, _ := json.Marshal(m)
		t.Logf("expected\n %s\ngot\n %s", e, f)
		t.Fail()
	}
	if !reflect.DeepEqual(msExpected, ms) {
		e, _ := json.Marshal(msExpected)
		f, _ := json.Marshal(ms)
		t.Logf("expected\n %s\ngot\n %s", e, f)
		t.Fail()
	}
	pt = map[string][]string{
		attrMarkerNames:  []string{"black", "color", "rainbow", "zebra", "pony"},
		attrMarkerTypes:  []string{"toner", "toner", "ink", "staples", "water"},
		attrMarkerLevels: []string{"10", "11", "12", "208", "13"},
	}
	mExpected = &[]cdd.Marker{
		cdd.Marker{
			VendorID: "black",
			Type:     cdd.MarkerToner,
			Color:    &cdd.MarkerColor{Type: cdd.MarkerColorBlack},
		},
		cdd.Marker{
			VendorID: "color",
			Type:     cdd.MarkerToner,
			Color:    &cdd.MarkerColor{Type: cdd.MarkerColorColor},
		},
		cdd.Marker{
			VendorID: "rainbow",
			Type:     cdd.MarkerInk,
			Color: &cdd.MarkerColor{
				Type: cdd.MarkerColorCustom,
				CustomDisplayNameLocalized: cdd.NewLocalizedString("rainbow"),
			},
		},
		cdd.Marker{
			VendorID: "zebra",
			Type:     cdd.MarkerStaples,
		},
	}
	msExpected = &cdd.MarkerState{
		Item: []cdd.MarkerStateItem{
			cdd.MarkerStateItem{
				VendorID:     "black",
				State:        cdd.MarkerStateExhausted,
				LevelPercent: &ten,
			},
			cdd.MarkerStateItem{
				VendorID:     "color",
				State:        cdd.MarkerStateOK,
				LevelPercent: &eleven,
			},
			cdd.MarkerStateItem{
				VendorID:     "rainbow",
				State:        cdd.MarkerStateOK,
				LevelPercent: &twelve,
			},
			cdd.MarkerStateItem{
				VendorID:     "zebra",
				State:        cdd.MarkerStateOK,
				LevelPercent: &eighty,
			},
		},
	}
	m, ms = convertMarkers(pt)
	if !reflect.DeepEqual(mExpected, m) {
		e, _ := json.Marshal(mExpected)
		f, _ := json.Marshal(m)
		t.Logf("expected\n %s\ngot\n %s", e, f)
		t.Fail()
	}
	if !reflect.DeepEqual(msExpected, ms) {
		e, _ := json.Marshal(msExpected)
		f, _ := json.Marshal(ms)
		t.Logf("expected\n %s\ngot\n %s", e, f)
		t.Fail()
	}
}

func TestConvertPagesPerSheet(t *testing.T) {
	vc := convertPagesPerSheet(nil)
	if vc != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	vc = convertPagesPerSheet(pt)
	if vc != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		"number-up-default":   []string{"4"},
		"number-up-supported": []string{"1", "2", "4"},
	}
	expected := &cdd.VendorCapability{
		ID:   "number-up",
		Type: cdd.VendorCapabilitySelect,
		SelectCap: &cdd.SelectCapability{
			Option: []cdd.SelectCapabilityOption{
				cdd.SelectCapabilityOption{
					Value:                "1",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("1"),
				},
				cdd.SelectCapabilityOption{
					Value:                "2",
					IsDefault:            false,
					DisplayNameLocalized: cdd.NewLocalizedString("2"),
				},
				cdd.SelectCapabilityOption{
					Value:                "4",
					IsDefault:            true,
					DisplayNameLocalized: cdd.NewLocalizedString("4"),
				},
			},
		},
		DisplayNameLocalized: cdd.NewLocalizedString("Pages per sheet"),
	}
	vc = convertPagesPerSheet(pt)
	if !reflect.DeepEqual(expected, vc) {
		e, _ := json.Marshal(expected)
		f, _ := json.Marshal(vc)
		t.Logf("expected\n %s\ngot\n %s", e, f)
		t.Fail()
	}
}

func TestConvertPageOrientation(t *testing.T) {
	po := convertPageOrientation(nil)
	if po != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	po = convertPageOrientation(pt)
	if po != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		"orientation-requested-default":   []string{"4"},
		"orientation-requested-supported": []string{"3", "4"},
	}
	expected := &cdd.PageOrientation{
		Option: []cdd.PageOrientationOption{
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationAuto,
				IsDefault: false,
			},
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationPortrait,
				IsDefault: false,
			},
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationLandscape,
				IsDefault: true,
			},
		},
	}
	po = convertPageOrientation(pt)
	if !reflect.DeepEqual(expected, po) {
		t.Logf("expected %+v, got %+v", expected, po)
		t.Fail()
	}

	pt = map[string][]string{
		"orientation-requested-supported": []string{"3", "4"},
	}
	expected = &cdd.PageOrientation{
		Option: []cdd.PageOrientationOption{
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationAuto,
				IsDefault: true,
			},
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationPortrait,
				IsDefault: false,
			},
			cdd.PageOrientationOption{
				Type:      cdd.PageOrientationLandscape,
				IsDefault: false,
			},
		},
	}
	po = convertPageOrientation(pt)
	if !reflect.DeepEqual(expected, po) {
		t.Logf("expected %+v, got %+v", expected, po)
		t.Fail()
	}

	pt = map[string][]string{
		"orientation-requested-default":   []string{},
		"orientation-requested-supported": []string{"3", "4"},
	}
	po = convertPageOrientation(pt)
	if !reflect.DeepEqual(expected, po) {
		t.Logf("expected %+v, got %+v", expected, po)
		t.Fail()
	}
}

func TestConvertCopies(t *testing.T) {
	c := convertCopies(nil)
	if c != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	c = convertCopies(pt)
	if c != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		"copies-default":   []string{"2"},
		"copies-supported": []string{"1~101"},
	}
	expected := &cdd.Copies{
		Default: int32(2),
		Max:     int32(101),
	}
	c = convertCopies(pt)
	if !reflect.DeepEqual(expected, c) {
		t.Logf("expected %+v, got %+v", expected, c)
		t.Fail()
	}
}

func TestConvertColorAttrs(t *testing.T) {
	c := convertColorAttrs(nil)
	if c != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt := map[string][]string{}
	c = convertColorAttrs(pt)
	if c != nil {
		t.Logf("expected nil")
		t.Fail()
	}

	pt = map[string][]string{
		"print-color-mode-default":   []string{"auto"},
		"print-color-mode-supported": []string{"color", "monochrome", "auto", "zebra"},
	}
	expected := &cdd.Color{
		Option: []cdd.ColorOption{
			cdd.ColorOption{"print-color-mode:color", cdd.ColorTypeStandardColor, "", false, cdd.NewLocalizedString("Color")},
			cdd.ColorOption{"print-color-mode:monochrome", cdd.ColorTypeStandardMonochrome, "", false, cdd.NewLocalizedString("Monochrome")},
			cdd.ColorOption{"print-color-mode:auto", cdd.ColorTypeAuto, "", true, cdd.NewLocalizedString("Auto")},
			cdd.ColorOption{"print-color-mode:zebra", cdd.ColorTypeCustomColor, "", false, cdd.NewLocalizedString("zebra")},
		},
	}
	c = convertColorAttrs(pt)
	if !reflect.DeepEqual(expected, c) {
		t.Logf("expected %+v, got %+v", expected, c)
		t.Fail()
	}
}
