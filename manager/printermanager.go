/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package manager

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/cups"
	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"
	"github.com/google/cups-connector/snmp"
	"github.com/google/cups-connector/xmpp"

	"github.com/golang/glog"
)

// Manages all interactions between CUPS and Google Cloud Print.
type PrinterManager struct {
	cups *cups.CUPS
	gcp  *gcp.GoogleCloudPrint
	xmpp *xmpp.XMPP
	snmp *snmp.SNMPManager

	// Do not mutate this map, only replace it with a new one. See syncPrinters().
	gcpPrintersByGCPID *lib.ConcurrentPrinterMap
	downloadSemaphore  *lib.Semaphore

	// Job stats are numbers reported to monitoring.
	jobStatsMutex sync.Mutex
	jobsDone      uint
	jobsError     uint

	// Jobs in flight are jobs that have been received, and are not
	// finished printing yet. Key is the GCP Job ID; value is meaningless.
	jobsInFlightMutex sync.Mutex
	jobsInFlight      map[string]bool

	cupsQueueSize     uint
	jobFullUsername   bool
	ignoreRawPrinters bool
	shareScope        string

	quit chan struct{}
}

func NewPrinterManager(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, xmpp *xmpp.XMPP, snmp *snmp.SNMPManager, printerPollInterval string, gcpMaxConcurrentDownload, cupsQueueSize uint, jobFullUsername, ignoreRawPrinters bool, shareScope string) (*PrinterManager, error) {
	// Get the GCP printer list.
	gcpPrinters, queuedJobsCount, err := allGCPPrinters(gcp)
	if err != nil {
		return nil, err
	}
	// Organize the GCP printers into a map.
	for i := range gcpPrinters {
		gcpPrinters[i].CUPSJobSemaphore = lib.NewSemaphore(cupsQueueSize)
	}
	gcpPrintersByGCPID := lib.NewConcurrentPrinterMap(gcpPrinters)

	// Construct.
	pm := PrinterManager{
		cups: cups,
		gcp:  gcp,
		xmpp: xmpp,
		snmp: snmp,

		gcpPrintersByGCPID: gcpPrintersByGCPID,
		downloadSemaphore:  lib.NewSemaphore(gcpMaxConcurrentDownload),

		jobStatsMutex: sync.Mutex{},
		jobsDone:      0,
		jobsError:     0,

		jobsInFlightMutex: sync.Mutex{},
		jobsInFlight:      make(map[string]bool),

		cupsQueueSize:     cupsQueueSize,
		jobFullUsername:   jobFullUsername,
		ignoreRawPrinters: ignoreRawPrinters,
		shareScope:        shareScope,

		quit: make(chan struct{}),
	}

	// Sync once before returning, to make sure things are working.
	if err = pm.syncPrinters(); err != nil {
		return nil, err
	}

	ppi, err := time.ParseDuration(printerPollInterval)
	if err != nil {
		return nil, err
	}
	pm.syncPrintersPeriodically(ppi)
	pm.listenXMPPNotifications()

	for gcpID := range queuedJobsCount {
		go pm.handlePrinterNewJobs(gcpID)
	}

	return &pm, nil
}

func (pm *PrinterManager) Quit() {
	close(pm.quit)
}

// allGCPPrinters calls gcp.List, then calls gcp.Printer, one goroutine per
// printer. This is a fast way to fetch all printers with corresponding CDD
// info, which the List API does not provide.
//
// The second return value is a map of GCPID -> queued print job quantity.
func allGCPPrinters(gcp *gcp.GoogleCloudPrint) ([]lib.Printer, map[string]uint, error) {
	ids, err := gcp.List()
	if err != nil {
		return nil, nil, err
	}

	type response struct {
		printer         *lib.Printer
		queuedJobsCount uint
		err             error
	}
	ch := make(chan response)
	for id := range ids {
		go func(id string) {
			printer, queuedJobsCount, err := gcp.Printer(id)
			ch <- response{printer, queuedJobsCount, err}
		}(id)
	}

	errs := make([]error, 0)
	printers := make([]lib.Printer, 0, len(ids))
	queuedJobsCount := make(map[string]uint)
	for _ = range ids {
		r := <-ch
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		printers = append(printers, *r.printer)
		if r.queuedJobsCount > 0 {
			queuedJobsCount[r.printer.GCPID] = r.queuedJobsCount
		}
	}

	if len(errs) == 0 {
		return printers, queuedJobsCount, nil
	} else if len(errs) == 1 {
		return nil, nil, errs[0]
	} else {
		// Return an error that is somewhat human-readable.
		b := bytes.NewBufferString(fmt.Sprintf("%d errors: ", len(errs)))
		for i, err := range errs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(err.Error())
		}
		return nil, nil, errors.New(b.String())
	}
}

