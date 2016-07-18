/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"

	"github.com/urfave/cli"
	"github.com/google/cloud-print-connector/cdd"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
)

var commonCommands = []cli.Command{
	cli.Command{
		Name:   "delete-all-gcp-printers",
		Usage:  "Delete all printers associated with this connector",
		Action: deleteAllGCPPrinters,
	},
	cli.Command{
		Name:   "backfill-config-file",
		Usage:  "Add all keys, with default values, to the config file",
		Action: backfillConfigFile,
	},
	cli.Command{
		Name:   "sparse-config-file",
		Usage:  "Remove all keys, with non-default values, from the config file",
		Action: sparseConfigFile,
	},
	cli.Command{
		Name:   "delete-gcp-job",
		Usage:  "Deletes one GCP job",
		Action: deleteGCPJob,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "job-id",
			},
		},
	},
	cli.Command{
		Name:   "cancel-gcp-job",
		Usage:  "Cancels one GCP job",
		Action: cancelGCPJob,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "job-id",
			},
		},
	},
	cli.Command{
		Name:   "delete-all-gcp-printer-jobs",
		Usage:  "Delete all queued jobs associated with a printer",
		Action: deleteAllGCPPrinterJobs,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "printer-id",
			},
		},
	},
	cli.Command{
		Name:   "cancel-all-gcp-printer-jobs",
		Usage:  "Cancels all queued jobs associated with a printer",
		Action: cancelAllGCPPrinterJobs,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "printer-id",
			},
		},
	},
	cli.Command{
		Name:   "show-gcp-printer-status",
		Usage:  "Shows the current status of a printer and it's jobs",
		Action: showGCPPrinterStatus,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "printer-id",
			},
		},
	},
	cli.Command{
		Name:   "share-gcp-printer",
		Usage:  "Shares a printer with user or group",
		Action: shareGCPPrinter,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "printer-id",
				Usage: "Printer to share.",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "Group or user to share with.",
			},
			cli.StringFlag{
				Name:  "role",
				Value: "USER",
				Usage: "Role granted. user or manager.",
			},
			cli.BoolTFlag{
				Name:  "skip-notification",
				Usage: "Skip sending email notice. Defaults to true",
			},
			cli.BoolFlag{
				Name:  "public",
				Usage: "Make the printer public (anyone can print)",
			},
		},
	},
	cli.Command{
		Name:   "unshare-gcp-printer",
		Usage:  "Removes user or group access to printer.",
		Action: unshareGCPPrinter,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "printer-id",
				Usage: "Printer to unshare.",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "Group or user to remove.",
			},
			cli.BoolFlag{
				Name:  "public",
				Usage: "Remove public printer access.",
			},
		},
	},
	cli.Command{
		Name:   "update-gcp-printer",
		Usage:  "Modifies settings for a printer.",
		Action: updateGCPPrinter,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "printer-id",
				Usage: "Printer to update.",
			},
			cli.BoolFlag{
				Name:  "enable-quota",
				Usage: "Set a daily per-user quota.",
			},
			cli.BoolFlag{
				Name:  "disable-quota",
				Usage: "Disable daily per-user quota.",
			},
			cli.IntFlag{
				Name:  "daily-quota",
				Usage: "Pages per-user per-day.",
			},
		},
	},
}

// getConfig returns a config object
func getConfig(context *cli.Context) (*lib.Config, error) {
	config, _, err := lib.GetConfig(context)
	if err != nil {
		log.Fatalln(err)
	}
	return config, nil
}

// getGCP returns a GoogleCloudPrint object
func getGCP(config *lib.Config) *gcp.GoogleCloudPrint {
	gcp, err := gcp.NewGoogleCloudPrint(config.GCPBaseURL, config.RobotRefreshToken,
		config.UserRefreshToken, config.ProxyName, config.GCPOAuthClientID,
		config.GCPOAuthClientSecret, config.GCPOAuthAuthURL, config.GCPOAuthTokenURL,
		0, nil)
	if err != nil {
		log.Fatalln(err)
	}
	return gcp
}

