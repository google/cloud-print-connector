/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package oid

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/cups-connector/cdd"
)

// Some MIB definitions from Printer-MIB (RFC 3805).
var (
	PrinterMIB                       OID = OID{1, 3, 6, 1, 2, 1, 43}
	PrinterMIBGeneral                    = append(PrinterMIB, OID{5}...)
	PrinterGeneralSerialNumber           = append(PrinterMIBGeneral, OID{1, 1, 17, 1}...)
	PrinterMIBCover                      = append(PrinterMIB, OID{6}...)
	PrinterCoverDescription              = append(PrinterMIBCover, OID{1, 1, 2, 1}...)
	PrinterCoverStatus                   = append(PrinterMIBCover, OID{1, 1, 3, 1}...)
	PrinterMIBInput                      = append(PrinterMIB, OID{8}...)
	PrinterInputMaxCapacity              = append(PrinterMIBInput, OID{2, 1, 9, 1}...)
	PrinterInputCurrentLevel             = append(PrinterMIBInput, OID{2, 1, 10, 1}...)
	PrinterInputStatus                   = append(PrinterMIBInput, OID{2, 1, 11, 1}...)
	PrinterInputName                     = append(PrinterMIBInput, OID{2, 1, 13, 1}...)
	PrinterMIBOutput                     = append(PrinterMIB, OID{9}...)
	PrinterOutputMaxCapacity             = append(PrinterMIBOutput, OID{2, 1, 4, 1}...)
	PrinterOutputRemainingCapacity       = append(PrinterMIBOutput, OID{2, 1, 5, 1}...)
	PrinterOutputStatus                  = append(PrinterMIBOutput, OID{2, 1, 6, 1}...)
	PrinterOutputName                    = append(PrinterMIBOutput, OID{2, 1, 7, 1}...)
	PrinterMIBMarker                     = append(PrinterMIB, OID{11}...)
	PrinterMarkerSuppliesClass           = append(PrinterMIBMarker, OID{1, 1, 4, 1}...)
	PrinterMarkerSuppliesType            = append(PrinterMIBMarker, OID{1, 1, 5, 1}...)
	PrinterMarkerSuppliesDescription     = append(PrinterMIBMarker, OID{1, 1, 6, 1}...)
	PrinterMarkerSuppliesSupplyUnit      = append(PrinterMIBMarker, OID{1, 1, 7, 1}...)
	PrinterMarkerSuppliesMaxCapacity     = append(PrinterMIBMarker, OID{1, 1, 8, 1}...)
	PrinterMarkerSuppliesLevel           = append(PrinterMIBMarker, OID{1, 1, 9, 1}...)
	PrinterMIBMarkerColorant             = append(PrinterMIB, OID{12}...)
	PrinterMarkerColorantValue           = append(PrinterMIBMarkerColorant, OID{1, 1, 4, 1}...)
)

// GetSerialNumber gets the printer serial number, if available.
func (vs *VariableSet) GetSerialNumber() (string, bool) {
	return vs.GetValue(PrinterGeneralSerialNumber)
}

// Printer cover status TC.
var PrinterCoverStatusTC map[string]string = map[string]string{
	"1": "other",
	"2": "unknown",
	"3": "coverOpen",
	"4": "coverClosed",
	"5": "interlockOpen",
	"6": "interlockClosed",
}

// Printer cover status to cdd.CoverStateType.
var PrinterCoverStatusToGCP map[string]cdd.CoverStateType = map[string]cdd.CoverStateType{
	"1": cdd.CoverStateFailure,
	"2": cdd.CoverStateFailure,
	"3": cdd.CoverStateOpen,
	"4": cdd.CoverStateOK,
	"5": cdd.CoverStateOpen,
	"6": cdd.CoverStateOK,
}

