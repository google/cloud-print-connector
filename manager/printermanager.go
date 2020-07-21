/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package manager

import (
	"fmt"
	"hash/adler32"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/cloud-print-connector/cdd"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"
	"github.com/google/cloud-print-connector/notification"
	"github.com/google/cloud-print-connector/privet"
	"github.com/google/cloud-print-connector/xmpp"
)

type NativePrintSystem interface {
	GetPrinters() ([]lib.Printer, error)
	GetJobState(printerName string, jobID uint32) (*cdd.PrintJobStateDiff, error)
	Print(printer *lib.Printer, fileName, title, user, gcpJobID string, ticket *cdd.CloudJobTicket) (uint32, error)
	ReleaseJob(printerName string, jobID uint32) error
	RemoveCachedPPD(printerName string)
}

// Manages state and interactions between the native print system and Google Cloud Print.
type PrinterManager struct {
	native NativePrintSystem
	gcp    *gcp.GoogleCloudPrint
	xmpp   *xmpp.XMPP
	privet *privet.Privet

	printers *lib.ConcurrentPrinterMap

	// Job stats are numbers reported to monitoring.
	jobStatsMutex sync.Mutex
	jobsDone      uint
	jobsError     uint

	// Jobs in flight are jobs that have been received, and are not
	// finished printing yet. Key is Job ID.
	jobsInFlightMutex sync.Mutex
	jobsInFlight      map[string]struct{}

	nativeJobQueueSize uint
	jobFullUsername    bool
	shareScope         string

	quit   chan struct{}
	useFcm bool
}

func NewPrinterManager(native NativePrintSystem, gcp *gcp.GoogleCloudPrint, privet *privet.Privet, printerPollInterval time.Duration, nativeJobQueueSize uint, jobFullUsername bool, shareScope string, jobs <-chan *lib.Job, notifications <-chan notification.PrinterNotification, useFcm bool) (*PrinterManager, error) {
	var printers *lib.ConcurrentPrinterMap
	var queuedJobsCount map[string]uint

	var err error
	if gcp != nil {
		// Get all GCP printers.
		var gcpPrinters []lib.Printer
		gcpPrinters, queuedJobsCount, err = gcp.ListPrinters()
		if err != nil {
			return nil, err
		}
		// Organize the GCP printers into a map.
		for i := range gcpPrinters {
			gcpPrinters[i].NativeJobSemaphore = lib.NewSemaphore(nativeJobQueueSize)
		}
		printers = lib.NewConcurrentPrinterMap(gcpPrinters)
	} else {
		printers = lib.NewConcurrentPrinterMap(nil)
	}

	// Construct.
	pm := PrinterManager{
		native: native,
		gcp:    gcp,
		privet: privet,

		printers: printers,

		jobStatsMutex: sync.Mutex{},
		jobsDone:      0,
		jobsError:     0,

		jobsInFlightMutex: sync.Mutex{},
		jobsInFlight:      make(map[string]struct{}),

		nativeJobQueueSize: nativeJobQueueSize,
		jobFullUsername:    jobFullUsername,
		shareScope:         shareScope,

		quit:   make(chan struct{}),
		useFcm: useFcm,
	}

	// Sync once before returning, to make sure things are working.
	// Ignore privet updates this first time because Privet always starts
	// with zero printers.
	if err = pm.SyncPrinters(true); err != nil {
		return nil, err
	}

	// Initialize Privet printers.
	if privet != nil {
		for _, printer := range pm.printers.GetAll() {
			err := privet.AddPrinter(printer, pm.printers.GetByNativeName)
			if err != nil {
				log.WarningPrinterf(printer.Name, "Failed to register locally: %s", err)
			} else {
				log.InfoPrinterf(printer.Name, "Registered locally")
			}
		}
	}

	pm.syncPrintersPeriodically(printerPollInterval)
	pm.listenNotifications(jobs, notifications)

	if gcp != nil {
		for gcpPrinterID := range queuedJobsCount {
			p, _ := printers.GetByGCPID(gcpPrinterID)
			go gcp.HandleJobs(&p, func() { pm.incrementJobsProcessed(false) })
		}
	}

	return &pm, nil
}

func (pm *PrinterManager) Quit() {
	close(pm.quit)
}

func (pm *PrinterManager) syncPrintersPeriodically(interval time.Duration) {
	go func() {
		t := time.NewTimer(interval)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				if err := pm.SyncPrinters(false); err != nil {
					log.Error(err)
				}
				t.Reset(interval)

			case <-pm.quit:
				return
			}
		}
	}()
}

