/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Package gcp is the Google Cloud Print API client.
package gcp

import (
	"cups-connector/lib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
)

const (
	// This prefix tickles a magic spell in GCP so that, for example,
	// the GCP UI shows location as the string found in the
	// printer-location CUPS attribute.
	gcpTagPrefix = "__cp__"

	// XMPP connections fail. Attempt to reconnect a few times before giving up.
	restartXMPPMaxRetries = 4

	// Stop retrying when two failures occur in a short period of time.
	minTimeBetweenXMPPFailures = time.Second * 3

	// OAuth constants.
	RedirectURL     = "oob"
	ScopeCloudPrint = "https://www.googleapis.com/auth/cloudprint"
	ScopeGoogleTalk = "https://www.googleapis.com/auth/googletalk"
	AccessType      = "offline"
)

// GoogleCloudPrint is the interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	baseURL     string
	xmppJID     string
	xmppClient  *gcpXMPP
	robotClient *http.Client
	userClient  *http.Client
	proxyName   string

	xmppServer              string
	xmppPort                uint16
	xmppPingTimeout         time.Duration
	xmppPingIntervalDefault time.Duration
}

// NewGoogleCloudPrint establishes a connection with GCP, returns a new GoogleCloudPrint object.
func NewGoogleCloudPrint(baseURL, xmppJID, robotRefreshToken, userRefreshToken, proxyName, oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, xmppServer string, xmppPort uint16, xmppPingTimeout, xmppPingIntervalDefault time.Duration) (*GoogleCloudPrint, error) {
	robotClient, err := newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, robotRefreshToken, ScopeCloudPrint, ScopeGoogleTalk)
	if err != nil {
		return nil, err
	}

	var userClient *http.Client
	if userRefreshToken != "" {
		userClient, err = newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, userRefreshToken, ScopeCloudPrint)
		if err != nil {
			return nil, err
		}
	}

	gcp := &GoogleCloudPrint{baseURL, xmppJID, nil, robotClient, userClient, proxyName,
		xmppServer, xmppPort, xmppPingTimeout, xmppPingIntervalDefault}

	return gcp, nil
}

type localSettingsSettings struct {
	XMPPPingInterval uint32 `json:"xmpp_timeout_value"`
	// Other values can be set here; omit until needed.
}

// localSettingsPush is for Register and Update.
// Using the pending field is not permitted from device accounts.
type localSettingsPush struct {
	Current localSettingsSettings `json:"current"`
}

// localSettingsPull is for List.
type localSettingsPull struct {
	Current localSettingsSettings `json:"current"`
	Pending localSettingsSettings `json:"pending"`
}

// Turn arguments into a JSON-encoded GCP LocalSettings object.
func marshalLocalSettings(xmppInterval time.Duration) (string, error) {
	var ls localSettingsPush
	ls.Current.XMPPPingInterval = uint32(xmppInterval.Seconds())
	lss, err := json.Marshal(ls)
	if err != nil {
		return "", err
	}
	return string(lss), nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (gcp *GoogleCloudPrint) Quit() {
	gcp.xmppClient.quit()
}

// CanShare answers the question "can we share printers when they are registered?"
func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userClient != nil
}

// StartXMPP tries to start an XMPP conversation.
// Tries multiple times before returning an error.
func (gcp *GoogleCloudPrint) StartXMPP() error {
	if gcp.xmppClient != nil {
		go gcp.xmppClient.quit()
	}

	var err error
	for i := 0; i < restartXMPPMaxRetries; i++ {
		// The current access token is the XMPP password.
		var token *oauth2.Token
		token, err = gcp.robotClient.Transport.(*oauth2.Transport).Source.Token()

		if err == nil {
			var xmpp *gcpXMPP
			xmpp, err = newXMPP(gcp.xmppJID, token.AccessToken, gcp.proxyName,
				gcp.xmppServer, gcp.xmppPort, gcp.xmppPingTimeout, gcp.xmppPingIntervalDefault)

			if err == nil {
				gcp.xmppClient = xmpp
				return nil
			}
		}

		// Sleep for 1, 2, 4, 8 seconds.
		time.Sleep(time.Duration((i+1)*2) * time.Second)
	}

	return fmt.Errorf("Failed to start XMPP conversation: %s", err)
}

