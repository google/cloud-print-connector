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
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

// Manages all interactions between CUPS and Google Cloud Print.
type PrinterManager struct {
	cups               *cups.CUPS
	gcp                *gcp.GoogleCloudPrint
	gcpPrintersByGCPID map[string]lib.Printer
	jobStatusRequest   chan jobStatusRequest
	jobPollQuit        chan bool
	gcpJobPollQuit     chan bool
	printerPollQuit    chan bool
}

func NewPrinterManager(cups *cups.CUPS, gcp *gcp.GoogleCloudPrint, printerPollInterval, jobPollInterval uint) (*PrinterManager, error) {
	gcpPrinters, err := gcp.List()
	if err != nil {
		return nil, err
	}
	gcpPrintersByGCPID := make(map[string]lib.Printer, len(gcpPrinters))
	for _, p := range gcpPrinters {
		gcpPrintersByGCPID[p.GCPID] = p
	}

	jobStatusRequest := make(chan jobStatusRequest)
	jobPollQuit := make(chan bool)
	gcpJobPollQuit := make(chan bool)
	printerPollQuit := make(chan bool)

	pm := PrinterManager{cups, gcp, gcpPrintersByGCPID, jobStatusRequest, jobPollQuit, gcpJobPollQuit, printerPollQuit}

	pm.syncPrinters()
	go pm.syncPrintersPeriodically(printerPollInterval)
	go pm.pollCUPSJobs(jobPollInterval)
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
	fmt.Println("Starting syncPrinters")

	cupsPrinters, err := pm.cups.GetDests()
	if err != nil {
		log.Printf("Failed to call GetDests():\n  %s\n", err)
	}
	diffs := lib.DiffPrinters(cupsPrinters, printerMapToSlice(pm.gcpPrintersByGCPID))

	if diffs == nil {
		// Nothing to do
		fmt.Println("Nothing to sync")
		return
	}

	currentPrinters := make(map[string]lib.Printer)

	for _, diff := range diffs {
		switch diff.Operation {
		case lib.RegisterPrinter:
			fmt.Printf("Registering %s\n", diff.Printer.Name)
			ppd, err := pm.cups.GetPPD(diff.Printer.Name)
			if err != nil {
				log.Printf("Failed to call GetPPD():\n  %s\n", err)
				break
			}
			if err := pm.gcp.Register(&diff.Printer, ppd); err != nil {
				log.Printf("Failed to register a new printer:\n  %s\n", err)
			} else {
				currentPrinters[diff.Printer.GCPID] = diff.Printer
			}

		case lib.UpdatePrinter:
			fmt.Printf("Updating %s\n", diff.Printer.Name)
			var ppd string
			if diff.CapsHashChanged {
				ppd, err = pm.cups.GetPPD(diff.Printer.Name)
				if err != nil {
					log.Printf("Failed to call GetPPD():\n  %s\n", err)
					break
				}
			}
			if err = pm.gcp.Update(&diff, ppd); err != nil {
				log.Printf("Failed to update a printer:\n  %s\n", err)
			} else {
				currentPrinters[diff.Printer.GCPID] = diff.Printer
			}

		case lib.DeletePrinter:
			fmt.Printf("Deleting %s\n", diff.Printer.Name)
			if err := pm.gcp.Delete(diff.Printer.GCPID); err != nil {
				log.Printf("Failed to delete a printer %s:\n  %s\n", diff.Printer.GCPID, err)
			}

		case lib.LeavePrinter:
			fmt.Printf("Leaving %s\n", diff.Printer.Name)
			currentPrinters[diff.Printer.GCPID] = diff.Printer
		}
	}

	pm.gcpPrintersByGCPID = currentPrinters

	fmt.Println("Finished syncPrinters")
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
			go pm.processJob(job.GCPPrinterID, job.GCPJobID, job.FileURL)
		case <-pm.gcpJobPollQuit:
			pm.gcpJobPollQuit <- true
			return
		}
	}
}