func (pm *PrinterManager) syncPrintersPeriodically(interval time.Duration) {
	go func() {
		t := time.NewTimer(interval)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				if err := pm.syncPrinters(); err != nil {
					glog.Error(err)
				}
				t.Reset(interval)

			case <-pm.quit:
				return
			}
		}
	}()
}

func (pm *PrinterManager) syncPrinters() error {
	glog.Info("Synchronizing printers, stand by")

	cupsPrinters, err := pm.cups.GetPrinters()
	if err != nil {
		return fmt.Errorf("Sync failed while calling GetPrinters(): %s", err)
	}
	if pm.ignoreRawPrinters {
		cupsPrinters, _ = lib.FilterRawPrinters(cupsPrinters)
	}

	if pm.snmp != nil {
		pm.snmp.AugmentPrinters(cupsPrinters)
		if err != nil {
			glog.Warningf("Failed to augment printers with SNMP data: %s", err)
		}
	}

	diffs := lib.DiffPrinters(cupsPrinters, pm.gcpPrintersByGCPID.GetAll())
	if diffs == nil {
		glog.Infof("Printers are already in sync; there are %d", len(cupsPrinters))
		return nil
	}

	ch := make(chan lib.Printer, len(diffs))
	for i := range diffs {
		go pm.applyDiff(&diffs[i], ch)
	}
	currentPrinters := make([]lib.Printer, 0, len(diffs))
	for _ = range diffs {
		p := <-ch
		if p.Name != "" {
			currentPrinters = append(currentPrinters, p)
		}
	}

	pm.gcpPrintersByGCPID.Refresh(currentPrinters)
	glog.Infof("Finished synchronizing %d printers", len(currentPrinters))

	return nil
}

func (pm *PrinterManager) applyDiff(diff *lib.PrinterDiff, ch chan<- lib.Printer) {
	switch diff.Operation {
	case lib.RegisterPrinter:
		if err := pm.gcp.Register(&diff.Printer); err != nil {
			glog.Errorf("Failed to register printer %s: %s", diff.Printer.Name, err)
			break
		}
		glog.Infof("Registered %s", diff.Printer.Name)

		if pm.gcp.CanShare() {
			if err := pm.gcp.Share(diff.Printer.GCPID, pm.shareScope); err != nil {
				glog.Errorf("Failed to share printer %s: %s", diff.Printer.Name, err)
			} else {
				glog.Infof("Shared %s", diff.Printer.Name)
			}
		}

		diff.Printer.CUPSJobSemaphore = lib.NewSemaphore(pm.cupsQueueSize)

		ch <- diff.Printer
		return

	case lib.UpdatePrinter:
		if err := pm.gcp.Update(diff); err != nil {
			glog.Errorf("Failed to update %s: %s", diff.Printer.Name, err)
		} else {
			glog.Infof("Updated %s", diff.Printer.Name)
		}

		ch <- diff.Printer
		return

	case lib.DeletePrinter:
		pm.cups.RemoveCachedPPD(diff.Printer.Name)
		if err := pm.gcp.Delete(diff.Printer.GCPID); err != nil {
			glog.Errorf("Failed to delete a printer %s: %s", diff.Printer.GCPID, err)
			break
		}
		glog.Infof("Deleted %s", diff.Printer.Name)

	case lib.NoChangeToPrinter:
		ch <- diff.Printer
		return
	}

	ch <- lib.Printer{}
}

