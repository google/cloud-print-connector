/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

type JobStatus uint8

func JobStatusFromInt(si uint8) JobStatus {
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

func (js JobStatus) GCPStatus() string {
	switch js {
	case 3, 4, 5:
		return "IN_PROGRESS"
	case 6, 7, 8:
		return "ERROR"
	case 9:
		return "DONE"
	default:
		panic("unreachable")
	}
}

// CUPS: ipp_jstate_t; GCP: Legacy Job Status; not 1:1
const (
	JobPending    JobStatus = 3 // CUPS: IPP_JSTATE_PENDING;    GCP: IN_PROGRESS
	JobHeld       JobStatus = 4 // CUPS: IPP_JSTATE_HELD;       GCP: IN_PROGRESS
	JobProcessing JobStatus = 5 // CUPS: IPP_JSTATE_PROCESSING; GCP: IN_PROGRESS
	JobStopped    JobStatus = 6 // CUPS: IPP_JSTATE_STOPPED;    GCP: ERROR
	JobCanceled   JobStatus = 7 // CUPS: IPP_JSTATE_CANCELED;   GCP: ERROR
	JobAborted    JobStatus = 8 // CUPS: IPP_JSTATE_ABORTED;    GCP: ERROR
	JobCompleted  JobStatus = 9 // CUPS: IPP_JSTATE_COMPLETED;  GCP: DONE
)

type Job struct {
	GCPPrinterID string
	GCPJobID     string
	FileURL      string
}