// backfillConfigFile opens the config file, adds all missing keys
// and default values, then writes the config file back.
func backfillConfigFile(context *cli.Context) error {
	config, cfBefore, err := lib.GetConfig(context)
	if err != nil {
		log.Fatalln(err)
	}
	if cfBefore == "" {
		fmt.Println("Could not find a config file to backfill")
		return nil
	}

	// Same config in []byte format.
	configRaw, err := ioutil.ReadFile(cfBefore)
	if err != nil {
		log.Fatalln(err)
	}

	// Same config in map format so that we can detect missing keys.
	var configMap map[string]interface{}
	if err = json.Unmarshal(configRaw, &configMap); err != nil {
		log.Fatalln(err)
	}

	if cfWritten, err := config.Backfill(configMap).ToFile(context); err != nil {
		fmt.Printf("Failed to write config file: %s\n", err)
	} else {
		fmt.Printf("Wrote %s\n", cfWritten)
	}
        return nil
}

// sparseConfigFile opens the config file, removes most keys
// that have default values, then writes the config file back.
func sparseConfigFile(context *cli.Context) error {
	config, cfBefore, err := lib.GetConfig(context)
	if err != nil {
		log.Fatalln(err)
	}
	if cfBefore == "" {
		fmt.Println("Could not find a config file to sparse")
		return nil
	}

	if cfWritten, err := config.Sparse(context).ToFile(context); err != nil {
		fmt.Printf("Failed to write config file: %s\n", err)
	} else {
		fmt.Printf("Wrote %s\n", cfWritten)
	}
        return nil
}

// deleteAllGCPPrinters finds all GCP printers associated with this
// connector, deletes them from GCP.
func deleteAllGCPPrinters(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	printers, err := gcp.List()
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	for gcpID, name := range printers {
		wg.Add(1)
		go func(gcpID, name string) {
			err := gcp.Delete(gcpID)
			if err != nil {
				fmt.Printf("Failed to delete %s \"%s\": %s\n", gcpID, name, err)
			} else {
				fmt.Printf("Deleted %s \"%s\" from GCP\n", gcpID, name)
			}
			wg.Done()
		}(gcpID, name)
	}
	wg.Wait()
        return nil
}

// deleteGCPJob deletes one GCP job
func deleteGCPJob(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	err := gcp.DeleteJob(context.String("job-id"))
	if err != nil {
		fmt.Printf("Failed to delete GCP job %s: %s\n", context.String("job-id"), err)
	} else {
		fmt.Printf("Deleted GCP job %s\n", context.String("job-id"))
	}
        return nil
}

// cancelGCPJob cancels one GCP job
func cancelGCPJob(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	cancelState := cdd.PrintJobStateDiff{
		State: &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		},
	}

	err := gcp.Control(context.String("job-id"), &cancelState)
	if err != nil {
		fmt.Printf("Failed to cancel GCP job %s: %s\n", context.String("job-id"), err)
	} else {
		fmt.Printf("Canceled GCP job %s\n", context.String("job-id"))
	}
        return nil
}

// deleteAllGCPPrinterJobs finds all GCP printer jobs associated with a
// a given printer id and deletes them.
func deleteAllGCPPrinterJobs(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No queued jobs\n")
	}

	ch := make(chan bool)
	for _, job := range jobs {
		go func(gcpJobID string) {
			err := gcp.DeleteJob(gcpJobID)
			if err != nil {
				fmt.Printf("Failed to delete GCP job %s: %s\n", gcpJobID, err)
			} else {
				fmt.Printf("Deleted GCP job %s\n", gcpJobID)
			}
			ch <- true
		}(job.GCPJobID)
	}

	for _ = range jobs {
		<-ch
	}
        return nil
}

// cancelAllGCPPrinterJobs finds all GCP printer jobs associated with a
// a given printer id and cancels them.
func cancelAllGCPPrinterJobs(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	jobs, err := gcp.Fetch(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No queued jobs\n")
	}

	cancelState := cdd.PrintJobStateDiff{
		State: &cdd.JobState{
			Type:            cdd.JobStateAborted,
			UserActionCause: &cdd.UserActionCause{ActionCode: cdd.UserActionCauseCanceled},
		},
	}

	ch := make(chan bool)
	for _, job := range jobs {
		go func(gcpJobID string) {
			err := gcp.Control(gcpJobID, &cancelState)
			if err != nil {
				fmt.Printf("Failed to cancel GCP job %s: %s\n", gcpJobID, err)
			} else {
				fmt.Printf("Cancelled GCP job %s\n", gcpJobID)
			}
			ch <- true
		}(job.GCPJobID)
	}

	for _ = range jobs {
		<-ch
	}
        return nil
}

