/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package manager

import (
	"cups-connector/cups"
	"cups-connector/gcp"
	"cups-connector/lib"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

// Number of seconds between XMPP channel pings.
const xmppTimeout = 300

// Manages all interactions between CUPS and Google Cloud Print.
type PrinterManager struct {
	cups *cups.CUPS
	gcp  *gcp.GoogleCloudPrint

	// Do not mutate this map, only replace it with a new one. See syncPrinters().
	gcpPrintersByGCPID map[string]lib.Printer
	gcpJobPollQuit     chan bool
	printerPollQuit    chan bool
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
}

func NewPrinterManager(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, printerPollInterval string, gcpMaxConcurrentDownload, cupsQueueSize uint, jobFullUsername, ignoreRawPrinters bool, shareScope string) (*PrinterManager, error) {
	gcpPrinters, queuedJobsCount, err := gcp.List()
	if err != nil {
		return nil, err
	}
	gcpPrintersByGCPID := make(map[string]lib.Printer, len(gcpPrinters))
	for i := range gcpPrinters {
		gcpPrinters[i].CUPSJobSemaphore = lib.NewSemaphore(cupsQueueSize)
		gcpPrintersByGCPID[gcpPrinters[i].GCPID] = gcpPrinters[i]
	}

	ppi, err := time.ParseDuration(printerPollInterval)
	if err != nil {
		return nil, err
	}

	pm := PrinterManager{
		cups: cups,
		gcp:  gcp,

		gcpPrintersByGCPID: gcpPrintersByGCPID,
		gcpJobPollQuit:     make(chan bool),
		printerPollQuit:    make(chan bool),
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
	}

	err = pm.syncPrinters()
	if err != nil {
		return nil, err
	}

	pm.syncPrintersPeriodically(ppi)
	pm.listenGCPJobs(queuedJobsCount)

	return &pm, nil
}

func (pm *PrinterManager) Quit() {
	pm.printerPollQuit <- true
	<-pm.printerPollQuit
}

func (pm *PrinterManager) syncPrintersPeriodically(interval time.Duration) {
	go func() {
		for {
			select {
			case <-time.After(interval):
				err := pm.syncPrinters()
				if err != nil {
					glog.Error(err)
				}
			case <-pm.printerPollQuit:
				pm.printerPollQuit <- true
				return
			}
		}
	}()
}

func printerMapToSlice(m map[string]lib.Printer) []lib.Printer {
	s := make([]lib.Printer, 0, len(m))
	for k := range m {
		s = append(s, m[k])
	}
	return s
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
	for i := range cupsPrinters {
		cupsPrinters[i].XMPPTimeout = xmppTimeout
	}

	diffs := lib.DiffPrinters(cupsPrinters, printerMapToSlice(pm.gcpPrintersByGCPID))

	if diffs == nil {
		glog.Infof("Printers are already in sync; there are %d", len(cupsPrinters))
		return nil
	}

	ch := make(chan lib.Printer)
	for i := range diffs {
		go pm.applyDiff(&diffs[i], ch)
	}
	currentPrinters := make(map[string]lib.Printer)
	for _ = range diffs {
		p := <-ch
		if p.Name != "" {
			currentPrinters[p.GCPID] = p
		}
	}

	// Notice that we never mutate pm.gcpPrintersByGCPID, only replace the map
	// that it points to.
	pm.gcpPrintersByGCPID = currentPrinters
	glog.Infof("Finished synchronizing %d printers", len(currentPrinters))

	return nil
}

func (pm *PrinterManager) applyDiff(diff *lib.PrinterDiff, ch chan<- lib.Printer) {
	switch diff.Operation {
	case lib.RegisterPrinter:
		ppd, err := pm.cups.GetPPD(diff.Printer.Name)
		if err != nil {
			glog.Errorf("Failed to call GetPPD() while registering printer %s: %s",
				diff.Printer.Name, err)
			break
		}
		if err := pm.gcp.Register(&diff.Printer, ppd); err != nil {
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
		var ppd string
		if diff.CapsHashChanged {
			var err error
			ppd, err = pm.cups.GetPPD(diff.Printer.Name)
			if err != nil {
				glog.Errorf("Failed to call GetPPD() while updating printer %s: %s",
					diff.Printer.Name, err)
				ch <- diff.Printer
				return
			}
		}

		if err := pm.gcp.Update(diff, ppd); err != nil {
			glog.Errorf("Failed to update a printer: %s", err)
		} else {
			glog.Infof("Updated %s", diff.Printer.Name)
		}

		ch <- diff.Printer
		return

	case lib.DeletePrinter:
		if err := pm.gcp.Delete(diff.Printer.GCPID); err != nil {
			glog.Errorf("Failed to delete a printer %s: %s", diff.Printer.GCPID, err)
			break
		}
		glog.Infof("Deleted %s", diff.Printer.Name)

	case lib.NoChangeToPrinter:
		glog.Infof("No change to %s", diff.Printer.Name)
		ch <- diff.Printer
		return
	}

	ch <- lib.Printer{}
}

func (pm *PrinterManager) listenGCPJobs(queuedJobsCount map[string]uint) {
	ch := make(chan *lib.Job)

	for gcpID := range queuedJobsCount {
		go func() {
			jobs, err := pm.gcp.Fetch(gcpID)
			if err != nil {
				glog.Warningf("Error fetching print jobs: %s", err)
				return
			}

			if len(jobs) > 0 {
				glog.Infof("Fetched %d waiting print jobs for printer %s", len(jobs), gcpID)
			}
			for i := range jobs {
				ch <- &jobs[i]
			}
		}()
	}

	go func() {
		for {
			jobs, err := pm.gcp.NextJobBatch()
			if err != nil {
				if err == gcp.ErrClosed {
					return
				}
				glog.Warningf("Error waiting for next print job notification: %s", err)

			} else {
				for i := range jobs {
					ch <- &jobs[i]
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case job := <-ch:
				go pm.processJob(job)
			case <-pm.gcpJobPollQuit:
				pm.gcpJobPollQuit <- true
				return
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
// ticket (aka options), and the job's PDF (what we're printing)
//
// The caller is responsible to remove the returned PDF file.
//
// Errors are returned as a string (last return value), for reporting
// to GCP and local logging.
func (pm *PrinterManager) assembleJob(job *lib.Job) (lib.Printer, map[string]string, *os.File, string) {
	printer, exists := pm.gcpPrintersByGCPID[job.GCPPrinterID]
	if !exists {
		return lib.Printer{}, nil, nil,
			fmt.Sprintf("Failed to find GCP printer %s for job %s", job.GCPPrinterID, job.GCPJobID)
	}

	options, err := pm.gcp.Ticket(job.TicketURL)
	if err != nil {
		return lib.Printer{}, nil, nil,
			fmt.Sprintf("Failed to get a ticket for job %s: %s", job.GCPJobID, err)
	}

	pdfFile, err := cups.CreateTempFile()
	if err != nil {
		return lib.Printer{}, nil, nil,
			fmt.Sprintf("Failed to create a temporary file for job %s: %s", job.GCPJobID, err)
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
		return lib.Printer{}, nil, nil,
			fmt.Sprintf("Failed to download PDF for job %s: %s", job.GCPJobID, err)
	}

	glog.Infof("Downloaded job %s in %s", job.GCPJobID, dt.String())
	pdfFile.Close()

	return printer, options, pdfFile, ""
}

// processJob performs these steps:
//
// 1) Assembles the job resources (printer, ticket, PDF)
// 2) Creates a new job in CUPS.
// 3) Follows up with the job status until done or error.
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

	printer, options, pdfFile, message := pm.assembleJob(job)
	if message != "" {
		pm.incrementJobsProcessed(false)
		glog.Error(message)
		if err := pm.gcp.Control(job.GCPJobID, lib.JobError, "NONE", message); err != nil {
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

	jobTitle := fmt.Sprintf("gcp:%s:%s", job.GCPJobID, job.Title)

	cupsJobID, err := pm.cups.Print(printer.Name, pdfFile.Name(), jobTitle, ownerID, options)
	if err != nil {
		pm.incrementJobsProcessed(false)
		message = fmt.Sprintf("Failed to send job %s to CUPS: %s", job.GCPJobID, err)
		glog.Error(message)
		if err := pm.gcp.Control(job.GCPJobID, lib.JobError, "NONE", message); err != nil {
			glog.Error(err)
		}
		return
	}

	glog.Infof("Submitted GCP job %s as CUPS job %d", job.GCPJobID, cupsJobID)

	pm.followJob(job, cupsJobID)
}

// followJob polls a CUPS job status to update the GCP job status and
// returns when the job status is DONE or ERROR.
//
// Nothing is returned, as all errors are reported and logged from
// this function.
func (pm *PrinterManager) followJob(job *lib.Job, cupsJobID uint32) {
	var cupsStatus lib.CUPSJobStatus
	var gcpStatus lib.GCPJobStatus
	var message string

	for _ = range time.Tick(time.Second) {
		latestCUPSStatus, latestMessage, err := pm.cups.GetJobStatus(cupsJobID)
		if err != nil {
			gcpStatus = lib.JobError
			cupsStatus = "UNKNOWN"
			message = fmt.Sprintf("Failed to get status of CUPS job %d: %s", cupsJobID, err)
			if err := pm.gcp.Control(job.GCPJobID, lib.JobError, "NONE", message); err != nil {
				glog.Error(err)
			}
			pm.incrementJobsProcessed(false)
			break
		}

		if latestCUPSStatus != cupsStatus || latestMessage != message {
			cupsStatus = latestCUPSStatus
			gcpStatus = latestCUPSStatus.GCPJobStatus()
			message = latestMessage
			if err = pm.gcp.Control(job.GCPJobID, gcpStatus, string(cupsStatus), message); err != nil {
				glog.Error(err)
			}
			glog.Infof("Job %s status is now: %s/%s", job.GCPJobID, cupsStatus, gcpStatus)
		}

		if gcpStatus != lib.JobInProgress {
			if gcpStatus == lib.JobDone {
				pm.incrementJobsProcessed(true)
			} else {
				pm.incrementJobsProcessed(false)
			}
			break
		}
	}
}

// GetJobStats returns information that is useful for monitoring
// the connector.
func (pm *PrinterManager) GetJobStats() (uint, uint, uint, error) {
	var processing uint

	for _, printer := range pm.gcpPrintersByGCPID {
		processing += printer.CUPSJobSemaphore.Count()
	}

	pm.jobStatsMutex.Lock()
	defer pm.jobStatsMutex.Unlock()

	return pm.jobsDone, pm.jobsError, processing, nil
}