func (vs *VariableSet) GetCovers() (*[]cdd.Cover, *cdd.CoverState, bool) {
	descriptions := vs.GetSubtree(PrinterCoverDescription).Variables()
	statuses := vs.GetSubtree(PrinterCoverStatus).Variables()

	if len(descriptions) < 1 ||
		len(descriptions) != len(statuses) {
		return nil, nil, false
	}

	covers := make([]cdd.Cover, 0, len(descriptions))
	coverState := cdd.CoverState{make([]cdd.CoverStateItem, 0, len(statuses))}

	for i := 0; i < len(descriptions); i++ {
		index := cdd.NewSchizophrenicInt64(descriptions[i].Name[len(descriptions[i].Name)-1])
		description := descriptions[i].Value
		state := PrinterCoverStatusToGCP[statuses[i].Value]
		cover := cdd.Cover{
			VendorID: index.String(),
			Index:    index,
		}

		switch strings.ToLower(description) {
		case "cover":
			cover.Type = cdd.CoverTypeCover
		case "door":
			cover.Type = cdd.CoverTypeDoor
		default:
			cover.Type = cdd.CoverTypeCustom
			cover.CustomDisplayNameLocalized = cdd.NewLocalizedString(description)
		}
		covers = append(covers, cover)

		coverStateItem := cdd.CoverStateItem{
			VendorID: index.String(),
			State:    state,
		}
		if state == cdd.CoverStateFailure {
			coverStateItem.VendorMessage = PrinterCoverStatusTC[statuses[i].Value]
		}
		coverState.Item = append(coverState.Item, coverStateItem)
	}

	return &covers, &coverState, true
}

type PrinterSubUnitStatusTC uint8

const (
	PrinterSubUnitAvailableAndIdle         PrinterSubUnitStatusTC = 0
	PrinterSubUnitAvailableAndStandby                             = 2
	PrinterSubUnitAvailableAndActive                              = 4
	PrinterSubUnitAvailableAndBusy                                = 6
	PrinterSubUnitUnavailable                                     = 1
	PrinterSubUnitUnavailableAndOnRequest                         = 1
	PrinterSubUnitUnavailableBecauseBroken                        = 3
	PrinterSubUnitUnknown                                         = 5
	PrinterSubUnitNonCritical                                     = 8
	PrinterSubUnitCritical                                        = 16
	PrinterSubUnitOffline                                         = 32
	PrinterSubUnitTransitioning                                   = 64
)

func (vs *VariableSet) GetInputTrays() (*[]cdd.InputTrayUnit, *cdd.InputTrayState, bool) {
	levelsMax := vs.GetSubtree(PrinterInputMaxCapacity).Variables()
	levelsCurrent := vs.GetSubtree(PrinterInputCurrentLevel).Variables()
	statuses := vs.GetSubtree(PrinterInputStatus).Variables()
	names := vs.GetSubtree(PrinterInputName).Variables()

	if len(levelsMax) < 1 ||
		len(levelsMax) != len(levelsCurrent) ||
		len(levelsMax) != len(statuses) ||
		len(levelsMax) != len(names) {
		return nil, nil, false
	}

	inputTrayUnits := make([]cdd.InputTrayUnit, 0, len(statuses))
	inputTrayState := cdd.InputTrayState{make([]cdd.InputTrayStateItem, 0, len(statuses))}

	for i := 0; i < len(statuses); i++ {
		index := cdd.NewSchizophrenicInt64(statuses[i].Name[len(statuses[i].Name)-1])

		status, err := strconv.ParseUint(statuses[i].Value, 10, 8)
		if err != nil {
			return nil, nil, false
		}
		state := cdd.InputTrayStateOK
		stateMessage := []string{}
		if (status & PrinterSubUnitUnavailable) != 0 {
			stateMessage = append(stateMessage, "unavailable")
			switch status & 7 {
			case PrinterSubUnitUnavailableAndOnRequest:
				stateMessage = append(stateMessage, "on request")
			case PrinterSubUnitUnavailableBecauseBroken:
				state = cdd.InputTrayStateFailure
				stateMessage = append(stateMessage, "broken")
			case PrinterSubUnitUnknown:
				state = cdd.InputTrayStateFailure
				stateMessage = append(stateMessage, "reason unknown")
			}
		}
		if (status & PrinterSubUnitNonCritical) != 0 {
			stateMessage = append(stateMessage, "non-critical")
		}
		if (status & PrinterSubUnitCritical) != 0 {
			state = cdd.InputTrayStateFailure
			stateMessage = append(stateMessage, "critical")
		}
		if (status & PrinterSubUnitOffline) != 0 {
			state = cdd.InputTrayStateOff
			stateMessage = append(stateMessage, "offline")
		}
		inputState := cdd.InputTrayStateItem{
			VendorID:      index.String(),
			State:         state,
			VendorMessage: strings.Join(stateMessage, ", "),
		}

		levelMax, err := strconv.ParseInt(levelsMax[i].Value, 10, 32)
		if err != nil {
			return nil, nil, false
		}
		levelCurrent, err := strconv.ParseInt(levelsCurrent[i].Value, 10, 32)
		if err != nil {
			return nil, nil, false
		}
		if levelMax >= 0 && levelCurrent >= 0 {
			if levelCurrent == 0 && state == cdd.InputTrayStateOK {
				inputState.State = cdd.InputTrayStateEmpty
			}
			var levelPercent int32
			if levelMax > 0 {
				levelPercent = int32(100 * levelCurrent / levelMax)
			}
			inputState.LevelPercent = &levelPercent
		}

		if inputState.State == cdd.InputTrayStateOK ||
			inputState.State == cdd.InputTrayStateEmpty {
			// No message necessary when state says everything.
			inputState.VendorMessage = ""
		}
		inputTrayState.Item = append(inputTrayState.Item, inputState)

		inputTrayUnits = append(inputTrayUnits, cdd.InputTrayUnit{
			VendorID: index.String(),
			Type:     cdd.InputTrayUnitCustom,
			Index:    index,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(names[i].Value),
		})
	}

	return &inputTrayUnits, &inputTrayState, true
}

