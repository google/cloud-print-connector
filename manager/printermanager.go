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
package manager

import (
	"cups-connector/cups"
	"cups-connector/gcp"
	"cups-connector/lib"
	"fmt"
	"os"
	"strings"
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
	jobStatsSemaphore  *lib.Semaphore
	jobsDone           uint
	jobsError          uint
	cupsQueueSize      uint
	jobFullUsername    bool
	ignoreRawPrinters  bool
	shareScope         string
}

func NewPrinterManager(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, printerPollInterval string, gcpMaxConcurrentDownload, cupsQueueSize uint, jobFullUsername, ignoreRawPrinters bool, shareScope string) (*PrinterManager, error) {
	gcpPrinters, err := gcp.List()
	if err != nil {
		return nil, err
	}
	gcpPrintersByGCPID := make(map[string]lib.Printer, len(gcpPrinters))
	for i := range gcpPrinters {
		gcpPrinters[i].CUPSJobSemaphore = lib.NewSemaphore(cupsQueueSize)
		gcpPrintersByGCPID[gcpPrinters[i].GCPID] = gcpPrinters[i]
	}

	gcpJobPollQuit := make(chan bool)
	printerPollQuit := make(chan bool)

	downloadSemaphore := lib.NewSemaphore(gcpMaxConcurrentDownload)
	jobStatsSemaphore := lib.NewSemaphore(1)

	ppi, err := time.ParseDuration(printerPollInterval)
	if err != nil {
		return nil, err
	}

	pm := PrinterManager{
		cups, gcp,
		gcpPrintersByGCPID,
		gcpJobPollQuit, printerPollQuit,
		downloadSemaphore, jobStatsSemaphore, 0, 0, cupsQueueSize,
		jobFullUsername, ignoreRawPrinters,
		shareScope}

	err = pm.syncPrinters()
	if err != nil {
		return nil, err
	}
	go pm.syncPrintersPeriodically(ppi)
	go pm.listenGCPJobs()

	return &pm, nil
}

func (pm *PrinterManager) Quit() {
	pm.printerPollQuit <- true
	<-pm.printerPollQuit
}

func (pm *PrinterManager) syncPrintersPeriodically(interval time.Duration) {
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

func (pm *PrinterManager) listenGCPJobs() {
	ch := make(chan *lib.Job)
	go func() {
		for {
			jobs, err := pm.gcp.NextJobBatch()
			if err != nil {
				if err == gcp.Closed {
					return
				}
				glog.Warningf("Error waiting for next printer: %s", err)
			} else {
				for i := range jobs {
					ch <- &jobs[i]
				}
			}
		}
	}()

	for {
		select {
		case job := <-ch:
			go func(job *lib.Job) {
				gcpJobID, gcpStatus, cupsStatus, message := pm.processJob(job)
				if gcpStatus == lib.JobDone {
					pm.incrementJobsProcessed(true)
				} else {
					glog.Error(message)
					pm.incrementJobsProcessed(false)
				}
				if err := pm.gcp.Control(gcpJobID, gcpStatus, string(cupsStatus), message); err != nil {
					glog.Error(err)
				}
			}(job)
		case <-pm.gcpJobPollQuit:
			pm.gcpJobPollQuit <- true
			return
		}
	}
}

func (pm *PrinterManager) incrementJobsProcessed(success bool) {
	pm.jobStatsSemaphore.Acquire()
	defer pm.jobStatsSemaphore.Release()

	if success {
		pm.jobsDone += 1
	} else {
		pm.jobsError += 1
	}
}

// processJob performs these steps:
//
// 0) Gets a job's ticket (job options).
// 1) Downloads a new print job PDF to a temp file.
// 2) Creates a new job in CUPS.
// 3) Polls the CUPS job status to update the GCP job status.
// 4) Returns when the job status is DONE or ERROR.
// 5) Deletes temp file.
//
// Returns GCP jobID, GCP status, CUPS status, and an error message (or "").
func (pm *PrinterManager) processJob(job *lib.Job) (string, lib.GCPJobStatus, lib.CUPSJobStatus, string) {
	glog.Infof("Received job %s", job.GCPJobID)

	printer, exists := pm.gcpPrintersByGCPID[job.GCPPrinterID]
	if !exists {
		return job.GCPJobID, lib.JobError, "NONE",
			fmt.Sprintf("Failed to find GCP printer %s for job %s", job.GCPPrinterID, job.GCPJobID)
	}

	options, err := pm.gcp.Ticket(job.TicketURL)
	if err != nil {
		return job.GCPJobID, lib.JobError, "NONE",
			fmt.Sprintf("Failed to get a ticket for job %s: %s", job.GCPJobID, err)
	}

	pdfFile, err := pm.cups.CreateTempFile()
	if err != nil {
		return job.GCPJobID, lib.JobError, "NONE",
			fmt.Sprintf("Failed to create a temporary file for job %s: %s", job.GCPJobID, err)
	}

	pm.downloadSemaphore.Acquire()
	t := time.Now()
	// Do not check err until semaphore is released and timer is stopped.
	err = pm.gcp.Download(pdfFile, job.FileURL)
	dt := time.Now().Sub(t)
	pm.downloadSemaphore.Release()
	if err != nil {
		return job.GCPJobID, lib.JobError, "NONE",
			fmt.Sprintf("Failed to download PDF for job %s: %s", job.GCPJobID, err)
	}

	glog.Infof("Downloaded job %s in %s", job.GCPJobID, dt.String())
	pdfFile.Close()
	defer os.Remove(pdfFile.Name())

	ownerID := job.OwnerID
	if !pm.jobFullUsername {
		ownerID = strings.Split(ownerID, "@")[0]
	}

	printer.CUPSJobSemaphore.Acquire()
	defer printer.CUPSJobSemaphore.Release()

	cupsJobID, err := pm.cups.Print(printer.Name, pdfFile.Name(), "gcp:"+job.GCPJobID, ownerID, options)
	if err != nil {
		return job.GCPJobID, lib.JobError, "NONE",
			fmt.Sprintf("Failed to send job %s to CUPS: %s", job.GCPJobID, err)
	}

	glog.Infof("Submitted GCP job %s as CUPS job %d", job.GCPJobID, cupsJobID)

	var cupsStatus lib.CUPSJobStatus
	var gcpStatus lib.GCPJobStatus
	message := ""

	for _ = range time.Tick(time.Second) {
		latestCUPSStatus, latestMessage, err := pm.cups.GetJobStatus(cupsJobID)
		if err != nil {
			return job.GCPJobID, lib.JobError, "UNKNOWN",
				fmt.Sprintf("Failed to get gcpStatus of CUPS job %d: %s", cupsJobID, err)
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
				return job.GCPJobID, lib.JobDone, cupsStatus, ""
			} else {
				return job.GCPJobID, lib.JobError, cupsStatus, fmt.Sprintf("Print job %s failed: %s", job.GCPJobID, message)
			}
		}
	}
	panic("unreachable")
}

func (pm *PrinterManager) GetJobStats() (uint, uint, uint, error) {
	var processing uint

	for _, printer := range pm.gcpPrintersByGCPID {
		processing += printer.CUPSJobSemaphore.Count()
	}

	pm.jobStatsSemaphore.Acquire()
	defer pm.jobStatsSemaphore.Release()

	return pm.jobsDone, pm.jobsError, processing, nil
}