// 1) Downloads a new print job PDF to a temp file.
// 2) Creates a new job in CUPS.
// 3) Polls the CUPS job status to update the GCP job status.
// 4) Returns when the job status is DONE or ERROR.
// 5) Deletes temp file.
func (pm *PrinterManager) processJob(gcpPrinterID, gcpJobID, fileURL string) {
	printer, exists := pm.gcpPrintersByGCPID[gcpPrinterID]
	if !exists {
		log.Printf("Failed to find printer %s for job %s\n", gcpPrinterID, gcpJobID)
		fmt.Printf("%+v\n", pm.gcpPrintersByGCPID)
		// TODO: gcp status=error
		return
	}

	pdfFile, err := pm.cups.CreateTempFile()
	if err != nil {
		log.Printf("Failed to process job: %s\n", err)
		// TODO: gcp status=error
		return
	}

	pm.gcp.Download(pdfFile, fileURL)
	pdfFile.Close()
	defer os.Remove(pdfFile.Name())

	cupsJobID, err := pm.cups.Print(printer.Name, pdfFile.Name(), "gcp:"+gcpJobID)
	if err != nil {
		log.Printf("Failed to send job %s to CUPS: %s\n", gcpJobID, err)
		// TODO: gcp status=error
		return
	}

	status := ""
	message := ""

	for _ = range time.Tick(time.Second * 5) {
		latestStatus, latestMessage, err := pm.getCUPSJobStatus(cupsJobID)
		if err != nil {
			log.Printf("Failed to get status of CUPS job %d\n", cupsJobID)
			// TODO: gcp status=error
			return
		}

		if latestStatus.GCPStatus() != status || latestMessage != message {
			status = latestStatus.GCPStatus()
			message = latestMessage
			pm.gcp.Control(gcpJobID, status, message)
		}

		if latestStatus.GCPStatus() != "IN_PROGRESS" {
			break
		}
	}
}

type jobStatusRequest struct {
	jobID    uint32
	response chan jobStatusResponse
}

type jobStatusResponse struct {
	status  lib.JobStatus
	message string
	err     error
}

func (pm *PrinterManager) getCUPSJobStatus(jobID uint32) (lib.JobStatus, string, error) {
	ch := make(chan jobStatusResponse)
	request := jobStatusRequest{jobID, ch}
	pm.jobStatusRequest <- request
	response := <-ch
	return response.status, response.message, response.err
}

// Answers requests to poll jobs on the jobStatusRequest channel.
// The CUPS API only knows how to poll all jobs, not one job. We need to be
// able to query one job, so this function caches CUPS API responses.
func (pm *PrinterManager) pollCUPSJobs(jobPollInterval uint) {
	maxPoll := time.Duration(jobPollInterval) * time.Second
	lastPoll := time.Time{}
	jobs := make(map[uint32]lib.JobStatus, 0)

	for {
		select {
		case request := <-pm.jobStatusRequest:
			status, exists := jobs[request.jobID]

			if time.Since(lastPoll) > maxPoll || !exists {
				// The jobs map is stale; refresh it.
				fmt.Println("polling jobs")
				jobs, err := pm.cups.GetJobs()
				if err != nil {
					jobs = make(map[uint32]lib.JobStatus, 0)
					request.response <- jobStatusResponse{0, "", err}
					continue
				} else {
					lastPoll = time.Now()
				}

				// Now that the jobs map is fresh, query it again.
				status, exists = jobs[request.jobID]
			}

			if exists {
				// TODO: Get status message with status.
				request.response <- jobStatusResponse{status, "", nil}
			} else {
				text := fmt.Sprintf("Job ID %d doesn't exist", request.jobID)
				request.response <- jobStatusResponse{0, "", errors.New(text)}
			}

		case <-pm.jobPollQuit:
			pm.jobPollQuit <- true
			return
		}
	}
}