func (vs *VariableSet) GetOutputBins() (*[]cdd.OutputBinUnit, *cdd.OutputBinState, bool) {
	capacitiesMax := vs.GetSubtree(PrinterOutputMaxCapacity).Variables()
	capacitiesRemaining := vs.GetSubtree(PrinterOutputRemainingCapacity).Variables()
	statuses := vs.GetSubtree(PrinterOutputStatus).Variables()
	names := vs.GetSubtree(PrinterOutputName).Variables()

	if len(names) < 1 ||
		len(names) != len(capacitiesMax) ||
		len(names) != len(capacitiesRemaining) ||
		len(names) != len(statuses) {
		return nil, nil, false
	}

	outputBinUnits := make([]cdd.OutputBinUnit, 0, len(names))
	outputBinState := cdd.OutputBinState{make([]cdd.OutputBinStateItem, 0, len(names))}

	for i := 0; i < len(names); i++ {
		index := cdd.NewSchizophrenicInt64(statuses[i].Name[len(statuses[i].Name)-1])

		status, err := strconv.ParseUint(statuses[i].Value, 10, 8)
		if err != nil {
			return nil, nil, false
		}
		state := cdd.OutputBinStateOK
		stateMessage := []string{}
		if (status & PrinterSubUnitUnavailable) != 0 {
			stateMessage = append(stateMessage, "unavailable")
			switch status & 7 {
			case PrinterSubUnitUnavailableAndOnRequest:
				stateMessage = append(stateMessage, "on request")
			case PrinterSubUnitUnavailableBecauseBroken:
				state = cdd.OutputBinStateFailure
				stateMessage = append(stateMessage, "broken")
			case PrinterSubUnitUnknown:
				state = cdd.OutputBinStateFailure
				stateMessage = append(stateMessage, "reason unknown")
			}
		}
		if (status & PrinterSubUnitNonCritical) != 0 {
			stateMessage = append(stateMessage, "non-critical")
		}
		if (status & PrinterSubUnitCritical) != 0 {
			state = cdd.OutputBinStateFailure
			stateMessage = append(stateMessage, "critical")
		}
		if (status & PrinterSubUnitOffline) != 0 {
			state = cdd.OutputBinStateOff
			stateMessage = append(stateMessage, "offline")
		}
		outputState := cdd.OutputBinStateItem{
			VendorID:      index.String(),
			State:         state,
			VendorMessage: strings.Join(stateMessage, ","),
		}

		capacityMax, err := strconv.ParseInt(capacitiesMax[i].Value, 10, 32)
		if err != nil {
			return nil, nil, false
		}
		capacityRemaining, err := strconv.ParseInt(capacitiesRemaining[i].Value, 10, 32)
		if err != nil {
			return nil, nil, false
		}
		if capacityMax >= 0 && capacityRemaining >= 0 {
			if capacityRemaining == 0 && state == cdd.OutputBinStateOK {
				outputState.State = cdd.OutputBinStateFull
			}
			levelPercent := 100 - int32(100*capacityRemaining/capacityMax)
			outputState.LevelPercent = &levelPercent
		}
		outputBinState.Item = append(outputBinState.Item, outputState)

		outputBinUnits = append(outputBinUnits, cdd.OutputBinUnit{
			VendorID: index.String(),
			Type:     cdd.OutputBinUnitCustom,
			Index:    index,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(names[i].Value),
		})
	}

	return &outputBinUnits, &outputBinState, true
}