// listenXMPPNotifications processes the messages found on the xmpp.Notifications()
// channel.
func (pm *PrinterManager) listenXMPPNotifications() {
	go func() {
		for {
			select {
			case <-pm.quit:
				return

			case notification := <-pm.xmpp.Notifications():
				switch notification.Type {
				case xmpp.PrinterNewJobs:
					go pm.handlePrinterNewJobs(notification.GCPID)
				case xmpp.PrinterDelete:
					glog.Errorf("Received XMPP request to delete %s but deleting printers is not supported yet", notification.GCPID)
				}
			}
		}
	}()
}

// handlePrinterNewJobs gets and processes jobs waiting on a printer.
func (pm *PrinterManager) handlePrinterNewJobs(gcpID string) {
	jobs, err := pm.gcp.Fetch(gcpID)
	if err != nil {
		glog.Errorf("Failed to fetch jobs for printer %s: %s", gcpID, err)
		return
	}
	for i := range jobs {
		go pm.processJob(&jobs[i])
	}
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

// addInFlightJob adds a job GCP ID to the in flight set.
//
// Returns true if the job GCP ID was added, false if it already exists.
func (pm *PrinterManager) addInFlightJob(gcpJobID string) bool {
	pm.jobsInFlightMutex.Lock()
	defer pm.jobsInFlightMutex.Unlock()

	if pm.jobsInFlight[gcpJobID] {
		return false
	}

	pm.jobsInFlight[gcpJobID] = true

	return true
}

// deleteInFlightJob deletes a job from the in flight set.
func (pm *PrinterManager) deleteInFlightJob(gcpID string) {
	pm.jobsInFlightMutex.Lock()
	defer pm.jobsInFlightMutex.Unlock()

	delete(pm.jobsInFlight, gcpID)
}

// assembleJob prepares for printing a job by fetching the job's printer,
// ticket, and the job's PDF (what we're printing)
//
// The caller is responsible to remove the returned PDF file.
//
// Errors are returned as a string (last return value), for reporting
// to GCP and local logging.
func (pm *PrinterManager) assembleJob(job *lib.Job) (lib.Printer, cdd.CloudJobTicket, *os.File, string, cdd.PrintJobStateDiff) {
	printer, exists := pm.gcpPrintersByGCPID.Get(job.GCPPrinterID)
	if !exists {
		return lib.Printer{}, cdd.CloudJobTicket{}, nil,
			fmt.Sprintf("Failed to find GCP printer %s for job %s", job.GCPPrinterID, job.GCPJobID),
			cdd.PrintJobStateDiff{
				State: cdd.JobState{
					Type:               "STOPPED",
					ServiceActionCause: &cdd.ServiceActionCause{ErrorCode: "PRINTER_DELETED"},
				},
			}
	}

	ticket, err := pm.gcp.Ticket(job.GCPJobID)
	if err != nil {
		return lib.Printer{}, cdd.CloudJobTicket{}, nil,
			fmt.Sprintf("Failed to get a ticket for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: cdd.JobState{
					Type:              "STOPPED",
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: "INVALID_TICKET"},
				},
			}
	}

	pdfFile, err := cups.CreateTempFile()
	if err != nil {
		return lib.Printer{}, cdd.CloudJobTicket{}, nil,
			fmt.Sprintf("Failed to create a temporary file for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: cdd.JobState{
					Type:              "STOPPED",
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: "OTHER"},
				},
			}
	}

	pm.downloadSemaphore.Acquire()
	t := time.Now()
	// Do not check err until semaphore is released and timer is stopped.
	err = pm.gcp.Download(pdfFile, job.FileURL)
	dt := time.Since(t)
	pm.downloadSemaphore.Release()
	if err != nil {
		// Clean up this temporary file so the caller doesn't need extra logic.
		os.Remove(pdfFile.Name())
		return lib.Printer{}, cdd.CloudJobTicket{}, nil,
			fmt.Sprintf("Failed to download PDF for job %s: %s", job.GCPJobID, err),
			cdd.PrintJobStateDiff{
				State: cdd.JobState{
					Type:              "STOPPED",
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: "DOWNLOAD_FAILURE"},
				},
			}
	}

	glog.Infof("Downloaded job %s in %s", job.GCPJobID, dt.String())
	pdfFile.Close()

	return printer, ticket, pdfFile, "", cdd.PrintJobStateDiff{}
}