// showGCPPrinterStatus shows the current status of a GCP printer and it's jobs
func showGCPPrinterStatus(context *cli.Context) error {
	config, _ := getConfig(context)
	gcp := getGCP(config)

	printer, _, err := gcp.Printer(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Name:", printer.DefaultDisplayName)
	fmt.Println("State:", printer.State.State)

	jobs, err := gcp.Jobs(context.String("printer-id"))
	if err != nil {
		log.Fatalln(err)
	}

	// Only init common states. Unusual states like DRAFT will only be shown
	// if there are jobs in that state.
	jobStateCounts := map[string]int{
		"DONE":        0,
		"ABORTED":     0,
		"QUEUED":      0,
		"STOPPED":     0,
		"IN_PROGRESS": 0,
	}

	for _, job := range jobs {
		jobState := string(job.SemanticState.State.Type)
		jobStateCounts[jobState]++
	}

	fmt.Println("Printer jobs:")
	for state, count := range jobStateCounts {
		fmt.Println(" ", state, ":", count)
	}
        return nil
}

// shareGCPPrinter shares a GCP printer
func shareGCPPrinter(context *cli.Context) error {
	config, _ := getConfig(context)
	gcpConn := getGCP(config)

	var role gcp.Role
	switch strings.ToUpper(context.String("role")) {
	case "USER":
		role = gcp.User
	case "MANAGER":
		role = gcp.Manager
	default:
		fmt.Println("role should be user or manager.")
		return nil
	}

	err := gcpConn.Share(context.String("printer-id"), context.String("email"),
		role, context.Bool("skip-notification"), context.Bool("public"))
	var sharedWith string
	if context.Bool("public") {
		sharedWith = "public"
	} else {
		sharedWith = context.String("email")
	}
	if err != nil {
		fmt.Printf("Failed to share GCP printer %s with %s: %s\n", context.String("printer-id"), sharedWith, err)
	} else {
		fmt.Printf("Shared GCP printer %s with %s\n", context.String("printer-id"), sharedWith)
	}
        return nil
}

// unshareGCPPrinter unshares a GCP printer.
func unshareGCPPrinter(context *cli.Context) error {
	config, _ := getConfig(context)
	gcpConn := getGCP(config)

	err := gcpConn.Unshare(context.String("printer-id"), context.String("email"), context.Bool("public"))
	var sharedWith string
	if context.Bool("public") {
		sharedWith = "public"
	} else {
		sharedWith = context.String("email")
	}
	if err != nil {
		fmt.Printf("Failed to unshare GCP printer %s with %s: %s\n", context.String("printer-id"), sharedWith, err)
	} else {
		fmt.Printf("Unshared GCP printer %s with %s\n", context.String("printer-id"), sharedWith)
	}
        return nil
}

// updateGCPPrinter updates settings for a GCP printer.
func updateGCPPrinter(context *cli.Context) error {
	config, _ := getConfig(context)
	gcpConn := getGCP(config)

	var diff lib.PrinterDiff
	diff.Printer = lib.Printer{GCPID: context.String("printer-id")}

	if context.Bool("enable-quota") {
		diff.Printer.QuotaEnabled = true
		diff.QuotaEnabledChanged = true
	} else if context.Bool("disable-quota") {
		diff.Printer.QuotaEnabled = false
		diff.QuotaEnabledChanged = true
	}
	if context.Int("daily-quota") > 0 {
		diff.Printer.DailyQuota = context.Int("daily-quota")
		diff.DailyQuotaChanged = true
	}
	err := gcpConn.Update(&diff)
	if err != nil {
		fmt.Printf("Failed to update GCP printer %s: %s", context.String("printer-id"), err)
	} else {
		fmt.Printf("Updated GCP printer %s", context.String("printer-id"))
	}
        return nil
}