// NextJobBatch gets the next batch of print jobs from GCP. Blocks on XMPP until
// batch notification arrives. Calls google.com/cloudprint/fetch to get the jobs.
//
// Returns ErrClosed if the XMPP connection closes; the caller should assume
// that life is gracefully ending.
//
// If any other error is returned, then there's probably no reason to go on living.
func (gcp *GoogleCloudPrint) NextJobBatch() ([]lib.Job, error) {
	var lastFailure time.Time

	var gcpID string
	var err error
	for {
		gcpID, err = gcp.xmppClient.nextPrinterWithJobs()
		if err == nil {
			// Success!
			break
		}

		if err == ErrClosed {
			// The connection is closed.
			return nil, err
		}

		if strings.Contains(err.Error(), "Unexpected element") {
			// Not really an error, but interesting, so log and try again.
			glog.Warningf("While waiting for print jobs: %s", err)
			continue
		}

		if time.Since(lastFailure) < time.Duration(minTimeBetweenXMPPFailures) {
			// We have seen two unaccounted-for errors in a short period of time.
			// This suggests that the XMPP conversation has failed, so let's
			// give up.
			return nil, fmt.Errorf("XMPP conversation failed: %s", err)
		}
		lastFailure = time.Now()
	}

	return gcp.Fetch(gcpID)
}

// NextPrinterWithUpdates gets the GCPID of the next printer with updates
// in the pending state.
func (gcp *GoogleCloudPrint) NextPrinterWithUpdates() (string, error) {
	return gcp.xmppClient.nextPrinterWithUpdates()
}

// Control calls google.com/cloudprint/control to set the status of a
// GCP print job.
func (gcp *GoogleCloudPrint) Control(jobID string, status lib.GCPJobStatus, code, message string) error {
	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("status", string(status))
	form.Set("code", code)
	form.Set("message", message)

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"control", form); err != nil {
		return err
	}

	return nil
}

// Delete calls google.com/cloudprint/delete to delete a printer from GCP.
func (gcp *GoogleCloudPrint) Delete(gcpID string) error {
	form := url.Values{}
	form.Set("printerid", gcpID)

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"delete", form); err != nil {
		return err
	}

	return nil
}

