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

// TODO
// marker (tonerWaste can go to VendorCapability with RangeType)

// Some MIB definitions from Printer-MIB (RFC 3805).
var (
	PrinterMIB                       OID = OID{1, 3, 6, 1, 2, 1, 43}
	PrinterGeneralSerialNumber           = append(PrinterMIB, OID{5, 1, 1, 17, 1}...)
	PrinterCoverDescription              = append(PrinterMIB, OID{6, 1, 1, 2, 1}...)
	PrinterCoverStatus                   = append(PrinterMIB, OID{6, 1, 1, 3, 1}...)
	PrinterInputMaxCapacity              = append(PrinterMIB, OID{8, 2, 1, 9, 1}...)
	PrinterInputCurrentLevel             = append(PrinterMIB, OID{8, 2, 1, 10, 1}...)
	PrinterInputStatus                   = append(PrinterMIB, OID{8, 2, 1, 11, 1}...)
	PrinterInputName                     = append(PrinterMIB, OID{8, 2, 1, 13, 1}...)
	PrinterOutputMaxCapacity             = append(PrinterMIB, OID{9, 2, 1, 4, 1}...)
	PrinterOutputRemainingCapacity       = append(PrinterMIB, OID{9, 2, 1, 5, 1}...)
	PrinterOutputStatus                  = append(PrinterMIB, OID{9, 2, 1, 6, 1}...)
	PrinterOutputName                    = append(PrinterMIB, OID{9, 2, 1, 7, 1}...)
	PrinterMarkerStatus                  = append(PrinterMIB, OID{10, 2, 1, 15, 1}...)
	PrinterMarkerSuppliesClass           = append(PrinterMIB, OID{11, 1, 1, 4, 1}...)
	PrinterMarkerSuppliesType            = append(PrinterMIB, OID{11, 1, 1, 5, 1}...)
	PrinterMarkerSuppliesDescription     = append(PrinterMIB, OID{11, 1, 1, 6, 1}...)
	PrinterMarkerSuppliesSupplyUnit      = append(PrinterMIB, OID{11, 1, 1, 7, 1}...)
	PrinterMarkerSuppliesMaxCapacity     = append(PrinterMIB, OID{11, 1, 1, 8, 1}...)
	PrinterMarkerSuppliesLevel           = append(PrinterMIB, OID{11, 1, 1, 9, 1}...)
	PrinterMarkerColorantValue           = append(PrinterMIB, OID{12, 1, 1, 4, 1}...)
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

func (vs *VariableSet) GetCovers() ([]cdd.Cover, *cdd.CoverState, bool) {
	descriptions := vs.GetSubtree(PrinterCoverDescription).Variables()
	statuses := vs.GetSubtree(PrinterCoverStatus).Variables()

	if len(descriptions) < 1 ||
		len(descriptions) != len(statuses) {
		return []cdd.Cover{}, nil, false
	}

	covers := make([]cdd.Cover, 0, len(descriptions))
	coverState := cdd.CoverState{make([]cdd.CoverStateItem, 0, len(statuses))}

	for i := 0; i < len(descriptions); i++ {
		index := int64(descriptions[i].Name[len(descriptions[i].Name)-1])
		description := descriptions[i].Value
		state := PrinterCoverStatusToGCP[statuses[i].Value]
		cover := cdd.Cover{
			VendorID: strconv.FormatInt(index, 10),
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
			VendorID: strconv.FormatInt(index, 10),
			State:    state,
		}
		if state == cdd.CoverStateFailure {
			coverStateItem.VendorMessage = PrinterCoverStatusTC[statuses[i].Value]
		}
		coverState.Item = append(coverState.Item, coverStateItem)
	}

	return covers, &coverState, true
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

func (vs *VariableSet) GetInputTrays() ([]cdd.InputTrayUnit, *cdd.InputTrayState, bool) {
	levelsMax := vs.GetSubtree(PrinterInputMaxCapacity).Variables()
	levelsCurrent := vs.GetSubtree(PrinterInputCurrentLevel).Variables()
	statuses := vs.GetSubtree(PrinterInputStatus).Variables()
	names := vs.GetSubtree(PrinterInputName).Variables()

	if len(levelsMax) < 1 ||
		len(levelsMax) != len(levelsCurrent) ||
		len(levelsMax) != len(statuses) ||
		len(levelsMax) != len(names) {
		return []cdd.InputTrayUnit{}, nil, false
	}

	inputTrayUnits := make([]cdd.InputTrayUnit, 0, len(statuses))
	inputTrayState := cdd.InputTrayState{make([]cdd.InputTrayStateItem, 0, len(statuses))}

	for i := 0; i < len(statuses); i++ {
		index := int64(statuses[i].Name[len(statuses[i].Name)-1])

		status, err := strconv.ParseUint(statuses[i].Value, 10, 8)
		if err != nil {
			return []cdd.InputTrayUnit{}, nil, false
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
			VendorID:      strconv.FormatInt(index, 10),
			State:         state,
			VendorMessage: strings.Join(stateMessage, ","),
		}

		levelMax, err := strconv.ParseInt(levelsMax[i].Value, 10, 32)
		if err != nil {
			return []cdd.InputTrayUnit{}, nil, false
		}
		levelCurrent, err := strconv.ParseInt(levelsCurrent[i].Value, 10, 32)
		if err != nil {
			return []cdd.InputTrayUnit{}, nil, false
		}
		if levelMax >= 0 && levelCurrent >= 0 {
			if levelCurrent == 0 && state == cdd.InputTrayStateOK {
				inputState.State = cdd.InputTrayStateEmpty
			}
			levelPercent := int32(100 * levelCurrent / levelMax)
			inputState.LevelPercent = &levelPercent
		}
		inputTrayState.Item = append(inputTrayState.Item, inputState)

		inputTrayUnits = append(inputTrayUnits, cdd.InputTrayUnit{
			VendorID: strconv.FormatInt(index, 10),
			Type:     cdd.InputTrayUnitCustom,
			Index:    index,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(names[i].Value),
		})
	}

	return inputTrayUnits, &inputTrayState, true
}

func (vs *VariableSet) GetOutputBins() ([]cdd.OutputBinUnit, *cdd.OutputBinState, bool) {
	capacitiesMax := vs.GetSubtree(PrinterOutputMaxCapacity).Variables()
	capacitiesRemaining := vs.GetSubtree(PrinterOutputRemainingCapacity).Variables()
	statuses := vs.GetSubtree(PrinterOutputStatus).Variables()
	names := vs.GetSubtree(PrinterOutputName).Variables()

	if len(names) < 1 ||
		len(names) != len(capacitiesMax) ||
		len(names) != len(capacitiesRemaining) ||
		len(names) != len(statuses) {
		return []cdd.OutputBinUnit{}, nil, false
	}

	outputBinUnits := make([]cdd.OutputBinUnit, 0, len(names))
	outputBinState := cdd.OutputBinState{make([]cdd.OutputBinStateItem, 0, len(names))}

	for i := 0; i < len(names); i++ {
		index := int64(statuses[i].Name[len(statuses[i].Name)-1])

		status, err := strconv.ParseUint(statuses[i].Value, 10, 8)
		if err != nil {
			return []cdd.OutputBinUnit{}, nil, false
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
			VendorID:      strconv.FormatInt(index, 10),
			State:         state,
			VendorMessage: strings.Join(stateMessage, ","),
		}

		capacityMax, err := strconv.ParseInt(capacitiesMax[i].Value, 10, 32)
		if err != nil {
			return []cdd.OutputBinUnit{}, nil, false
		}
		capacityRemaining, err := strconv.ParseInt(capacitiesRemaining[i].Value, 10, 32)
		if err != nil {
			return []cdd.OutputBinUnit{}, nil, false
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
			VendorID: strconv.FormatInt(index, 10),
			Type:     cdd.OutputBinUnitCustom,
			Index:    index,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(names[i].Value),
		})
	}

	return outputBinUnits, &outputBinState, true
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

func (vs *VariableSet) GetMarkers() ([]cdd.Marker, *cdd.MarkerState, *cdd.VendorState, bool) {
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
		return []cdd.Marker{}, nil, nil, false
	}

	markers := []cdd.Marker{}
	markerState := cdd.MarkerState{}
	vendorState := cdd.VendorState{}

	for i := 0; i < len(classes); i++ {
		index := int64(classes[i].Name[len(classes[i].Name)-1])

		levelMax, err := strconv.ParseInt(levelsMax[i].Value, 10, 32)
		if err != nil {
			return []cdd.Marker{}, nil, nil, false
		}
		levelCurrent, err := strconv.ParseInt(levelsCurrent[i].Value, 10, 32)
		if err != nil {
			return []cdd.Marker{}, nil, nil, false
		}
		levelPercent := int32(100 * levelCurrent / levelMax)

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

			marker := cdd.Marker{
				VendorID: strconv.FormatInt(index, 10),
				Type:     markerType,
				Color: &cdd.MarkerColor{
					Type: cdd.MarkerColorCustom,
					CustomDisplayNameLocalized: cdd.NewLocalizedString(colors[i].Value),
				},
			}

			markerState.Item = append(markerState.Item, markerStateItem)
			markers = append(markers, marker)

		} else {
			// GCP doesn't call this a Marker, so treat it like a VendorState.
			class := PrinterMarkerSuppliesClassTC(classes[i].Value)
			if levelPercent <= 10 && class != PrinterMarkerSuppliesClassOther {
				var description string
				if class == PrinterMarkerSuppliesClassFilled {
					levelPercent = 100 - levelPercent
					description = fmt.Sprintf("%s at %d%%", descriptions[i].Value)
					if levelPercent == 100 {
						description = fmt.Sprintf("%s full", descriptions[i].Value)
					}
				} else { // class == PrinterMarkerSuppliesClassConsumed
					description = fmt.Sprintf("%s at %d%%", descriptions[i].Value)
					if levelPercent == 0 {
						description = fmt.Sprintf("%s empty", descriptions[i].Value)
					}
				}
				vendorState.Item = append(vendorState.Item, cdd.VendorStateItem{
					State:                cdd.VendorStateError,
					DescriptionLocalized: cdd.NewLocalizedString(description),
				})
			}
		}
	}

	return markers, &markerState, &vendorState, true
}