// processJob performs these steps:
//
// 1) Assembles the job resources (printer, ticket, PDF)
// 2) Creates a new job in CUPS.
// 3) Follows up with the job state until done or error.
// 4) Deletes temporary file.
//
// Nothing is returned; intended for use as goroutine.
func (pm *PrinterManager) processJob(job *lib.Job) {
	if !pm.addInFlightJob(job.GCPJobID) {
		// This print job was already received. We probably received it
		// again because the first instance is still queued (ie not
		// IN_PROGRESS). That's OK, just throw away the second instance.
		return
	}
	defer pm.deleteInFlightJob(job.GCPJobID)

	glog.Infof("Received job %s", job.GCPJobID)

	printer, ticket, pdfFile, message, state := pm.assembleJob(job)
	if message != "" {
		pm.incrementJobsProcessed(false)
		glog.Error(message)
		if err := pm.gcp.Control(job.GCPJobID, state); err != nil {
			glog.Error(err)
		}
		return
	}
	defer os.Remove(pdfFile.Name())

	ownerID := job.OwnerID
	if !pm.jobFullUsername {
		ownerID = strings.Split(ownerID, "@")[0]
	}

	printer.CUPSJobSemaphore.Acquire()
	defer printer.CUPSJobSemaphore.Release()

	jobTitle := fmt.Sprintf("gcp:%s %s", job.GCPJobID, job.Title)
	if len(jobTitle) > 255 {
		jobTitle = jobTitle[:255]
	}

	cupsJobID, err := pm.cups.Print(printer.Name, pdfFile.Name(), jobTitle, ownerID, ticket)
	if err != nil {
		pm.incrementJobsProcessed(false)
		message = fmt.Sprintf("Failed to send job %s to CUPS: %s", job.GCPJobID, err)
		glog.Error(message)
		state := cdd.PrintJobStateDiff{
			State: cdd.JobState{
				Type:              "STOPPED",
				DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: "PRINT_FAILURE"},
			},
		}
		if err := pm.gcp.Control(job.GCPJobID, state); err != nil {
			glog.Error(err)
		}
		return
	}

	glog.Infof("Submitted GCP job %s as CUPS job %d", job.GCPJobID, cupsJobID)

	pm.followJob(job, cupsJobID)
}

// followJob polls a CUPS job state to update the GCP job state and
// returns when the job state is DONE, STOPPED, or ABORTED.
//
// Nothing is returned, as all errors are reported and logged from
// this function.
func (pm *PrinterManager) followJob(job *lib.Job, cupsJobID uint32) {
	var gcpState cdd.PrintJobStateDiff

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for _ = range ticker.C {
		cupsState, err := pm.cups.GetJobState(cupsJobID)
		if err != nil {
			glog.Warningf("Failed to get state of CUPS job %d: %s", cupsJobID, err)

			gcpState := cdd.PrintJobStateDiff{
				State: cdd.JobState{
					Type:              "STOPPED",
					DeviceActionCause: &cdd.DeviceActionCause{ErrorCode: "OTHER"},
				},
				PagesPrinted: gcpState.PagesPrinted,
			}
			if err := pm.gcp.Control(job.GCPJobID, gcpState); err != nil {
				glog.Error(err)
			}
			pm.incrementJobsProcessed(false)
			return
		}

		if !reflect.DeepEqual(cupsState, gcpState) {
			gcpState = cupsState
			if err = pm.gcp.Control(job.GCPJobID, gcpState); err != nil {
				glog.Error(err)
			}
			glog.Infof("Job %s state is now: %s", job.GCPJobID, gcpState.State.Type)
		}

		if gcpState.State.Type != "IN_PROGRESS" {
			if gcpState.State.Type == "DONE" {
				pm.incrementJobsProcessed(true)
			} else {
				pm.incrementJobsProcessed(false)
			}
			return
		}
	}
}

// GetJobStats returns information that is useful for monitoring
// the connector.
func (pm *PrinterManager) GetJobStats() (uint, uint, uint, error) {
	var processing uint

	for _, printer := range pm.gcpPrintersByGCPID.GetAll() {
		processing += printer.CUPSJobSemaphore.Count()
	}

	pm.jobStatsMutex.Lock()
	defer pm.jobStatsMutex.Unlock()

	return pm.jobsDone, pm.jobsError, processing, nil
}