func (pm *PrinterManager) SyncPrinters(ignorePrivet bool) error {
	log.Debug("Synchronizing printers, stand by")

	// Get current snapshot of native printers.
	nativePrinters, err := pm.native.GetPrinters()
	if err != nil {
		return fmt.Errorf("Sync failed while calling GetPrinters(): %s", err)
	}

	// Set CapsHash on all printers.
	for i := range nativePrinters {
		h := adler32.New()
		lib.DeepHash(nativePrinters[i].Tags, h)
		nativePrinters[i].Tags["tagshash"] = fmt.Sprintf("%x", h.Sum(nil))

		h = adler32.New()
		lib.DeepHash(nativePrinters[i].Description, h)
		nativePrinters[i].CapsHash = fmt.Sprintf("%x", h.Sum(nil))

		if pm.useFcm {
			nativePrinters[i].NotificationChannel = gcp.FCP_CHANNEL
		} else {
			nativePrinters[i].NotificationChannel = gcp.XMPP_CHANNEL
		}
	}

	// Compare the snapshot to what we know currently.
	diffs := lib.DiffPrinters(nativePrinters, pm.printers.GetAll())
	if diffs == nil {
		log.Debugf("Printers are already in sync; there are %d", len(nativePrinters))
		return nil
	}

	// Update GCP.
	ch := make(chan lib.Printer, len(diffs))
	for i := range diffs {
		go pm.applyDiff(&diffs[i], ch, ignorePrivet)
	}
	currentPrinters := make([]lib.Printer, 0, len(diffs))
	for _ = range diffs {
		p := <-ch
		if p.Name != "" {
			currentPrinters = append(currentPrinters, p)
		}
	}

	// Update what we know.
	pm.printers.Refresh(currentPrinters)
	log.Debugf("Finished synchronizing %d printers", len(currentPrinters))

	return nil
}

func (pm *PrinterManager) applyDiff(diff *lib.PrinterDiff, ch chan<- lib.Printer, ignorePrivet bool) {
	switch diff.Operation {
	case lib.RegisterPrinter:
		if pm.gcp != nil {
			if err := pm.gcp.Register(&diff.Printer); err != nil {
				log.ErrorPrinterf(diff.Printer.Name, "Failed to register: %s", err)
				break
			}
			log.InfoPrinterf(diff.Printer.Name+" "+diff.Printer.GCPID, "Registered in the cloud")

			if pm.gcp.CanShare() {
				if err := pm.gcp.Share(diff.Printer.GCPID, pm.shareScope, gcp.User, true, false); err != nil {
					log.ErrorPrinterf(diff.Printer.Name, "Failed to share: %s", err)
				} else {
					log.InfoPrinterf(diff.Printer.Name, "Shared")
				}
			}
		}

		diff.Printer.NativeJobSemaphore = lib.NewSemaphore(pm.nativeJobQueueSize)

		if pm.privet != nil && !ignorePrivet {
			err := pm.privet.AddPrinter(diff.Printer, pm.printers.GetByNativeName)
			if err != nil {
				log.WarningPrinterf(diff.Printer.Name, "Failed to register locally: %s", err)
			} else {
				log.InfoPrinterf(diff.Printer.Name, "Registered locally")
			}
		}

		ch <- diff.Printer
		return

	case lib.UpdatePrinter:
		if pm.gcp != nil {
			if err := pm.gcp.Update(diff); err != nil {
				log.ErrorPrinterf(diff.Printer.Name+" "+diff.Printer.GCPID, "Failed to update: %s", err)
			} else {
				log.InfoPrinterf(diff.Printer.Name+" "+diff.Printer.GCPID, "Updated in the cloud")
			}
		}

		if pm.privet != nil && !ignorePrivet && diff.DefaultDisplayNameChanged {
			err := pm.privet.UpdatePrinter(diff)
			if err != nil {
				log.WarningPrinterf(diff.Printer.Name, "Failed to update locally: %s", err)
			} else {
				log.InfoPrinterf(diff.Printer.Name, "Updated locally")
			}
		}

		ch <- diff.Printer
		return

	case lib.DeletePrinter:
		pm.native.RemoveCachedPPD(diff.Printer.Name)

		if pm.gcp != nil {
			if err := pm.gcp.Delete(diff.Printer.GCPID); err != nil {
				log.ErrorPrinterf(diff.Printer.Name+" "+diff.Printer.GCPID, "Failed to delete from the cloud: %s", err)
				break
			}
			log.InfoPrinterf(diff.Printer.Name+" "+diff.Printer.GCPID, "Deleted from the cloud")
		}

		if pm.privet != nil && !ignorePrivet {
			err := pm.privet.DeletePrinter(diff.Printer.Name)
			if err != nil {
				log.WarningPrinterf(diff.Printer.Name, "Failed to delete: %s", err)
			} else {
				log.InfoPrinterf(diff.Printer.Name, "Deleted locally")
			}
		}

	case lib.NoChangeToPrinter:
		ch <- diff.Printer
		return
	}

	ch <- lib.Printer{}
}

// listenNotifications handles the messages found on the channels.
func (pm *PrinterManager) listenNotifications(jobs <-chan *lib.Job, messages <-chan notification.PrinterNotification) {
	go func() {
		for {
			select {
			case <-pm.quit:
				return

			case job := <-jobs:
				log.DebugJobf(job.JobID, "Received job: %+v", job)
				go pm.printJob(job.NativePrinterName, job.Filename, job.Title, job.User, job.JobID, job.Ticket, job.UpdateJob)

			case message := <-messages:
				log.Debugf("Received message: %+v", message)
				if message.Type == notification.PrinterNewJobs {
					if p, exists := pm.printers.GetByGCPID(message.GCPID); exists {
						go pm.gcp.HandleJobs(&p, func() { pm.incrementJobsProcessed(false) })
					}
				}
			}
		}
	}()
}