// Fetch calls google.com/cloudprint/fetch to get the outstanding print jobs for
// a GCP printer.
func (gcp *GoogleCloudPrint) Fetch(gcpID string) ([]lib.Job, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, errorCode, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"fetch", form)
	if err != nil {
		if errorCode == 413 {
			// 413 means "Zero print jobs returned", which isn't really an error.
			return []lib.Job{}, nil
		}
		return nil, err
	}

	var jobsData struct {
		Jobs []struct {
			ID        string
			Title     string
			FileURL   string
			TicketURL string
			OwnerID   string
		}
	}
	if err = json.Unmarshal(responseBody, &jobsData); err != nil {
		return nil, err
	}

	jobs := make([]lib.Job, 0, len(jobsData.Jobs))

	for _, jobData := range jobsData.Jobs {
		job := lib.Job{
			GCPPrinterID: gcpID,
			GCPJobID:     jobData.ID,
			FileURL:      jobData.FileURL,
			TicketURL:    jobData.TicketURL,
			OwnerID:      jobData.OwnerID,
			Title:        jobData.Title,
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// List calls google.com/cloudprint/list to get all GCP printers assigned
// to this connector.
//
// The second return value is a map of GCPID -> queued print job quantity.
// The third return value is a map of GCPID -> pending XMPP ping interval changes.
func (gcp *GoogleCloudPrint) List() ([]lib.Printer, map[string]uint, map[string]time.Duration, error) {
	form := url.Values{}
	form.Set("proxy", gcp.proxyName)
	form.Set("extra_fields", "queuedJobsCount")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"list", form)
	if err != nil {
		return nil, nil, nil, err
	}

	var listData struct {
		Printers []struct {
			ID                 string
			Name               string
			DefaultDisplayName string
			Description        string
			Status             string
			CapsHash           string
			LocalSettings      localSettingsPull `json:"local_settings"`
			Tags               []string
			QueuedJobsCount    uint
		}
	}
	if err = json.Unmarshal(responseBody, &listData); err != nil {
		return nil, nil, nil, err
	}

	queuedJobsCount := make(map[string]uint)
	xmppPingIntervalChanges := make(map[string]time.Duration)
	printers := make([]lib.Printer, 0, len(listData.Printers))

	for _, p := range listData.Printers {
		tags := make(map[string]string, len(p.Tags))
		for _, tag := range p.Tags {
			if !strings.HasPrefix(tag, gcpTagPrefix) {
				// This tag is not managed by the CUPS Connector, so ignore it.
				continue
			}
			s := strings.SplitN(tag[len(gcpTagPrefix):], "=", 2)
			key := s[0]
			var value string
			if len(s) > 1 {
				value = s[1]
			}
			tags[key] = value
		}

		var xmppPingInterval time.Duration
		if p.LocalSettings.Pending.XMPPPingInterval > 0 {
			xmppPingIntervalSeconds := p.LocalSettings.Pending.XMPPPingInterval
			xmppPingIntervalChanges[p.ID] = time.Second * time.Duration(xmppPingIntervalSeconds)
			xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
		} else if p.LocalSettings.Current.XMPPPingInterval > 0 {
			xmppPingIntervalSeconds := p.LocalSettings.Current.XMPPPingInterval
			xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
		}

		printer := lib.Printer{
			GCPID:              p.ID,
			Name:               p.Name,
			DefaultDisplayName: p.DefaultDisplayName,
			Description:        p.Description,
			Status:             lib.PrinterStatusFromString(p.Status),
			CapsHash:           p.CapsHash,
			XMPPPingInterval:   xmppPingInterval,
			Tags:               tags,
		}
		printers = append(printers, printer)

		if p.QueuedJobsCount > 0 {
			queuedJobsCount[p.ID] = p.QueuedJobsCount
		}
	}

	return printers, queuedJobsCount, xmppPingIntervalChanges, nil
}

// Register calls google.com/cloudprint/register to register a GCP printer.
//
// Sets the GCPID field in the printer arg.
func (gcp *GoogleCloudPrint) Register(printer *lib.Printer, ppd string) error {
	if len(ppd) <= 0 {
		return errors.New("GCP requires a non-empty PPD")
	}

	cdd, err := gcp.Translate(ppd)
	if err != nil {
		return err
	}

	localSettings, err := marshalLocalSettings(gcp.xmppPingIntervalDefault)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("name", printer.Name)
	form.Set("default_display_name", printer.DefaultDisplayName)
	form.Set("proxy", gcp.proxyName)
	form.Set("local_settings", localSettings)
	form.Set("use_cdd", "true")
	form.Set("capabilities", cdd)
	form.Set("description", printer.Description)
	form.Set("status", string(printer.Status))
	form.Set("capsHash", printer.CapsHash)
	form.Set("content_types", "application/pdf")

	sortedKeys := make([]string, 0, len(printer.Tags))
	for key := range printer.Tags {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		form.Add("tag", fmt.Sprintf("%s%s=%s", gcpTagPrefix, key, printer.Tags[key]))
	}

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"register", form)
	if err != nil {
		return err
	}

	var registerData struct {
		Printers []struct {
			ID string
		}
	}
	if err = json.Unmarshal(responseBody, &registerData); err != nil {
		return err
	}

	printer.GCPID = registerData.Printers[0].ID

	return nil
}

// Update calls google.com/cloudprint/update to update a GCP printer.
func (gcp *GoogleCloudPrint) Update(diff *lib.PrinterDiff, ppd string) error {
	form := url.Values{}
	form.Set("printerid", diff.Printer.GCPID)
	form.Set("proxy", gcp.proxyName)

	// Ignore Name field because it never changes.
	if diff.DefaultDisplayNameChanged {
		form.Set("default_display_name", diff.Printer.DefaultDisplayName)
	}

	if diff.DescriptionChanged {
		form.Set("description", diff.Printer.Description)
	}

	if diff.StatusChanged {
		form.Set("status", string(diff.Printer.Status))
	}

	if diff.CapsHashChanged {
		cdd, err := gcp.Translate(ppd)
		if err != nil {
			return err
		}

		form.Set("use_cdd", "true")
		form.Set("capsHash", diff.Printer.CapsHash)
		form.Set("capabilities", cdd)
	}

	if diff.XMPPPingIntervalChanged {
		localSettings, err := marshalLocalSettings(diff.Printer.XMPPPingInterval)
		if err != nil {
			return err
		}

		form.Set("local_settings", localSettings)
	}

	if diff.TagsChanged {
		sortedKeys := make([]string, 0, len(diff.Printer.Tags))
		for key := range diff.Printer.Tags {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)
		for _, key := range sortedKeys {
			form.Add("tag", fmt.Sprintf("%s%s=%s", gcpTagPrefix, key, diff.Printer.Tags[key]))
		}

		form.Set("remove_tag", gcpTagPrefix+".*")
	}

	if _, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"update", form); err != nil {
		return err
	}

	return nil
}

// SetConnectorXMPPPingInterval sets the interval at which this connector will
// ping the XMPP server.
func (gcp *GoogleCloudPrint) SetConnectorXMPPPingInterval(interval time.Duration) {
	gcp.xmppClient.setPingInterval(interval)
}

