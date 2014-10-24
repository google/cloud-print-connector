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
	"log"
	"os"
	"strings"
	"time"
)

// Manages all interactions between CUPS and Google Cloud Print.
type PrinterManager struct {
	cups               *cups.CUPS
	gcp                *gcp.GoogleCloudPrint
	gcpPrintersByGCPID map[string]lib.Printer
	gcpJobPollQuit     chan bool
	printerPollQuit    chan bool
	downloadSemaphore  *lib.Semaphore
	jobPollInterval    time.Duration
	jobFullUsername    bool
}

func NewPrinterManager(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, printerPollInterval, jobPollInterval, gcpMaxConcurrentDownload uint, jobFullUsername bool) (*PrinterManager, error) {
	gcpPrinters, err := gcp.List()
	if err != nil {
		return nil, err
	}
	gcpPrintersByGCPID := make(map[string]lib.Printer, len(gcpPrinters))
	for _, p := range gcpPrinters {
		gcpPrintersByGCPID[p.GCPID] = p
	}

	gcpJobPollQuit := make(chan bool)
	printerPollQuit := make(chan bool)

	downloadSemaphore := lib.NewSemaphore(gcpMaxConcurrentDownload)

	jpi := time.Duration(jobPollInterval) * time.Second

	pm := PrinterManager{cups, gcp, gcpPrintersByGCPID, gcpJobPollQuit, printerPollQuit,
		downloadSemaphore, jpi, jobFullUsername}

	pm.syncPrinters()
	go pm.syncPrintersPeriodically(printerPollInterval)
	go pm.listenGCPJobs()

	return &pm, nil
}

func (pm *PrinterManager) Quit() {
	pm.printerPollQuit <- true
	<-pm.printerPollQuit
}

func (pm *PrinterManager) syncPrintersPeriodically(printerPollInterval uint) {
	interval := time.Duration(printerPollInterval) * time.Second
	for {
		select {
		case <-time.After(interval):
			pm.syncPrinters()
		case <-pm.printerPollQuit:
			pm.printerPollQuit <- true
			return
		}
	}
}

func printerMapToSlice(m map[string]lib.Printer) []lib.Printer {
	s := make([]lib.Printer, 0, len(m))
	for _, p := range m {
		s = append(s, p)
	}
	return s
}

func (pm *PrinterManager) syncPrinters() {
	fmt.Println("Synchronizing printers, stand by")

	cupsPrinters, err := pm.cups.GetPrinters()
	if err != nil {
		log.Printf("Sync failed while calling GetPrinters():\n  %s\n", err)
		return
	}
	diffs := lib.DiffPrinters(cupsPrinters, printerMapToSlice(pm.gcpPrintersByGCPID))

	if diffs == nil {
		fmt.Printf("Printers are already in sync; there are %d\n", len(cupsPrinters))
		return
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

	pm.gcpPrintersByGCPID = currentPrinters

	fmt.Printf("Finished synchronizing %d printers\n", len(currentPrinters))
}

func (pm *PrinterManager) applyDiff(diff *lib.PrinterDiff, ch chan<- lib.Printer) {
	switch diff.Operation {
	case lib.RegisterPrinter:
		ppd, err := pm.cups.GetPPD(diff.Printer.Name)
		if err != nil {
			log.Printf("Failed to call GetPPD() while registering printer %s:\n  %s\n",
				diff.Printer.Name, err)
			break
		}
		if err := pm.gcp.Register(&diff.Printer, ppd); err != nil {
			log.Printf("Failed to register printer %s:\n  %s\n", diff.Printer.Name, err)
			break
		}
		fmt.Printf("Registered %s\n", diff.Printer.Name)

		if pm.gcp.CanShare() {
			if err := pm.gcp.Share(diff.Printer.GCPID); err != nil {
				log.Printf("Failed to share printer %s:\n  %s\n", diff.Printer.Name, err)
			} else {
				fmt.Printf("Shared %s\n", diff.Printer.Name)
			}
		}

		ch <- diff.Printer
		return

	case lib.UpdatePrinter:
		var ppd string
		if diff.CapsHashChanged {
			var err error
			ppd, err = pm.cups.GetPPD(diff.Printer.Name)
			if err != nil {
				log.Printf("Failed to call GetPPD() while updating printer %s:\n  %s\n",
					diff.Printer.Name, err)
				ch <- diff.Printer
				return
			}
		}

		if err := pm.gcp.Update(diff, ppd); err != nil {
			log.Printf("Failed to update a printer:\n  %s\n", err)
		} else {
			fmt.Printf("Updated %s\n", diff.Printer.Name)
		}

		ch <- diff.Printer
		return

	case lib.DeletePrinter:
		if err := pm.gcp.Delete(diff.Printer.GCPID); err != nil {
			log.Printf("Failed to delete a printer %s:\n  %s\n", diff.Printer.GCPID, err)
			break
		}
		fmt.Printf("Deleted %s\n", diff.Printer.Name)

	case lib.LeavePrinter:
		// TODO(jacobmarble): When proper logging, this is DEBUG.
		fmt.Printf("No change to %s\n", diff.Printer.Name)
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
				log.Printf("Error waiting for next printer: %s", err)
			}
			for _, job := range jobs {
				ch <- &job
			}
		}
	}()

	for {
		select {
		case job := <-ch:
			go pm.processJob(job)
		case <-pm.gcpJobPollQuit:
			pm.gcpJobPollQuit <- true
			return
		}
	}
}

