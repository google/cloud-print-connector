/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cdd

type CloudConnectionStateType string

const (
	CloudConnectionStateUnknown       CloudConnectionStateType = "UNKNOWN"
	CloudConnectionStateNotConfigured CloudConnectionStateType = "NOT_CONFIGURED"
	CloudConnectionStateOnline        CloudConnectionStateType = "ONLINE"
	CloudConnectionStateOffline       CloudConnectionStateType = "OFFLINE"
)

type CloudDeviceState struct {
	Version              string                    `json:"version"`
	CloudConnectionState *CloudConnectionStateType `json:"cloud_connection_state,omitempty"`
	Printer              *PrinterStateSection      `json:"printer"`
}

type CloudDeviceStateType string

const (
	CloudDeviceStateIdle       CloudDeviceStateType = "IDLE"
	CloudDeviceStateProcessing CloudDeviceStateType = "PROCESSING"
	CloudDeviceStateStopped    CloudDeviceStateType = "STOPPED"
)

type PrinterStateSection struct {
	State          CloudDeviceStateType `json:"state"`
	InputTrayState *InputTrayState      `json:"input_tray_state,omitempty"`
	OutputBinState *OutputBinState      `json:"output_bin_state,omitempty"`
	MarkerState    *MarkerState         `json:"marker_state,omitempty"`
	CoverState     *CoverState          `json:"cover_state,omitempty"`
	MediaPathState *MediaPathState      `json:"media_path_state,omitempty"`
	VendorState    *VendorState         `json:"vendor_state,omitempty"`
}

type InputTrayState struct {
	Item []InputTrayStateItem `json:"item"`
}

type InputTrayStateType string

const (
	InputTrayStateOK      InputTrayStateType = "OK"
	InputTrayStateEmpty   InputTrayStateType = "EMPTY"
	InputTrayStateOpen    InputTrayStateType = "OPEN"
	InputTrayStateOff     InputTrayStateType = "OFF"
	InputTrayStateFailure InputTrayStateType = "FAILURE"
)

type InputTrayStateItem struct {
	VendorID      string             `json:"vendor_id"`
	State         InputTrayStateType `json:"state"`
	LevelPercent  *int32             `json:"level_percent,omitempty"`
	VendorMessage string             `json:"vendor_message,omitempty"`
}

type OutputBinState struct {
	Item []OutputBinStateItem `json:"item"`
}

type OutputBinStateType string

const (
	OutputBinStateOK      OutputBinStateType = "OK"
	OutputBinStateFull    OutputBinStateType = "FULL"
	OutputBinStateOpen    OutputBinStateType = "OPEN"
	OutputBinStateOff     OutputBinStateType = "OFF"
	OutputBinStateFailure OutputBinStateType = "FAILURE"
)

type OutputBinStateItem struct {
	VendorID      string             `json:"vendor_id"`
	State         OutputBinStateType `json:"state"`
	LevelPercent  *int32             `json:"level_percent,omitempty"`
	VendorMessage string             `json:"vendor_message,omitempty"`
}

type MarkerState struct {
	Item []MarkerStateItem `json:"item"`
}

type MarkerStateType string

const (
	MarkerStateOK        MarkerStateType = "OK"
	MarkerStateExhausted MarkerStateType = "EXHAUSTED"
	MarkerStateRemoved   MarkerStateType = "REMOVED"
	MarkerStateFailure   MarkerStateType = "FAILURE"
)

type MarkerStateItem struct {
	VendorID      string          `json:"vendor_id"`
	State         MarkerStateType `json:"state"`
	LevelPercent  *int32          `json:"level_percent,omitempty"`
	LevelPages    *int32          `json:"level_pages,omitempty"`
	VendorMessage string          `json:"vendor_message,omitempty"`
}

type CoverState struct {
	Item []CoverStateItem `json:"item"`
}

type CoverStateType string

const (
	CoverStateOK      CoverStateType = "OK"
	CoverStateOpen    CoverStateType = "OPEN"
	CoverStateFailure CoverStateType = "FAILURE"
)

type CoverStateItem struct {
	VendorID      string         `json:"vendor_id"`
	State         CoverStateType `json:"state"`
	VendorMessage string         `json:"vendor_message,omitempty"`
}

type MediaPathState struct {
	Item []MediaPathStateItem `json:"item"`
}

type MediaPathStateType string

const (
	MediaPathStateOK       MediaPathStateType = "OK"
	MediaPathStateMediaJam MediaPathStateType = "MEDIA_JAM"
	MediaPathStateFailure  MediaPathStateType = "FAILURE"
)

type MediaPathStateItem struct {
	VendorID      string             `json:"vendor_id"`
	State         MediaPathStateType `json:"state"`
	VendorMessage string             `json:"vendor_message,omitempty"`
}

type VendorState struct {
	Item []VendorStateItem `json:"item"`
}

type VendorStateType string

const (
	VendorStateError   VendorStateType = "ERROR"
	VendorStateWarning VendorStateType = "WARNING"
	VendorStateInfo    VendorStateType = "INFO"
)

type VendorStateItem struct {
	State                VendorStateType    `json:"state"`
	Description          string             `json:"description,omitempty"`
	DescriptionLocalized *[]LocalizedString `json:"description_localized,omitempty"`
}