// Printer marker supplies supply unit TC.
var PrinterMarkerSuppliesSupplyUnitTC map[string]string = map[string]string{
	"1":  "other",
	"2":  "unknown",
	"3":  "tenThousandthsOfInches",
	"4":  "micrometers",
	"7":  "impressions",
	"8":  "sheets",
	"11": "hours",
	"12": "thousandthsOfOunces",
	"13": "tenthsOfGrams",
	"14": "hundrethsOfFluidOunces",
	"15": "tenthsOfMilliliters",
	"16": "feet",
	"17": "meters",
	"18": "items",
	"19": "percent",
}

// Printer marker supplies type TC.
var PrinterMarkerSuppliesTypeTC map[string]string = map[string]string{
	"1":  "other",
	"2":  "unknown",
	"3":  "toner",
	"4":  "wasteToner",
	"5":  "ink",
	"6":  "inkCartridge",
	"7":  "inkRibbon",
	"8":  "wasteInk",
	"9":  "opc",
	"10": "developer",
	"11": "fuserOil",
	"12": "solidWax",
	"13": "ribbonWax",
	"14": "wasteWax",
	"15": "fuser",
	"16": "coronaWire",
	"17": "fuserOilWick",
	"18": "cleanerUnit",
	"19": "fuserCleaningPad",
	"20": "transferUnit",
	"21": "tonerCartridge",
	"22": "fuserOiler",
	"23": "water",
	"24": "wasteWater",
	"25": "glueWaterAdditive",
	"26": "wastePaper",
	"27": "bindingSupply",
	"28": "bandingSupply",
	"29": "stitchingWire",
	"30": "shrinkWrap",
	"31": "paperWrap",
	"32": "staples",
	"33": "inserts",
	"34": "covers",
}

var PrinterMarkerSuppliesTypeToGCP map[string]cdd.MarkerType = map[string]cdd.MarkerType{
	"1":  "",
	"2":  "",
	"3":  cdd.MarkerToner,
	"4":  "",
	"5":  cdd.MarkerInk,
	"6":  cdd.MarkerInk,
	"7":  cdd.MarkerInk,
	"8":  "",
	"9":  "",
	"10": "",
	"11": "",
	"12": "",
	"13": "",
	"14": "",
	"15": "",
	"16": "",
	"17": "",
	"18": "",
	"19": "",
	"20": "",
	"21": cdd.MarkerToner,
	"22": "",
	"23": "",
	"24": "",
	"25": "",
	"26": "",
	"27": "",
	"28": "",
	"29": "",
	"30": "",
	"31": "",
	"32": cdd.MarkerStaples,
	"33": "",
	"34": "",
}

type PrinterMarkerSuppliesClassTC string

const (
	PrinterMarkerSuppliesClassOther    PrinterMarkerSuppliesClassTC = "1" // other
	PrinterMarkerSuppliesClassConsumed                              = "3" // supplyThatIsConsumed
	PrinterMarkerSuppliesClassFilled                                = "4" // receptacleThatIsFilled
)

