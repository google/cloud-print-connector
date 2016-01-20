/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/log"
)

// Jobs expire after this much time.
const jobLifetime = time.Hour

type entry struct {
	jobID     string
	ticket    *cdd.CloudJobTicket
	expiresAt time.Time

	state        cdd.JobState
	pagesPrinted *int32

	jobName string
	jobType string
	jobSize int64

	timer *time.Timer
}

func newEntry(jobID string, ticket *cdd.CloudJobTicket) *entry {
	var state cdd.JobState
	if ticket == nil {
		state.Type = cdd.JobStateDraft
	} else {
		state.Type = cdd.JobStateQueued
	}
	entry := entry{
		jobID:     jobID,
		ticket:    ticket,
		expiresAt: time.Now().Add(jobLifetime),
		state:     state,
	}

	return &entry
}

func (e *entry) expiresIn() int32 {
	i := int32(e.expiresAt.Sub(time.Now()).Seconds())
	if i < 0 {
		return 0
	}
	return i
}

type jobCache struct {
	nextJobID    int64
	nextJobMutex sync.Mutex
	entries      map[string]entry
	entriesMutex sync.RWMutex
}

func newJobCache() *jobCache {
	return &jobCache{
		nextJobID: time.Now().UnixNano(),
		entries:   make(map[string]entry),
	}
}

func (jc *jobCache) getNextJobID() string {
	jc.nextJobMutex.Lock()
	defer jc.nextJobMutex.Unlock()

	jc.nextJobID += 1
	return strconv.FormatInt(jc.nextJobID, 36)
}

// createJob creates a new job, returns the new jobID and expires_in value.
func (jc *jobCache) createJob(ticket *cdd.CloudJobTicket) (string, int32) {
	jobID := jc.getNextJobID()
	entry := newEntry(jobID, ticket)

	jc.entriesMutex.Lock()
	defer jc.entriesMutex.Unlock()

	entry.timer = time.AfterFunc(jobLifetime, func() {
		jc.deleteJob(jobID)
	})
	jc.entries[jobID] = *entry

	return entry.jobID, int32(jobLifetime.Seconds())
}

func (jc *jobCache) getJobExpiresIn(jobID string) (int32, *cdd.CloudJobTicket, bool) {
	jc.entriesMutex.RLock()
	defer jc.entriesMutex.RUnlock()

	if entry, ok := jc.entries[jobID]; !ok {
		return 0, nil, false
	} else {
		return entry.expiresIn(), entry.ticket, true
	}
}

func (jc *jobCache) submitJob(jobID, jobName, jobType string, jobSize int64) int32 {
	jc.entriesMutex.Lock()
	defer jc.entriesMutex.Unlock()

	if entry, ok := jc.entries[jobID]; ok {
		entry.jobName = jobName
		entry.jobType = jobType
		entry.jobSize = jobSize
		jc.entries[jobID] = entry
		return entry.expiresIn()
	}

	return 0
}

func (jc *jobCache) deleteJob(jobID string) {
	jc.entriesMutex.Lock()
	defer jc.entriesMutex.Unlock()

	if entry, exists := jc.entries[jobID]; exists {
		// In case this job was deleted early, cancel the timer.
		entry.timer.Stop()
	}

	delete(jc.entries, jobID)
}

func (jc *jobCache) updateJob(jobID string, stateDiff *cdd.PrintJobStateDiff) error {
	jc.entriesMutex.Lock()
	defer jc.entriesMutex.Unlock()

	if entry, ok := jc.entries[jobID]; ok {
		if stateDiff.State != nil {
			entry.state = *stateDiff.State
		}
		if stateDiff.PagesPrinted != nil {
			entry.pagesPrinted = stateDiff.PagesPrinted
		}
		jc.entries[jobID] = entry
	}

	return nil
}

// jobState gets the state of the job identified by jobID as JSON-encoded response.
//
// Returns an empty byte array if the job doesn't exist (because it expired).
func (jc *jobCache) jobState(jobID string) ([]byte, bool) {
	jc.entriesMutex.Lock()
	defer jc.entriesMutex.Unlock()

	entry, exists := jc.entries[jobID]
	if !exists {
		return []byte{}, false
	}

	var response struct {
		JobID         string            `json:"job_id"`
		State         cdd.JobStateType  `json:"state"`
		ExpiresIn     int32             `json:"expires_in"`
		JobType       string            `json:"job_type,omitempty"`
		JobSize       int64             `json:"job_size,omitempty"`
		JobName       string            `json:"job_name,omitempty"`
		SemanticState cdd.PrintJobState `json:"semantic_state"`
	}

	response.JobID = jobID
	response.State = entry.state.Type
	response.ExpiresIn = entry.expiresIn()
	response.JobType = entry.jobType
	response.JobSize = entry.jobSize
	response.JobName = entry.jobName
	response.SemanticState.Version = "1.0"
	response.SemanticState.State = entry.state
	response.SemanticState.PagesPrinted = entry.pagesPrinted

	j, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal Privet jobState: %s", err)
		return []byte{}, false
	}

	return j, true
}
