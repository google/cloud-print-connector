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
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/google/cups-connector/cdd"
)

// translateAttrs extracts a PrinterDescriptionSection, PrinterStateSection, name, default diplay name, UUID, and tags from maps of tags (CUPS attributes)
func translateAttrs(printerTags map[string][]string) (*cdd.PrinterDescriptionSection, *cdd.PrinterStateSection, string, string, string, map[string]string) {
	var name, info string
	if n, ok := printerTags[attrPrinterName]; ok && len(n) > 0 {
		name = n[0]
	}
	if i, ok := printerTags[attrPrinterInfo]; ok && len(i) > 0 {
		info = i[0]
	}
	uuid := getUUID(printerTags)

	var desc = cdd.PrinterDescriptionSection{VendorCapability: &[]cdd.VendorCapability{}}
	var state cdd.PrinterStateSection

	desc.SupportedContentType = convertSupportedContentType(printerTags)
	desc.Marker, state.MarkerState = convertMarkers(printerTags)
	desc.PageOrientation = convertPageOrientation(printerTags)
	desc.Copies = convertCopies(printerTags)
	desc.Color = convertColorAttrs(printerTags)
	if vc := convertPagesPerSheet(printerTags); vc != nil {
		*desc.VendorCapability = append(*desc.VendorCapability, *vc)
	}

	state.State = getState(printerTags)
	state.VendorState = getVendorState(printerTags)

	tags := make(map[string]string, len(printerTags))
	for k, v := range printerTags {
		tags[k] = strings.Join(v, ",")
	}

	return &desc, &state, name, info, uuid, tags
}

func getUUID(printerTags map[string][]string) string {
	var uuid string
	if u, ok := printerTags[attrPrinterUUID]; ok {
		uuid = u[0]
		uuid = strings.TrimPrefix(uuid, "urn:")
		uuid = strings.TrimPrefix(uuid, "uuid:")
	} else if u, ok := printerTags[attrPrinterName]; ok {
		// CUPS < 1.5 doesn't send a printer-uuid attribute.
		uuid = u[0]
	}
	return uuid
}

func getState(printerTags map[string][]string) cdd.CloudDeviceStateType {
	if s, ok := printerTags[attrPrinterState]; ok {
		switch s[0] {
		case "3":
			return cdd.CloudDeviceStateIdle
		case "4":
			return cdd.CloudDeviceStateProcessing
		case "5":
			return cdd.CloudDeviceStateStopped
		default:
			return cdd.CloudDeviceStateIdle
		}
	}
	return cdd.CloudDeviceStateIdle
}

func getVendorState(printerTags map[string][]string) *cdd.VendorState {
	reasons, exists := printerTags[attrPrinterStateReasons]
	if !exists || len(reasons) < 1 {
		return nil
	}

	sort.Strings(reasons)
	vendorState := &cdd.VendorState{Item: make([]cdd.VendorStateItem, len(reasons))}
	for i, reason := range reasons {
		vs := cdd.VendorStateItem{DescriptionLocalized: cdd.NewLocalizedString(reason)}
		if strings.HasSuffix(reason, "-error") {
			vs.State = cdd.VendorStateError
		} else if strings.HasSuffix(reason, "-warning") {
			vs.State = cdd.VendorStateWarning
		} else if strings.HasSuffix(reason, "-report") {
			vs.State = cdd.VendorStateInfo
		} else {
			vs.State = cdd.VendorStateError
		}
		vendorState.Item[i] = vs
	}

	return vendorState
}

func getAdobeVersionRange(pdfVersionsSupported []string) (string, string) {
	var min, max string
	for _, pdfVersion := range pdfVersionsSupported {
		var v string
		if _, err := fmt.Sscanf(pdfVersion, "adobe-%s", &v); err != nil || len(v) < 1 {
			continue
		}
		if min == "" || v < min {
			min = v
		}
		if max == "" || v > max {
			max = v
		}
	}
	return min, max
}

// CUPS can accept some unsafe types, like application/x-perl, so limit to these.
var mimeTypesAllowed = map[string]struct{}{
	"application/pdf":        struct{}{},
	"application/postscript": struct{}{},
	"image/pwg-raster":       struct{}{},
	"image/gif":              struct{}{},
	"image/jp2":              struct{}{},
	"image/jpeg":             struct{}{},
	"image/png":              struct{}{},
	"image/tiff":             struct{}{},
	"text/plain":             struct{}{},
	"text/rtf":               struct{}{},
}