var snmpMarkerColorToGCP map[string]cdd.MarkerColorType = map[string]cdd.MarkerColorType{
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

func (vs *VariableSet) GetMarkers() (*[]cdd.Marker, *cdd.MarkerState, *cdd.VendorState, bool) {
	classes := vs.GetSubtree(PrinterMarkerSuppliesClass).Variables()
	types := vs.GetSubtree(PrinterMarkerSuppliesType).Variables()
	descriptions := vs.GetSubtree(PrinterMarkerSuppliesDescription).Variables()
	units := vs.GetSubtree(PrinterMarkerSuppliesSupplyUnit).Variables()
	levelsMax := vs.GetSubtree(PrinterMarkerSuppliesMaxCapacity).Variables()
	levelsCurrent := vs.GetSubtree(PrinterMarkerSuppliesLevel).Variables()
	colors := vs.GetSubtree(PrinterMarkerColorantValue).Variables()

	if len(classes) < 1 ||
		len(classes) != len(types) ||
		len(classes) != len(descriptions) ||
		len(classes) != len(units) ||
		len(classes) != len(levelsMax) ||
		len(classes) != len(levelsCurrent) ||
		len(classes) != len(colors) {
		return nil, nil, nil, false
	}

	markers := []cdd.Marker{}
	markerState := cdd.MarkerState{}
	vendorState := cdd.VendorState{}

	for i := 0; i < len(classes); i++ {
		index := int64(classes[i].Name[len(classes[i].Name)-1])

		levelMax, err := strconv.ParseInt(levelsMax[i].Value, 10, 32)
		if err != nil {
			return nil, nil, nil, false
		}
		levelCurrent, err := strconv.ParseInt(levelsCurrent[i].Value, 10, 32)
		if err != nil {
			return nil, nil, nil, false
		}
		var levelPercent int32
		if levelMax > 0 {
			levelPercent = int32(100 * levelCurrent / levelMax)
		}

		if markerType, exists := PrinterMarkerSuppliesTypeToGCP[types[i].Value]; exists && markerType != "" {
			// GCP calls this a Marker.
			state := cdd.MarkerStateOK
			markerStateItem := cdd.MarkerStateItem{
				VendorID: strconv.FormatInt(index, 10),
				State:    state,
			}

			if levelMax >= 0 && levelCurrent >= 0 {
				if levelPercent <= 10 {
					markerStateItem.State = cdd.MarkerStateExhausted
				}
				markerStateItem.LevelPercent = &levelPercent
				if unit, exists := PrinterMarkerSuppliesSupplyUnitTC[units[i].Value]; exists && unit == "sheets" {
					levelPages := int32(levelCurrent)
					markerStateItem.LevelPages = &levelPages
				}
			}

			rawColor := strings.Replace(strings.Replace(strings.ToLower(colors[i].Value), " ", "", -1), "-", "", -1)
			colorType := cdd.MarkerColorCustom
			for k, v := range snmpMarkerColorToGCP {
				if strings.HasPrefix(rawColor, k) {
					colorType = v
					break
				}
			}
			markerColor := cdd.MarkerColor{Type: colorType}
			if colorType == cdd.MarkerColorCustom {
				name := colors[i].Value
				name = strings.TrimSuffix(name, " Cartridge")
				name = strings.TrimSuffix(name, " cartridge")
				name = strings.TrimSuffix(name, " Ribbon")
				name = strings.TrimSuffix(name, " ribbon")
				name = strings.TrimSuffix(name, " Toner")
				name = strings.TrimSuffix(name, " toner")
				name = strings.TrimSuffix(name, " Ink")
				name = strings.TrimSuffix(name, " ink")
				name = strings.Replace(name, "-", " ", -1)
				markerColor.CustomDisplayNameLocalized = cdd.NewLocalizedString(name)
			}

			marker := cdd.Marker{
				VendorID: strconv.FormatInt(index, 10),
				Type:     markerType,
				Color:    &markerColor,
			}

			markerState.Item = append(markerState.Item, markerStateItem)
			markers = append(markers, marker)

		} else {
			var state cdd.VendorStateType
			if levelPercent <= 1 {
				state = cdd.VendorStateError
			} else if levelPercent <= 10 {
				state = cdd.VendorStateWarning
			} else {
				state = cdd.VendorStateInfo
			}

			// GCP doesn't call this a Marker, so treat it like a VendorState.
			class := PrinterMarkerSuppliesClassTC(classes[i].Value)
			var description string
			if class == PrinterMarkerSuppliesClassFilled {
				levelPercent = 100 - levelPercent
				description = fmt.Sprintf("%s at %d%%", descriptions[i].Value, levelPercent)
				if levelPercent == 100 {
					description = fmt.Sprintf("%s full", descriptions[i].Value)
				}
			} else { // class == PrinterMarkerSuppliesClassConsumed
				description = fmt.Sprintf("%s at %d%%", descriptions[i].Value, levelPercent)
				if levelPercent == 0 {
					description = fmt.Sprintf("%s empty", descriptions[i].Value)
				}
			}

			vendorState.Item = append(vendorState.Item, cdd.VendorStateItem{
				State:                state,
				DescriptionLocalized: cdd.NewLocalizedString(description),
			})
		}
	}

	return &markers, &markerState, &vendorState, true
}