// 0) Gets a job's ticket (job options).
// 1) Downloads a new print job PDF to a temp file.
// 2) Creates a new job in CUPS.
// 3) Polls the CUPS job status to update the GCP job status.
// 4) Returns when the job status is DONE or ERROR.
// 5) Deletes temp file.
func (pm *PrinterManager) processJob(job *lib.Job) {
	fmt.Printf("Received job %s\n", job.GCPJobID)

	printer, exists := pm.gcpPrintersByGCPID[job.GCPPrinterID]
	if !exists {
		msg := fmt.Sprintf("Failed to find printer %s for job %s", job.GCPPrinterID, job.GCPJobID)
		log.Println(msg)
		pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
		return
	}

	options, err := pm.gcp.Ticket(job.TicketURL)
	if err != nil {
		msg := fmt.Sprintf("Failed to get a job ticket: %s", err)
		log.Println(msg)
		pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
		return
	}

	pdfFile, err := pm.cups.CreateTempFile()
	if err != nil {
		msg := fmt.Sprintf("Failed to create a temporary file for job: %s", err)
		log.Println(msg)
		pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
		return
	}

	pm.downloadSemaphore.Acquire()
	t := time.Now()
	err = pm.gcp.Download(pdfFile, job.FileURL)
	dt := time.Now().Sub(t)
	pm.downloadSemaphore.Release()
	if err != nil {
		msg := fmt.Sprintf("Failed to download a job PDF: %s", err)
		log.Println(msg)
		pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
		return
	}

	fmt.Printf("Downloaded job %s in %s\n", job.GCPJobID, dt.String())
	pdfFile.Close()
	defer os.Remove(pdfFile.Name())

	ownerID := job.OwnerID
	if !pm.jobFullUsername {
		ownerID = strings.Split(ownerID, "@")[0]
	}

	cupsJobID, err := pm.cups.Print(printer.Name, pdfFile.Name(), "gcp:"+job.GCPJobID, ownerID, options)
	if err != nil {
		msg := fmt.Sprintf("Failed to send job %s to CUPS: %s", job.GCPJobID, err)
		log.Println(msg)
		pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
		return
	}

	fmt.Printf("Submitted GCP job %s as CUPS job %d\n", job.GCPJobID, cupsJobID)

	status := ""
	message := ""

	for _ = range time.Tick(pm.jobPollInterval) {
		latestStatus, latestMessage, err := pm.cups.GetJobStatus(cupsJobID)
		if err != nil {
			msg := fmt.Sprintf("Failed to get status of CUPS job %d: %s", cupsJobID, err)
			log.Println(msg)
			pm.gcp.Control(job.GCPJobID, lib.JobError, msg)
			return
		}

		if latestStatus.GCPStatus() != status || latestMessage != message {
			status = latestStatus.GCPStatus()
			message = latestMessage
			pm.gcp.Control(job.GCPJobID, status, message)
			fmt.Printf("Job %s status: %s\n", job.GCPJobID, status)
		}

		if latestStatus.GCPStatus() != lib.JobInProgress {
			break
		}
	}
}