func convertSupportedContentType(printerTags map[string][]string) *[]cdd.SupportedContentType {
	mimeTypes, exists := printerTags[attrDocumentFormatSupported]
	if !exists || len(mimeTypes) < 1 {
		return nil
	}

	pdfMin, pdfMax := getAdobeVersionRange(printerTags[attrPDFVersionsSupported])
	pdf := cdd.SupportedContentType{ContentType: "application/pdf"}
	if pdfMin != "" && pdfMax != "" {
		pdf.MinVersion = pdfMin
		pdf.MaxVersion = pdfMax
	}
	sct := []cdd.SupportedContentType{pdf, cdd.SupportedContentType{ContentType: "application/postscript"}}

	// Preferred order:
	//  1) PDF (vector and small).
	//  2) Postscript (vector).
	//  3) Any CUPS-supported formats (don't need conversion in client or cloud).
	//  4) PWG-Raster (all clients support but it's huge).

	for _, mimeType := range mimeTypes {
		if mimeType == "application/pdf" || // Already added.
			mimeType == "application/postscript" || // Already added.
			mimeType == "image/pwg-raster" { // Added last, if at all.
			continue
		}
		if _, exists = mimeTypesAllowed[mimeType]; !exists {
			continue
		}
		sct = append(sct, cdd.SupportedContentType{ContentType: mimeType})
	}
	/*
		TODO: Consider adding pwg-raster with config option to enable/disable.
		- All clients authored by Google do not create PWG Raster jobs.
		- cups-filters only supports pwg-raster input in recent versions.
		  https://www.cups.org/pipermail/cups/2015-July/026927.html
	*/
	return &sct
}

var cupsMarkerNameToGCP map[string]cdd.MarkerColorType = map[string]cdd.MarkerColorType{
	"black":        cdd.MarkerColorBlack,
	"color":        cdd.MarkerColorColor,
	"cyan":         cdd.MarkerColorCyan,
	"magenta":      cdd.MarkerColorMagenta,
	"yellow":       cdd.MarkerColorYellow,
	"lightcyan":    cdd.MarkerColorLightCyan,
	"lightmagenta": cdd.MarkerColorLightMagenta,
	"gray":         cdd.MarkerColorGray,
	"lightgray":    cdd.MarkerColorLightGray,
	"pigmentblack": cdd.MarkerColorPigmentBlack,
	"matteblack":   cdd.MarkerColorMatteBlack,
	"photocyan":    cdd.MarkerColorPhotoCyan,
	"photomagenta": cdd.MarkerColorPhotoMagenta,
	"photoyellow":  cdd.MarkerColorPhotoYellow,
	"photogray":    cdd.MarkerColorPhotoGray,
	"red":          cdd.MarkerColorRed,
	"green":        cdd.MarkerColorGreen,
	"blue":         cdd.MarkerColorBlue,
}

