/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

type (
	CUPSJobState     uint8
	GCPJobState      uint8
	GCPJobStateCause uint8
)

// CUPS: ipp_jstate_t; GCP: CJS JobState.Type; not 1:1
const (
	CUPSJobPending    CUPSJobState = iota // CUPS: IPP_JSTATE_PENDING;    GCP: DRAFT
	CUPSJobHeld                    = iota // CUPS: IPP_JSTATE_HELD;       GCP: HELD
	CUPSJobProcessing              = iota // CUPS: IPP_JSTATE_PROCESSING; GCP: IN_PROGRESS
	CUPSJobStopped                 = iota // CUPS: IPP_JSTATE_STOPPED;    GCP: STOPPED
	CUPSJobCanceled                = iota // CUPS: IPP_JSTATE_CANCELED;   GCP: ABORTED
	CUPSJobAborted                 = iota // CUPS: IPP_JSTATE_ABORTED;    GCP: ABORTED
	CUPSJobCompleted               = iota // CUPS: IPP_JSTATE_COMPLETED;  GCP: DONE

	GCPJobDraft      GCPJobState = iota
	GCPJobHeld                   = iota
	GCPJobQueued                 = iota
	GCPJobInProgress             = iota
	GCPJobStopped                = iota
	GCPJobDone                   = iota
	GCPJobAborted                = iota

	// GCP: CJS JobState.DeviceActionCause
	GCPJobDownloadFailure GCPJobStateCause = iota
	GCPJobInvalidTicket                    = iota
	GCPJobPrintFailure                     = iota
	GCPJobOther                            = iota
	// GCP: CJS JobState.UserActionCause
	GCPJobCanceled = iota
)

func (js CUPSJobState) String() string {
	switch js {
	case CUPSJobPending:
		return "PENDING"
	case CUPSJobHeld:
		return "HELD"
	case CUPSJobProcessing:
		return "PROCESSING"
	case CUPSJobStopped:
		return "STOPPED"
	case CUPSJobCanceled:
		return "CANCELED"
	case CUPSJobAborted:
		return "ABORTED"
	case CUPSJobCompleted:
		return "COMPLETED"
	}
	panic("unreachable")
}

// CUPSJobStateFromInt converts an integer to a CUPSJobState.
func CUPSJobStateFromInt(si uint8) CUPSJobState {
	switch si {
	case 3:
		return CUPSJobPending
	case 4:
		return CUPSJobHeld
	case 5:
		return CUPSJobProcessing
	case 6:
		return CUPSJobStopped
	case 7:
		return CUPSJobCanceled
	case 8:
		return CUPSJobAborted
	case 9:
		return CUPSJobCompleted
	}
	panic("unreachable")
}

// GCPJobState converts this CUPSJobState object to a GCPJobState object.
func (js CUPSJobState) GCPJobState() (GCPJobState, GCPJobStateCause) {
	switch js {
	case CUPSJobPending:
		return GCPJobDraft, 100
	case CUPSJobHeld:
		return GCPJobHeld, 100
	case CUPSJobProcessing:
		return GCPJobInProgress, 100
	case CUPSJobStopped:
		return GCPJobStopped, GCPJobOther
	case CUPSJobCompleted:
		return GCPJobDone, 100
	case CUPSJobAborted:
		return GCPJobAborted, GCPJobPrintFailure
	case CUPSJobCanceled:
		// GCP doesn't have a canceled state, but it does have a canceled cause.
		return GCPJobAborted, GCPJobCanceled
	}
	panic("unreachable")
}

func (js GCPJobState) String() string {
	switch js {
	case GCPJobDraft:
		return "DRAFT"
	case GCPJobHeld:
		return "HELD"
	case GCPJobQueued:
		return "QUEUED"
	case GCPJobInProgress:
		return "IN_PROGRESS"
	case GCPJobStopped:
		return "STOPPED"
	case GCPJobDone:
		return "DONE"
	case GCPJobAborted:
		return "ABORTED"
	}
	panic("unreachable")
}

func (jsc GCPJobStateCause) String() string {
	switch jsc {
	case GCPJobDownloadFailure:
		return "DOWNLOAD_FAILURE"
	case GCPJobInvalidTicket:
		return "INVALID_TICKET"
	case GCPJobPrintFailure:
		return "PRINT_FAILURE"
	case GCPJobOther:
		return "OTHER"
	case GCPJobCanceled:
		return "CANCELLED" // Spelled with two L's.
	}
	panic("unreachable")
}

type Job struct {
	GCPPrinterID string
	GCPJobID     string
	FileURL      string
	OwnerID      string
	Title        string
}
