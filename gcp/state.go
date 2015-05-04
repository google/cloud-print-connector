/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package gcp

import (
	"cups-connector/lib"
	"encoding/json"
	"strings"
)

// cloudDeviceState represents a CloudDeviceState message.
type cloudDeviceState struct {
	Version string              `json:"version"`
	Printer printerStateSection `json:"printer"`
}

// printerStateSection => CDS PrinterStateSection.
type printerStateSection struct {
	State          string          `json:"state"`
	InputTrayState *inputTrayState `json:"input_tray_state,omitempty"`
	OutputBinState *outputBinState `json:"output_bin_state,omitempty"`
	MarkerState    *markerState    `json:"marker_state,omitempty"`
	CoverState     *coverState     `json:"cover_state,omitempty"`
	MediaPathState *mediaPathState `json:"media_path_state,omitempty"`
	VendorState    *vendorState    `json:"vendor_state,omitempty"`
}

// inputTrayState => CDS InputTrayState.
type inputTrayState struct {
	Items []inputTrayStateItem `json:"item"`
}

// inputTrayStateItem => CDS InputTrayState.Item.
type inputTrayStateItem struct {
	VendorID string `json:"vendor_id"`
	// OK, EMPTY, OPEN, OFF, FAILURE.
	State         string `json:"state"`
	LevelPercent  uint8  `json:"level_percent"`
	VendorMessage string `json:"vendor_message"`
}

// outputBinState => CDS OutputBinState.
type outputBinState struct {
	Items []outputBinStateItem `json:"item"`
}

// outputBinStateItem => CDS OutputBinState.Item.
type outputBinStateItem struct {
	VendorID string `json:"vendor_id"`
	// OK, FULL, OPEN, OFF, FAILURE.
	State         string `json:"state"`
	LevelPercent  uint8  `json:"level_percent"`
	VendorMessage string `json:"vendor_message"`
}

// markerState => CDS MarkerState.
type markerState struct {
	Items []markerStateItem `json:"item"`
}

// markerStateItem => CDS MarkerState.Item.
type markerStateItem struct {
	VendorID string `json:"vendor_id"`
	// OK, EXHAUSTED, REMOVED, FAILURE.
	State         string `json:"state"`
	LevelPercent  uint8  `json:"level_percent"`
	LevelPages    uint8  `json:"level_pages,omitempty"`
	VendorMessage string `json:"vendor_message,omitempty"`
}

// coverState => CDS CoverState.
type coverState struct {
	Items []coverStateItem `json:"item"`
}

// coverStateItem => CDS CoverState.Item.
type coverStateItem struct {
	VendorID string `json:"vendor_id"`
	// OK, OPEN, FAILURE.
	State         string `json:"state"`
	VendorMessage string `json:"vendor_message"`
}

// mediaPathState => CDS MediaPathState.
type mediaPathState struct {
	Items []mediaPathStateItem `json:"item"`
}

// mediaPathStateItem => CDS MediaPathState.Item.
type mediaPathStateItem struct {
	VendorID string `json:"vendor_id"`
	// OK, MEDIA_JAM, FAILURE.
	State         string `json:"state"`
	VendorMessage string `json:"vendor_message"`
}

// vendorState => CDS VendorState.
type vendorState struct {
	Items []vendorStateItem `json:"item"`
}

// vendorStateItem => CDS VendorState.Item.
type vendorStateItem struct {
	// ERROR, WARNING, INFO.
	State       string `json:"state"`
	Description string `json:"description"`
}

// marshalSemanticState turns state and reasons into a JSON-encoded GCP CloudDeviceState message.
func marshalSemanticState(state lib.PrinterState, reasons []string, markers map[string]uint8) (string, error) {
	printerState := printerStateSection{
		State: state.GCPPrinterState(),
	}

	if len(reasons) > 0 {
		vendorStateItems := make([]vendorStateItem, 0)
		for _, reason := range reasons {
			var vendorStateType string
			var vendorState string
			if strings.HasSuffix(reason, "-error") {
				vendorStateType = "ERROR"
				vendorState = strings.TrimSuffix(reason, "-error")
			} else if strings.HasSuffix(reason, "-warning") {
				vendorStateType = "WARNING"
				vendorState = strings.TrimSuffix(reason, "-warning")
			} else if strings.HasSuffix(reason, "-report") {
				vendorStateType = "INFO"
				vendorState = strings.TrimSuffix(reason, "-report")
			} else {
				vendorStateType = "INFO"
			}

			switch vendorState {
			default:
				vendorStateItems = append(vendorStateItems, vendorStateItem{vendorStateType, reason})
			}
		}
		printerState.VendorState = &vendorState{vendorStateItems}
	}

	if len(markers) > 0 {
		markerStateItems := make([]markerStateItem, 0, len(markers))
		for name, level := range markers {
			state := "OK"
			if level < 5 {
				state = "EXHAUSTED"
			}
			markerStateItems = append(markerStateItems, markerStateItem{VendorID: name, State: state, LevelPercent: level})
		}
		printerState.MarkerState = &markerState{markerStateItems}
	}

	semanticState := cloudDeviceState{
		Version: "1.0",
		Printer: printerState,
	}

	ss, err := json.MarshalIndent(semanticState, "", "  ")
	if err != nil {
		return "", err
	}

	return string(ss), nil
}

func unmarshalSemanticState(semanticState cloudDeviceState) (lib.PrinterState, []string, map[string]uint8) {
	state := lib.PrinterStateFromGCP(semanticState.Printer.State)

	var stateReasons []string
	if semanticState.Printer.VendorState != nil {
		stateReasons = make([]string, 0, len(semanticState.Printer.VendorState.Items))
		for _, item := range semanticState.Printer.VendorState.Items {
			stateReasons = append(stateReasons, item.Description)
		}
	} else {
		stateReasons = []string{}
	}

	var markerStates map[string]uint8
	if semanticState.Printer.MarkerState != nil {
		markerStates = make(map[string]uint8, len(semanticState.Printer.MarkerState.Items))
		for _, item := range semanticState.Printer.MarkerState.Items {
			markerStates[item.VendorID] = item.LevelPercent
		}
	} else {
		markerStates = map[string]uint8{}
	}

	return state, stateReasons, markerStates
}