// convertMarkers converts CUPS marker-(names|types|levels) to *[]cdd.Marker and *cdd.MarkerState.
//
// Normalizes marker type: toner(Cartridge|-cartridge) => toner,
// ink(Cartridge|-cartridge|Ribbon|-ribbon) => ink
func convertMarkers(printerTags map[string][]string) (*[]cdd.Marker, *cdd.MarkerState) {
	names, types, levels := printerTags[attrMarkerNames], printerTags[attrMarkerTypes], printerTags[attrMarkerLevels]
	if len(names) == 0 || len(types) == 0 || len(levels) == 0 {
		return nil, nil
	}

	if len(names) != len(levels) {
		newNames := fixMarkers(names)
		if len(newNames) != len(levels) {
			glog.Warningf("Received badly-formatted marker-names from CUPS: %s, %s, %s",
				strings.Join(names, ";"), strings.Join(types, ";"), strings.Join(levels, ";"))
			return nil, nil
		}
		names = newNames
	}

	{
		nameSet := make(map[string]struct{}, len(names))
		for _, name := range names {
			if _, exists := nameSet[name]; exists {
				return nil, nil
			}
			nameSet[name] = struct{}{}
		}
	}

	if len(types) != len(levels) {
		newTypes := fixMarkers(types)
		if len(newTypes) != len(levels) {
			glog.Warningf("Received badly-formatted marker-types from CUPS: %s, %s, %s",
				strings.Join(names, ";"), strings.Join(types, ";"), strings.Join(levels, ";"))
			return nil, nil
		}
		types = newTypes
	}

	markers := make([]cdd.Marker, 0, len(names))
	states := cdd.MarkerState{make([]cdd.MarkerStateItem, 0, len(names))}
	for i := 0; i < len(names); i++ {
		if len(names[i]) == 0 {
			return nil, nil
		}
		var markerType cdd.MarkerType
		switch strings.ToLower(types[i]) {
		case "toner", "tonercartridge", "toner-cartridge":
			markerType = cdd.MarkerToner
		case "ink", "inkcartridge", "ink-cartridge", "ink-ribbon", "inkribbon":
			markerType = cdd.MarkerInk
		case "staples":
			markerType = cdd.MarkerStaples
		default:
			continue
		}

		var color *cdd.MarkerColor
		if markerType == cdd.MarkerToner || markerType == cdd.MarkerInk {
			nameStripped := strings.Replace(strings.Replace(strings.ToLower(names[i]), " ", "", -1), "-", "", -1)
			colorType := cdd.MarkerColorCustom
			for k, v := range cupsMarkerNameToGCP {
				if strings.HasPrefix(nameStripped, k) {
					colorType = v
					break
				}
			}
			color = &cdd.MarkerColor{Type: colorType}
			if colorType == cdd.MarkerColorCustom {
				name := names[i]
				name = strings.TrimSuffix(name, " Cartridge")
				name = strings.TrimSuffix(name, " cartridge")
				name = strings.TrimSuffix(name, " Ribbon")
				name = strings.TrimSuffix(name, " ribbon")
				name = strings.TrimSuffix(name, " Toner")
				name = strings.TrimSuffix(name, " toner")
				name = strings.TrimSuffix(name, " Ink")
				name = strings.TrimSuffix(name, " ink")
				name = strings.Replace(name, "-", " ", -1)
				color.CustomDisplayNameLocalized = cdd.NewLocalizedString(name)
			}
		}

		marker := cdd.Marker{
			VendorID: names[i],
			Type:     markerType,
			Color:    color,
		}

		level, err := strconv.ParseInt(levels[i], 10, 32)
		if err != nil {
			glog.Warningf("Failed to parse CUPS marker state %s=%s: %s", names[i], levels[i], err)
			return nil, nil
		}
		if level > 100 {
			// Lop off extra (proprietary?) bits.
			level = level & 0x7f
		}
		if level < 0 || level > 100 {
			return nil, nil
		}

		var state cdd.MarkerStateType
		if level > 10 {
			state = cdd.MarkerStateOK
		} else {
			state = cdd.MarkerStateExhausted
		}
		level32 := int32(level)
		markerState := cdd.MarkerStateItem{
			VendorID:     names[i],
			State:        state,
			LevelPercent: &level32,
		}

		markers = append(markers, marker)
		states.Item = append(states.Item, markerState)
	}

	return &markers, &states
}

//fixMarkers corrects some drivers' marker names/types where CUPS detects names/types with a comma
// as two separate values. The second value of these pairs contain a space, so it's easy to detect.
func fixMarkers(values []string) []string {
	var newValues []string

	for i := range values {
		if i > 0 && len(values[i]) > 1 && values[i][0] == ' ' {
			newValues[len(newValues)-1] = strings.Join([]string{newValues[len(newValues)-1], values[i]}, ",")
		} else {
			newValues = append(newValues, values[i])
		}
	}
	return newValues
}

func convertPagesPerSheet(printerTags map[string][]string) *cdd.VendorCapability {
	numberUpSupported, exists := printerTags[attrNumberUpSupported]
	if !exists {
		return nil
	}

	c := cdd.VendorCapability{
		ID:                   attrNumberUp,
		Type:                 cdd.VendorCapabilitySelect,
		SelectCap:            &cdd.SelectCapability{},
		DisplayNameLocalized: cdd.NewLocalizedString("Pages per sheet"),
	}

	def, exists := printerTags[attrNumberUpDefault]
	if !exists {
		def = []string{"1"}
	}

	for _, number := range numberUpSupported {
		option := cdd.SelectCapabilityOption{
			Value:                number,
			IsDefault:            reflect.DeepEqual(number, def[0]),
			DisplayNameLocalized: cdd.NewLocalizedString(number),
		}
		c.SelectCap.Option = append(c.SelectCap.Option, option)
	}

	return &c
}

