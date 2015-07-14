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

type JobStateType string

const (
	JobStateDraft      JobStateType = "DRAFT"
	JobStateHeld       JobStateType = "HELD"
	JobStateQueued     JobStateType = "QUEUED"
	JobStateInProgress JobStateType = "IN_PROGRESS"
	JobStateStopped    JobStateType = "STOPPED"
	JobStateDone       JobStateType = "DONE"
	JobStateAborted    JobStateType = "ABORTED"
)

type JobState struct {
	Type               JobStateType        `json:"type"`
	UserActionCause    *UserActionCause    `json:"user_action_cause,omitempty"`
	DeviceStateCause   *DeviceStateCause   `json:"device_state_cause,omitempty"`
	DeviceActionCause  *DeviceActionCause  `json:"device_action_cause,omitempty"`
	ServiceActionCause *ServiceActionCause `json:"service_action_cause,omitempty"`
}

type UserActionCauseCode string

const (
	UserActionCauseCanceled UserActionCauseCode = "CANCELLED" // Two L's
	UserActionCausePaused   UserActionCauseCode = "PAUSED"
	UserActionCauseOther    UserActionCauseCode = "OTHER"
)

type UserActionCause struct {
	ActionCode UserActionCauseCode `json:"action_code"`
}

type DeviceStateCauseCode string

const (
	DeviceStateCauseInputTray DeviceStateCauseCode = "INPUT_TRAY"
	DeviceStateCauseMarker    DeviceStateCauseCode = "MARKER"
	DeviceStateCauseMediaPath DeviceStateCauseCode = "MEDIA_PATH"
	DeviceStateCauseMediaSize DeviceStateCauseCode = "MEDIA_SIZE"
	DeviceStateCauseMediaType DeviceStateCauseCode = "MEDIA_TYPE"
	DeviceStateCauseOther     DeviceStateCauseCode = "OTHER"
)

type DeviceStateCause struct {
	ErrorCode DeviceStateCauseCode `json:"error_code"`
}

type DeviceActionCauseCode string

const (
	DeviceActionCauseDownloadFailure DeviceActionCauseCode = "DOWNLOAD_FAILURE"
	DeviceActionCauseInvalidTicket   DeviceActionCauseCode = "INVALID_TICKET"
	DeviceActionCausePrintFailure    DeviceActionCauseCode = "PRINT_FAILURE"
	DeviceActionCauseOther           DeviceActionCauseCode = "OTHER"
)

type DeviceActionCause struct {
	ErrorCode DeviceActionCauseCode `json:"error_code"`
}

type ServiceActionCauseCode string

const (
	ServiceActionCauseCommunication        ServiceActionCauseCode = "COMMUNICATION_WITH_DEVICE_ERROR"
	ServiceActionCauseConversionError      ServiceActionCauseCode = "CONVERSION_ERROR"
	ServiceActionCauseConversionFileTooBig ServiceActionCauseCode = "CONVERSION_FILE_TOO_BIG"
	ServiceActionCauseConversionType       ServiceActionCauseCode = "CONVERSION_UNSUPPORTED_CONTENT_TYPE"
	ServiceActionCauseDeliveryFailure      ServiceActionCauseCode = "DELIVERY_FAILURE"
	ServiceActionCauseExpiration           ServiceActionCauseCode = "EXPIRATION"
	ServiceActionCauseFetchForbidden       ServiceActionCauseCode = "FETCH_DOCUMENT_FORBIDDEN"
	ServiceActionCauseFetchNotFound        ServiceActionCauseCode = "FETCH_DOCUMENT_NOT_FOUND"
	ServiceActionCauseDriveQuota           ServiceActionCauseCode = "GOOGLE_DRIVE_QUOTA"
	ServiceActionCauseInconsistentJob      ServiceActionCauseCode = "INCONSISTENT_JOB"
	ServiceActionCauseInconsistentPrinter  ServiceActionCauseCode = "INCONSISTENT_PRINTER"
	ServiceActionCausePrinterDeleted       ServiceActionCauseCode = "PRINTER_DELETED"
	ServiceActionCauseRemoteJobNoExist     ServiceActionCauseCode = "REMOTE_JOB_NO_LONGER_EXISTS"
	ServiceActionCauseRemoteJobError       ServiceActionCauseCode = "REMOTE_JOB_ERROR"
	ServiceActionCauseRemoteJobTimeout     ServiceActionCauseCode = "REMOTE_JOB_TIMEOUT"
	ServiceActionCauseRemoteJobAborted     ServiceActionCauseCode = "REMOTE_JOB_ABORTED"
	ServiceActionCauseOther                ServiceActionCauseCode = "OTHER"
)

type ServiceActionCause struct {
	ErrorCode ServiceActionCauseCode `json:"error_code"`
}
