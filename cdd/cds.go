/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cdd

type CloudDeviceState struct {
	Version              string              `json:"version"`
	CloudConnectionState *string             `json:"cloud_connection_state,omitempty"` // enum
	Printer              PrinterStateSection `json:"printer"`
}

type PrinterStateSection struct {
	State          string          `json:"state"`
	InputTrayState *InputTrayState `json:"input_tray_state,omitempty"`
	OutputBinState *OutputBinState `json:"output_bin_state,omitempty"`
	MarkerState    *MarkerState    `json:"marker_state,omitempty"`
	CoverState     *CoverState     `json:"cover_state,omitempty"`
	MediaPathState *MediaPathState `json:"media_path_state,omitempty"`
	VendorState    *VendorState    `json:"vendor_state,omitempty"`
}

type InputTrayState struct {
	Item []InputTrayStateItem `json:"item"`
}

type InputTrayStateItem struct {
	VendorID      string `json:"vendor_id"`
	State         string `json:"state"` // enum
	LevelPercent  int32  `json:"level_percent"`
	VendorMessage string `json:"vendor_message"`
}

type OutputBinState struct {
	Item []OutputBinStateItem `json:"item"`
}

type OutputBinStateItem struct {
	VendorID      string `json:"vendor_id"`
	State         string `json:"state"` // enum
	LevelPercent  int32  `json:"level_percent"`
	VendorMessage string `json:"vendor_message"`
}

type MarkerState struct {
	Item []MarkerStateItem `json:"item"`
}

type MarkerStateItem struct {
	VendorID      string `json:"vendor_id"`
	State         string `json:"state"` // enum
	LevelPercent  int32  `json:"level_percent"`
	LevelPages    int32  `json:"level_pages,omitempty"`
	VendorMessage string `json:"vendor_message"`
}

type CoverState struct {
	Item []CoverStateItem `json:"item"`
}

type CoverStateItem struct {
	VendorID      string `json:"vendor_id"`
	State         string `json:"state"` // enum
	VendorMessage string `json:"vendor_message"`
}

type MediaPathState struct {
	Item []MediaPathStateItem `json:"item"`
}

type MediaPathStateItem struct {
	VendorID      string `json:"vendor_id"`
	State         string `json:"state"` // enum
	VendorMessage string `json:"vendor_message"`
}

type VendorState struct {
	Item []VendorStateItem `json:"item"`
}

type VendorStateItem struct {
	State                string             `json:"state"` // enum
	Description          string             `json:"description,omitempty"`
	DescriptionLocalized *[]LocalizedString `json:"description_localized,omitempty"`
}
