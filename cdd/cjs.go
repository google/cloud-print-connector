/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cdd

type PrintJobState struct {
	Version          string   `json:"version"`
	State            JobState `json:"state"`
	PagesPrinted     int32    `json:"pages_printed,omitempty"`
	DeliveryAttempts int32    `json:"delivery_attempts,omitempty"`
}

type PrintJobStateDiff struct {
	State        JobState `json:"state,omitempty"`
	PagesPrinted int32    `json:"pages_printed,omitempty"`
}

type JobState struct {
	Type               string              `json:"type"` // enum
	UserActionCause    *UserActionCause    `json:"user_action_cause,omitempty"`
	DeviceStateCause   *DeviceStateCause   `json:"device_state_cause,omitempty"`
	DeviceActionCause  *DeviceActionCause  `json:"device_action_cause,omitempty"`
	ServiceActionCause *ServiceActionCause `json:"service_action_cause,omitempty"`
}

type UserActionCause struct {
	ActionCode string `json:"action_code"` // enum
}

type DeviceStateCause struct {
	ErrorCode string `json:"error_code"` // enum
}

type DeviceActionCause struct {
	ErrorCode string `json:"error_code"` // enum
}

type ServiceActionCause struct {
	ErrorCode string `json:"error_code"` // enum
}