func (pm *PrinterManager) incrementJobsProcessed(success bool) {
	pm.jobStatsMutex.Lock()
	defer pm.jobStatsMutex.Unlock()

	if success {
		pm.jobsDone += 1
	} else {
		pm.jobsError += 1
	}
}

// addInFlightJob adds a job ID to the in flight set.
//
// Returns true if the job ID was added, false if it already exists.
func (pm *PrinterManager) addInFlightJob(jobID string) bool {
	pm.jobsInFlightMutex.Lock()
	defer pm.jobsInFlightMutex.Unlock()

	if _, exists := pm.jobsInFlight[jobID]; exists {
		return false
	}

	pm.jobsInFlight[jobID] = struct{}{}

	return true
}

// deleteInFlightJob deletes a job from the in flight set.
func (pm *PrinterManager) deleteInFlightJob(jobID string) {
	pm.jobsInFlightMutex.Lock()
	defer pm.jobsInFlightMutex.Unlock()

	delete(pm.jobsInFlight, jobID)
}

// printJob prints a new job to a native printer, then polls the native job state
// and updates the GCP/Privet job state. then returns when the job state is DONE
// or ABORTED.
//
// All errors are reported and logged from inside this function.
func (pm *PrinterManager) printJob(nativePrinterName, filename, title, user, jobID string, ticket *cdd.CloudJobTicket, updateJob func(string, *cdd.PrintJobStateDiff) error) {
	defer os.Remove(filename)
	if !pm.addInFlightJob(jobID) {
		// This print job was already received. We probably received it
		// again because the first instance is still QUEUED (ie not
		// IN_PROGRESS). That's OK, just throw away the second instance.
		return
	}
	defer pm.deleteInFlightJob(jobID)

	if !pm.jobFullUsername {
		user = strings.Split(user, "@")[0]
	}

	printer, exists := pm.printers.GetByNativeName(nativePrinterName)
	if !exists {
		pm.incrementJobsProcessed(false)
		state := cdd.PrintJobStateDiff{
			State: &cdd.JobState{
				Type:               cdd.JobStateAborted,
				ServiceActionCause: &cdd.ServiceActionCause{ErrorCode: cdd.ServiceActionCausePrinterDeleted},
			},
		}
		if err := updateJob(jobID, &state); err != nil {
			log.ErrorJob(jobID, err)
		}
		return
	}

	nativeJobID, err := pm.native.Print(&printer, filename, title, user, jobID, ticket)
	if err != nil {
		pm.incrementJobsProcessed(false)
		log.ErrorJobf(jobID, "Failed to submit to native print system: %s", err)
		state := cdd.PrintJobStateDiff{
			State: &cdd.JobState{
				Type:              cdd.JobStateAborted,
				DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCausePrintFailure},
			},
		}
		if err := updateJob(jobID, &state); err != nil {
			log.ErrorJob(jobID, err)
		}
		return
	}

	log.InfoJobf(jobID, "Submitted as native job %d", nativeJobID)

	var state cdd.PrintJobStateDiff

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer pm.releaseJob(printer.Name, nativeJobID, jobID)

	for _ = range ticker.C {
		nativeState, err := pm.native.GetJobState(printer.Name, nativeJobID)
		if err != nil {
			log.WarningJobf(jobID, "Failed to get state of native job %d: %s", nativeJobID, err)

			state = cdd.PrintJobStateDiff{
				State: &cdd.JobState{
					Type:              cdd.JobStateAborted,
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: cdd.DeviceActionCauseOther},
				},
				PagesPrinted: state.PagesPrinted,
			}
			if err := updateJob(jobID, &state); err != nil {
				log.ErrorJob(jobID, err)
			}
			pm.incrementJobsProcessed(false)
			return
		}

		if !reflect.DeepEqual(*nativeState, state) {
			state = *nativeState
			if err = updateJob(jobID, &state); err != nil {
				log.ErrorJob(jobID, err)
			}
			log.InfoJobf(jobID, "State: %s", state.State.Type)
		}

		if state.State.Type != cdd.JobStateInProgress && state.State.Type != cdd.JobStateStopped {
			if state.State.Type == cdd.JobStateDone {
				pm.incrementJobsProcessed(true)
			} else {
				pm.incrementJobsProcessed(false)
			}
			return
		}
	}
}

func (pm *PrinterManager) releaseJob(printerName string, nativeJobID uint32, jobID string) {
	if err := pm.native.ReleaseJob(printerName, nativeJobID); err != nil {
		log.ErrorJob(jobID, err)
	}
}

// GetJobStats returns information that is useful for monitoring
// the connector.
func (pm *PrinterManager) GetJobStats() (uint, uint, uint, error) {
	var processing uint

	for _, printer := range pm.printers.GetAll() {
		processing += printer.NativeJobSemaphore.Count()
	}

	pm.jobStatsMutex.Lock()
	defer pm.jobStatsMutex.Unlock()

	return pm.jobsDone, pm.jobsError, processing, nil
}
