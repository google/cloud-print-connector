/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

type GCPJobStatus string
type CUPSJobStatus string

// CUPSJobStatusFromInt converts an integer to a CUPSJobStatus.
func CUPSJobStatusFromInt(si uint8) CUPSJobStatus {
	switch si {
	case 3:
		return JobPending
	case 4:
		return JobHeld
	case 5:
		return JobProcessing
	case 6:
		return JobStopped
	case 7:
		return JobCanceled
	case 8:
		return JobAborted
	case 9:
		return JobCompleted
	default:
		panic("unreachable")
	}
}

// GCPJobStatus converts this CUPSJobStatus object to a GCPJobStatus object.
func (js CUPSJobStatus) GCPJobStatus() GCPJobStatus {
	switch js {
	case JobPending, JobHeld, JobProcessing:
		return JobInProgress
	case JobStopped, JobCanceled, JobAborted:
		return JobError
	case JobCompleted:
		return JobDone
	default:
		panic("unreachable")
	}
}

// CUPS: ipp_jstate_t; GCP: Legacy Job Status; not 1:1
const (
	JobPending    CUPSJobStatus = "PENDING"    // CUPS: IPP_JSTATE_PENDING;    GCP: IN_PROGRESS
	JobHeld       CUPSJobStatus = "HELD"       // CUPS: IPP_JSTATE_HELD;       GCP: IN_PROGRESS
	JobProcessing CUPSJobStatus = "PROCESSING" // CUPS: IPP_JSTATE_PROCESSING; GCP: IN_PROGRESS
	JobStopped    CUPSJobStatus = "STOPPED"    // CUPS: IPP_JSTATE_STOPPED;    GCP: ERROR
	JobCanceled   CUPSJobStatus = "CANCELED"   // CUPS: IPP_JSTATE_CANCELED;   GCP: ERROR
	JobAborted    CUPSJobStatus = "ABORTED"    // CUPS: IPP_JSTATE_ABORTED;    GCP: ERROR
	JobCompleted  CUPSJobStatus = "COMPLETED"  // CUPS: IPP_JSTATE_COMPLETED;  GCP: DONE

	JobInProgress GCPJobStatus = "IN_PROGRESS"
	JobError      GCPJobStatus = "ERROR"
	JobDone       GCPJobStatus = "DONE"
)

type Job struct {
	GCPPrinterID string
	GCPJobID     string
	FileURL      string
	OwnerID      string
	Title        string
}