// SetPrinterXMPPPingInterval checks GCP for a pending XMPP interval change,
// and applies the change, to one printer.
func (gcp *GoogleCloudPrint) SetPrinterXMPPPingInterval(printer lib.Printer) error {
	diff := lib.PrinterDiff{
		Operation:               lib.UpdatePrinter,
		Printer:                 printer,
		XMPPPingIntervalChanged: true,
	}

	if err := gcp.Update(&diff, ""); err != nil {
		return err
	}

	return nil
}

// Printer gets the printer identified by it's GCPID.
func (gcp *GoogleCloudPrint) Printer(gcpID string) (*lib.Printer, error) {
	form := url.Values{}
	form.Set("printerid", gcpID)

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"printer", form)
	if err != nil {
		return nil, err
	}

	var printersData struct {
		Printers []struct {
			ID                 string
			Name               string
			DefaultDisplayName string
			Description        string
			Status             string
			CapsHash           string
			LocalSettings      localSettingsPull `json:"local_settings"`
			Tags               []string
		}
	}
	if err = json.Unmarshal(responseBody, &printersData); err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	for _, tag := range printersData.Printers[0].Tags {
		s := strings.SplitN(tag, "=", 2)
		key := s[0][6:]
		var value string
		if len(s) > 1 {
			value = s[1]
		}
		tags[key] = value
	}

	var xmppPingInterval time.Duration
	if printersData.Printers[0].LocalSettings.Pending.XMPPPingInterval > 0 {
		xmppPingIntervalSeconds := printersData.Printers[0].LocalSettings.Pending.XMPPPingInterval
		xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
	} else if printersData.Printers[0].LocalSettings.Current.XMPPPingInterval > 0 {
		xmppPingIntervalSeconds := printersData.Printers[0].LocalSettings.Current.XMPPPingInterval
		xmppPingInterval = time.Second * time.Duration(xmppPingIntervalSeconds)
	}

	printer := &lib.Printer{
		GCPID:              printersData.Printers[0].ID,
		Name:               printersData.Printers[0].Name,
		DefaultDisplayName: printersData.Printers[0].DefaultDisplayName,
		Description:        printersData.Printers[0].Description,
		Status:             lib.PrinterStatusFromString(printersData.Printers[0].Status),
		CapsHash:           printersData.Printers[0].CapsHash,
		XMPPPingInterval:   xmppPingInterval,
		Tags:               tags,
	}

	return printer, err
}

// Translate calls google.com/cloudprint/tools/cdd/translate to translate a PPD to CDD.
func (gcp *GoogleCloudPrint) Translate(ppd string) (string, error) {
	form := url.Values{}
	form.Set("capabilities", ppd)

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"tools/cdd/translate", form)
	if err != nil {
		return "", err
	}

	var cddInterface interface{}
	if err = json.Unmarshal(responseBody, &cddInterface); err != nil {
		return "", fmt.Errorf("Failed to unmarshal translated CDD: %s", err)
	}

	cdd, ok := cddInterface.(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}
	cdd, ok = cdd["cdd"].(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}
	p, ok := cdd["printer"].(map[string]interface{})
	if !ok {
		return "", errors.New("Failed to parse translated CDD")
	}

	p["copies"] = map[string]int{
		"max":     100,
		"default": 1,
	}
	p["collate"] = map[string]bool{
		"default": true,
	}

	cddString, err := json.Marshal(cdd)
	if err != nil {
		return "", fmt.Errorf("Failed to remarshal translated CDD: %s", err)
	}

	return string(cddString), nil
}

// Share calls google.com/cloudprint/share to share a registered GCP printer.
func (gcp *GoogleCloudPrint) Share(gcpID, shareScope string) error {
	if gcp.userClient == nil {
		return errors.New("Cannot share because user OAuth credentials not provided.")
	}

	form := url.Values{}
	form.Set("printerid", gcpID)
	form.Set("scope", shareScope)
	form.Set("role", "USER")
	form.Set("skip_notification", "true")

	if _, _, _, err := postWithRetry(gcp.userClient, gcp.baseURL+"share", form); err != nil {
		return err
	}

	return nil
}

// Download downloads a URL (a print job PDF) directly to a Writer.
func (gcp *GoogleCloudPrint) Download(dst io.Writer, url string) error {
	response, err := getWithRetry(gcp.robotClient, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	_, err = io.Copy(dst, response.Body)
	if err != nil {
		return err
	}

	return nil
}