var (
	pageOrientationByValue map[string]cdd.PageOrientationType = map[string]cdd.PageOrientationType{
		"3":    cdd.PageOrientationPortrait,
		"4":    cdd.PageOrientationLandscape,
		"auto": cdd.PageOrientationAuto, // custom value, not CUPS standard
	}
	orientationValueByType map[cdd.PageOrientationType]string = map[cdd.PageOrientationType]string{
		cdd.PageOrientationPortrait:  "3",
		cdd.PageOrientationLandscape: "4",
	}
)

func convertPageOrientation(printerTags map[string][]string) *cdd.PageOrientation {
	orientationDefault, exists := printerTags[attrOrientationRequestedDefault]
	if !exists || len(orientationDefault) != 1 {
		orientationDefault = []string{"auto"}
	}

	orientationSupported, exists := printerTags[attrOrientationRequestedSupported]
	if !exists {
		return nil
	}

	pageOrientation := cdd.PageOrientation{}
	for _, orientation := range append([]string{"auto"}, orientationSupported...) {
		if po, exists := pageOrientationByValue[orientation]; exists {
			option := cdd.PageOrientationOption{
				Type:      po,
				IsDefault: orientation == orientationDefault[0],
			}
			pageOrientation.Option = append(pageOrientation.Option, option)
		}
	}
	return &pageOrientation
}

func convertCopies(printerTags map[string][]string) *cdd.Copies {
	var err error
	var def int64
	if copiesDefault, exists := printerTags[attrCopiesDefault]; !exists || len(copiesDefault) != 1 {
		def = 1
	} else {
		def, err = strconv.ParseInt(copiesDefault[0], 10, 32)
		if err != nil {
			def = 1
		}
	}

	var max int64
	if copiesSupported, exists := printerTags[attrCopiesSupported]; !exists || len(copiesSupported) != 1 {
		return nil
	} else {
		c := strings.SplitN(copiesSupported[0], "~", 2)
		max, err = strconv.ParseInt(c[1], 10, 32)
		if err != nil {
			return nil
		}
	}

	return &cdd.Copies{
		Default: int32(def),
		Max:     int32(max),
	}
}

var colorByKeyword = map[string]cdd.ColorOption{
	"auto": cdd.ColorOption{
		VendorID: fmt.Sprintf("%s%s%s", attrPrintColorMode, internalKeySeparator, "auto"),
		Type:     cdd.ColorTypeAuto,
		CustomDisplayNameLocalized: cdd.NewLocalizedString("Auto"),
	},
	"color": cdd.ColorOption{
		VendorID: fmt.Sprintf("%s%s%s", attrPrintColorMode, internalKeySeparator, "color"),
		Type:     cdd.ColorTypeStandardColor,
		CustomDisplayNameLocalized: cdd.NewLocalizedString("Color"),
	},
	"monochrome": cdd.ColorOption{
		VendorID: fmt.Sprintf("%s%s%s", attrPrintColorMode, internalKeySeparator, "monochrome"),
		Type:     cdd.ColorTypeStandardMonochrome,
		CustomDisplayNameLocalized: cdd.NewLocalizedString("Monochrome"),
	},
}

func convertColorAttrs(printerTags map[string][]string) *cdd.Color {
	colorSupported, exists := printerTags[attrPrintColorModeSupported]
	if !exists {
		return nil
	}

	colorDefault, exists := printerTags[attrPrintColorModeDefault]
	if !exists || len(colorDefault) != 1 {
		colorDefault = colorSupported[:1]
	}

	var c cdd.Color
	for _, color := range colorSupported {
		var co cdd.ColorOption
		var exists bool
		if co, exists = colorByKeyword[color]; !exists {
			co = cdd.ColorOption{
				VendorID: fmt.Sprintf("%s%s%s", attrPrintColorMode, internalKeySeparator, color),
				Type:     cdd.ColorTypeCustomColor,
				CustomDisplayNameLocalized: cdd.NewLocalizedString(color),
			}
		}
		if color == colorDefault[0] {
			co.IsDefault = true
		}
		c.Option = append(c.Option, co)
	}

	return &c
}
