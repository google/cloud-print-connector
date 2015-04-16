/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Package gcp is the Google Cloud Print API client.
package gcp

import (
	"bytes"
	"cups-connector/lib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

var (
	re_man = regexp.MustCompile(`(?m)^\*Manufacturer:\s+"(.+)"\s*$`)
	re_mod = regexp.MustCompile(`(?m)^\*ModelName:\s+"(.+)"\s*$`)
)

// GoogleCloudPrint is the interface between Go and the Google Cloud Print API.
type GoogleCloudPrint struct {
	baseURL     string
	xmppJID     string
	xmpp        *XMPP
	robotClient *http.Client
	userClient  *http.Client
	proxyName   string

	xmppServer              string
	xmppPort                uint16
	xmppPingTimeout         time.Duration
	xmppPingIntervalDefault time.Duration

	xmppPrintersJobs        chan string
	xmppPrintersUpdates     chan string
	xmppPingIntervalUpdates chan time.Duration
	xmppDead                chan interface{}

	quit chan interface{}
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

	gcp := &GoogleCloudPrint{
		baseURL:                 baseURL,
		xmppJID:                 xmppJID,
		xmpp:                    nil,
		robotClient:             robotClient,
		userClient:              userClient,
		proxyName:               proxyName,
		xmppServer:              xmppServer,
		xmppPort:                xmppPort,
		xmppPingTimeout:         xmppPingTimeout,
		xmppPingIntervalDefault: xmppPingIntervalDefault,
		xmppPrintersJobs:        make(chan string, 10),
		xmppPrintersUpdates:     make(chan string, 10),
		xmppPingIntervalUpdates: make(chan time.Duration, 1),
		xmppDead:                make(chan interface{}),
		quit:                    make(chan interface{}),
	}

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

// Turn arguments into a JSON-encoded GCP LocalSettings message.
func marshalLocalSettings(xmppInterval time.Duration) (string, error) {
	var ls localSettingsPush
	ls.Current.XMPPPingInterval = uint32(xmppInterval.Seconds())
	lss, err := json.Marshal(ls)
	if err != nil {
		return "", err
	}
	return string(lss), nil
}

// cloudDeviceState represents a CloudDeviceState message.
type cloudDeviceState struct {
	Version string              `json:"version"`
	Printer printerStateSection `json:"printer"`
}

// printerStateSection represents a CDS PrinterStateSection message.
type printerStateSection struct {
	State       string      `json:"state"`
	VendorState vendorState `json:"vendor_state"`
	// TODO add other GCP 2.0 fields.
}

// vendorState represents a CDS VendorState message.
type vendorState struct {
	Items []vendorStateItem `json:"item"`
}

// vendorStateItem represents a CDS VendorState.Item message.
type vendorStateItem struct {
	State       string `json:"state"`
	Description string `json:"description"`
}

// marshalSemanticState turns state and reasons into a JSON-encoded GCP CloudDeviceState message.
func marshalSemanticState(state lib.PrinterState, reasons []string) (string, error) {
	vendorStateItems := make([]vendorStateItem, 0, len(reasons))
	for _, reason := range reasons {
		reasonSplit := strings.Split(reason, "-")
		reasonSuffix := reasonSplit[len(reasonSplit)-1]
		var vendorStateType string
		switch reasonSuffix {
		case "error":
			vendorStateType = "ERROR"
		case "warning":
			vendorStateType = "WARNING"
		case "report":
			vendorStateType = "INFO"
		default:
			vendorStateType = "INFO"
		}
		item := vendorStateItem{
			State:       vendorStateType,
			Description: reason,
		}
		vendorStateItems = append(vendorStateItems, item)
	}

	semanticState := cloudDeviceState{
		Version: "1.0",
		Printer: printerStateSection{
			State: state.GCPPrinterState(),
			VendorState: vendorState{
				Items: vendorStateItems,
			},
		},
	}

	ss, err := json.Marshal(semanticState)
	if err != nil {
		return "", err
	}
	return string(ss), nil
}

func unmarshalSemanticState(semanticState cloudDeviceState) (lib.PrinterState, []string) {
	state := lib.PrinterStateFromGCP(semanticState.Printer.State)
	stateReasons := make([]string, len(semanticState.Printer.VendorState.Items))
	for _, item := range semanticState.Printer.VendorState.Items {
		stateReasons = append(stateReasons, item.Description)
	}
	return state, stateReasons
}

// parseManufacturerAndModel finds the *Manufacturer and *ModelName values in a PPD string.
func parseManufacturerAndModel(ppd string) (string, string) {
	res := re_man.FindStringSubmatch(ppd)
	man := res[1]
	res = re_mod.FindStringSubmatch(ppd)
	mod := res[1]

	if man == "" {
		man = "Unknown"
	} else if mod == "" {
		mod = "Unknown"
	} else if strings.HasPrefix(mod, man) && len(mod) > len(man) {
		// ModelName starts with Manufacturer (as it should).
		// Remove Manufacturer from ModelName.
		mod = strings.TrimPrefix(mod, man)
		mod = strings.TrimSpace(mod)
	}

	return man, mod
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (gcp *GoogleCloudPrint) Quit() {
	if gcp.xmpp != nil {
		// Signal to KeepXMPPAlive.
		gcp.quit <- new(interface{})
		select {
		case <-gcp.xmppDead:
			// Wait for XMPP to die.
		case <-time.After(5 * time.Second):
			// But not too long.
			glog.Error("XMPP taking a while to close, so giving up")
		}
	}
}

// CanShare answers the question "can we share printers when they are registered?"
func (gcp *GoogleCloudPrint) CanShare() bool {
	return gcp.userClient != nil
}

// StartXMPP tries to start an XMPP conversation.
// Tries multiple times before returning an error.
func (gcp *GoogleCloudPrint) StartXMPP() error {
	if gcp.xmpp != nil {
		go gcp.xmpp.Quit()
	}

	var err error
	for i := 0; i < restartXMPPMaxRetries; i++ {
		// The current access token is the XMPP password.
		var token *oauth2.Token
		token, err = gcp.robotClient.Transport.(*oauth2.Transport).Source.Token()
		if err == nil {
			var xmpp *XMPP
			xmpp, err = NewXMPP(gcp.xmppJID, token.AccessToken, gcp.proxyName,
				gcp.xmppServer, gcp.xmppPort, gcp.xmppPingTimeout, gcp.xmppPingIntervalDefault,
				gcp.xmppPrintersJobs, gcp.xmppPrintersUpdates, gcp.xmppPingIntervalUpdates, gcp.xmppDead)

			if err == nil {
				// Success!
				gcp.xmpp = xmpp
				// Don't give up.
				go gcp.keepXMPPAlive()
				return nil
			}
		}

		// Sleep for 1, 2, 4, 8 seconds.
		time.Sleep(time.Duration((i+1)*2) * time.Second)
	}

	return fmt.Errorf("Failed to start XMPP conversation: %s", err)
}

// KeepXMPPAlive restarts XMPP when it fails.
func (gcp *GoogleCloudPrint) keepXMPPAlive() {
	for {
		select {
		case <-gcp.xmppDead:
			glog.Error("XMPP conversation died; restarting")
			if err := gcp.StartXMPP(); err != nil {
				glog.Fatalf("Failed to keep XMPP conversation alive: %s", err)
			}
		case <-gcp.quit:
			// Close XMPP.
			gcp.xmpp.Quit()
			return
		}
	}
}

// NextJobBatch gets the next batch of print jobs from GCP. Blocks on XMPP until
// batch notification arrives. Calls Fetch to get the jobs.
func (gcp *GoogleCloudPrint) NextJobBatch() ([]lib.Job, error) {
	gcpID := <-gcp.xmppPrintersJobs
	return gcp.Fetch(gcpID)
}

// NextPrinterWithUpdates gets the GCPID of the next printer with updates
// in the pending state.
func (gcp *GoogleCloudPrint) NextPrinterWithUpdates() string {
	return <-gcp.xmppPrintersUpdates
}

// printJobStateDiff represents a CJS PrintJobStateDiff message.
type printJobStateDiff struct {
	State        jobState `json:"state"`
	PagesPrinted uint32   `json:"pages_printed"`
}

// jobState represents a CJS JobState message.
type jobState struct {
	Type              string             `json:"type"`
	UserActionCause   *userActionCause   `json:"user_action_cause,omitempty"`
	DeviceActionCause *deviceActionCause `json:"device_action_cause,omitempty"`
}

// userActionCause represents a CJS JobState.UserActionCause message.
type userActionCause struct {
	ActionCode string `json:"action_code"`
}

// deviceActionCause represents a CJS JobState.DeviceActionCause message.
type deviceActionCause struct {
	ErrorCode string `json:"error_code"`
}

// Control calls google.com/cloudprint/control to set the state of a
// GCP print job.
func (gcp *GoogleCloudPrint) Control(jobID string, state lib.GCPJobState, cause lib.GCPJobStateCause, pages uint32) error {
	var semanticState printJobStateDiff
	if cause == lib.GCPJobCanceled {
		semanticState = printJobStateDiff{
			State: jobState{
				Type: state.String(),
				UserActionCause: &userActionCause{
					ActionCode: cause.String(),
				},
			},
			PagesPrinted: pages,
		}
	} else if state == lib.GCPJobAborted || state == lib.GCPJobStopped {
		semanticState = printJobStateDiff{
			State: jobState{
				Type: state.String(),
				DeviceActionCause: &deviceActionCause{
					ErrorCode: cause.String(),
				},
			},
			PagesPrinted: pages,
		}
	} else {
		semanticState = printJobStateDiff{
			State: jobState{
				Type: state.String(),
			},
			PagesPrinted: pages,
		}
	}

	ss, err := json.Marshal(semanticState)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("jobid", jobID)
	form.Set("semantic_state_diff", string(ss))

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
			ID      string
			Title   string
			FileURL string
			OwnerID string
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
	form.Set("extra_fields", "queuedJobsCount,semanticState")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"list", form)
	if err != nil {
		return nil, nil, nil, err
	}

	var listData struct {
		Printers []struct {
			ID                 string
			Name               string
			DefaultDisplayName string
			CapsHash           string
			LocalSettings      localSettingsPull `json:"local_settings"`
			Tags               []string
			QueuedJobsCount    uint
			SemanticState      cloudDeviceState `json:"semantic_state"`
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

		state, stateReasons := unmarshalSemanticState(p.SemanticState)

		printer := lib.Printer{
			GCPID:              p.ID,
			Name:               p.Name,
			DefaultDisplayName: p.DefaultDisplayName,
			State:              state,
			StateReasons:       stateReasons,
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

	manufacturer, model := parseManufacturerAndModel(ppd)

	localSettings, err := marshalLocalSettings(gcp.xmppPingIntervalDefault)
	if err != nil {
		return err
	}

	semanticState, err := marshalSemanticState(printer.State, printer.StateReasons)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("name", printer.Name)
	form.Set("default_display_name", printer.DefaultDisplayName)
	form.Set("proxy", gcp.proxyName)
	form.Set("uuid", printer.Name) // CUPS doesn't provide serial number.
	form.Set("manufacturer", manufacturer)
	form.Set("model", model)
	form.Set("gcp_version", "2.0")
	form.Set("setup_url", lib.ConnectorHomeURL)
	form.Set("support_url", lib.ConnectorHomeURL)
	form.Set("update_url", lib.ConnectorHomeURL)
	form.Set("firmware", "CUPS Connector "+lib.GetBuildDate())
	form.Set("local_settings", localSettings)
	form.Set("semantic_state", semanticState)
	form.Set("use_cdd", "true")
	form.Set("capabilities", cdd)
	form.Set("capsHash", printer.CapsHash)

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
	form.Set("gcp_version", "2.0")
	form.Set("firmware", "CUPS Connector "+lib.GetBuildDate())

	// Ignore Name field because it never changes.
	if diff.DefaultDisplayNameChanged {
		form.Set("default_display_name", diff.Printer.DefaultDisplayName)
	}

	if diff.StateChanged {
		semanticState, err := marshalSemanticState(diff.Printer.State, diff.Printer.StateReasons)
		if err != nil {
			return err
		}
		form.Set("semantic_state", semanticState)
	}

	if diff.CapsHashChanged {
		cdd, err := gcp.Translate(ppd)
		if err != nil {
			return err
		}

		manufacturer, model := parseManufacturerAndModel(ppd)

		form.Set("manufacturer", manufacturer)
		form.Set("model", model)
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
	gcp.xmppPingIntervalUpdates <- interval
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
	form.Set("use_cdd", "true")
	form.Set("extra_fields", "semanticState")

	responseBody, _, _, err := postWithRetry(gcp.robotClient, gcp.baseURL+"printer", form)
	if err != nil {
		return nil, err
	}

	var printersData struct {
		Printers []struct {
			ID                 string
			Name               string
			DefaultDisplayName string
			CapsHash           string
			LocalSettings      localSettingsPull `json:"local_settings"`
			Tags               []string
			SemanticState      cloudDeviceState `json:"semantic_state"`
		}
	}
	if err = json.Unmarshal(responseBody, &printersData); err != nil {
		return nil, err
	}

	state, stateReasons := unmarshalSemanticState(printersData.Printers[0].SemanticState)

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
		State:              state,
		StateReasons:       stateReasons,
		CapsHash:           printersData.Printers[0].CapsHash,
		Tags:               tags,
		XMPPPingInterval:   xmppPingInterval,
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

	d := json.NewDecoder(bytes.NewReader(responseBody))
	d.UseNumber() // Force large numbers to be formatted not in scientific notation.

	var cddInterface interface{}
	if err = d.Decode(&cddInterface); err != nil {
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
	p["supported_content_type"] = []map[string]string{
		map[string]string{
			"content_type": "application/pdf",
		},
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
