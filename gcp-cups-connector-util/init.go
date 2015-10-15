/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/cups-connector/gcp"
	"github.com/google/cups-connector/lib"

	"golang.org/x/oauth2"
)

// All flags are string type. This makes parsing "not set" easier, and
// allows default values to be separated.
var (
	retainUserOAuthTokenFlag = flag.String(
		"retain-user-oauth-token", "",
		"Whether to retain the user's OAuth token to enable automatic sharing (true/false)")
	shareScopeFlag = flag.String(
		"share-scope", "",
		"Scope (user, group, domain) to share printers with")
	proxyNameFlag = flag.String(
		"proxy-name", "",
		"User-chosen name of this proxy. Should be unique per Google user account")
	gcpMaxConcurrentDownloadsFlag = flag.String(
		"gcp-max-concurrent-downloads", "",
		"Maximum quantity of PDFs to download concurrently")
	cupsMaxConnectionsFlag = flag.String(
		"cups-max-connections", "",
		"Max connections to CUPS server")
	cupsConnectTimeoutFlag = flag.String(
		"cups-connect-timeout", "",
		"CUPS timeout for opening a new connection")
	cupsJobQueueSizeFlag = flag.String(
		"cups-job-queue-size", "",
		"CUPS job queue size")
	cupsPrinterPollIntervalFlag = flag.String(
		"cups-printer-poll-interval", "",
		"Interval, in seconds, between CUPS printer state polls")
	cupsJobFullUsernameFlag = flag.String(
		"cups-job-full-username", "",
		"Whether to use the full username (joe@example.com) in CUPS jobs")
	cupsIgnoreRawPrintersFlag = flag.String(
		"cups-ignore-raw-printers", "",
		"Whether to ignore raw printers")
	copyPrinterInfoToDisplayNameFlag = flag.String(
		"copy-printer-info-to-display-name", "",
		"Whether to copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName")
	prefixJobIDToJobTitleFlag = flag.String(
		"prefix-job-id-to-job-title", "",
		"Whether to add the job ID to the beginning of the job title")
	displayNamePrefixFlag = flag.String(
		"display-name-prefix", "",
		"Prefix to add to GCP printer's defaultDisplayName")
	monitorSocketFilenameFlag = flag.String(
		"socket-filename", "",
		"Filename of unix socket for connector-check to talk to connector")
	gcpBaseURLFlag = flag.String(
		"gcp-base-url", "",
		"GCP API base URL")
	gcpXMPPServerFlag = flag.String(
		"gcp-xmpp-server", "",
		"GCP XMPP server FQDN")
	gcpXMPPPortFlag = flag.String(
		"gcp-xmpp-port", "",
		"GCP XMPP port number")
	gcpXMPPPingTimeoutFlag = flag.String(
		"gcp-xmpp-ping-timeout", "",
		"GCP XMPP ping timeout (give up waiting for ping response after this)")
	gcpXMPPPingIntervalDefaultFlag = flag.String(
		"gcp-xmpp-ping-interval-default", "",
		"GCP XMPP ping interval default (ping every this often)")
	gcpOAuthClientIDFlag = flag.String(
		"gcp-oauth-client-id", "",
		"GCP OAuth client ID")
	gcpOAuthClientSecretFlag = flag.String(
		"gcp-oauth-client-secret", "",
		"GCP OAuth client secret")
	gcpOAuthAuthURLFlag = flag.String(
		"gcp-oauth-auth-url", "",
		"GCP OAuth auth URL")
	gcpOAuthTokenURLFlag = flag.String(
		"gcp-oauth-token-url", "",
		"GCP OAuth token URL")
	snmpEnableFlag = flag.String(
		"snmp-enable", "",
		"SNMP enable")
	snmpCommunityFlag = flag.String(
		"snmp-community", "",
		"SNMP community (usually \"public\")")
	snmpMaxConnectionsFlag = flag.String(
		"snmp-max-connections", "",
		"Max connections to SNMP agents")
	localPrintingEnableFlag = flag.String(
		"local-printing-enable", "",
		"Enable local discovery and printing")
	cloudPrintingEnableFlag = flag.String(
		"cloud-printing-enable", "",
		"Enable cloud discovery and printing")
	logFileNameFlag = flag.String(
		"log-file-name", "",
		"Log file name, including directory")
	logFileMaxMegabytesFlag = flag.String(
		"log-file-max-megabytes", "",
		"Log file max size, in megabytes")
	logMaxFilesFlag = flag.String(
		"log-max-files", "",
		"Maximum log files")
	logLevelFlag = flag.String(
		"log-level", "",
		"Minimum event severity to log: PANIC, ERROR, WARN, INFO, DEBUG, VERBOSE")

	gcpUserOAuthRefreshTokenFlag = flag.String(
		"gcp-user-refresh-token", "",
		"GCP user refresh token, useful when managing many connectors")
	gcpAPITimeoutFlag = flag.Duration(
		"gcp-api-timeout", 30*time.Second,
		"GCP API timeout, for debugging")
	gcpOAuthDeviceCodeURL = flag.String(
		"gcp-oauth-device-code-url", "https://accounts.google.com/o/oauth2/device/code",
		"GCP OAuth device code URL")
	gcpOAuthTokenPollURL = flag.String(
		"gcp-oauth-token-poll-url", "https://www.googleapis.com/oauth2/v3/token",
		"GCP OAuth token poll URL")
)

const (
	gcpOAuthGrantTypeDevice = "http://oauth.net/grant_type/device/1.0"
)

// flagToUint returns the value of a flag, or its default, as a uint.
// Panics if string is not properly formatted as a uint value.
func flagToUint(flag *string, defaultValue uint) uint {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	value, err := strconv.ParseUint(*flag, 10, 0)
	if err != nil {
		panic(err)
	}

	return uint(value)
}

// flagToUint16 returns the value of a flag, or its default, as a uint16.
// Panics if string is not properly formatted as a uint16 value.
func flagToUint16(flag *string, defaultValue uint16) uint16 {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	value, err := strconv.ParseUint(*flag, 10, 16)
	if err != nil {
		panic(err)
	}

	return uint16(value)
}

// flagToUint64 returns the value of a flag, or its default, as a uint64.
// Panics if string is not properly formatted as a uint64 value.
func flagToUint64(flag *string, defaultValue uint64) uint64 {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	value, err := strconv.ParseUint(*flag, 10, 64)
	if err != nil {
		panic(err)
	}

	return uint64(value)
}

// flagToBool returns the value of a flag, or it's default, as a bool.
// Panics if string is not properly formatted as a bool value.
func flagToBool(flag *string, defaultValue bool) bool {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(*flag)
	if err != nil {
		panic(err)
	}

	return value
}

// flagToString returns the value of a flag, or it's default, as a string.
func flagToString(flag *string, defaultValue string) string {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	return *flag
}

// flagToString returns the value of a flag, or it's default, as a string.
// Panics if string is not properly formatted as a time.Duration string.
func flagToDurationString(flag *string, defaultValue string) string {
	if flag == nil {
		panic("Flag pointer is nil")
	}

	if *flag == "" {
		return defaultValue
	}

	if _, err := time.ParseDuration(*flag); err != nil {
		panic(err)
	}

	return *flag
}

// getUserClientFromUser follows the token acquisition steps outlined here:
// https://developers.google.com/identity/protocols/OAuth2ForDevices
func getUserClientFromUser() (*http.Client, string) {
	form := url.Values{
		"client_id": {flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID)},
		"scope":     {gcp.ScopeCloudPrint},
	}
	response, err := http.PostForm(*gcpOAuthDeviceCodeURL, form)
	if err != nil {
		log.Fatal(err)
	}

	var r struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	json.NewDecoder(response.Body).Decode(&r)

	fmt.Printf("Visit %s, and enter this code. I'll wait for you.\n%s\n",
		r.VerificationURL, r.UserCode)

	return pollOAuthConfirmation(r.DeviceCode, r.Interval)
}

func pollOAuthConfirmation(deviceCode string, interval int) (*http.Client, string) {
	config := oauth2.Config{
		ClientID:     flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID),
		ClientSecret: flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret),
		Endpoint: oauth2.Endpoint{
			AuthURL:  flagToString(gcpOAuthAuthURLFlag, lib.DefaultConfig.GCPOAuthAuthURL),
			TokenURL: flagToString(gcpOAuthTokenURLFlag, lib.DefaultConfig.GCPOAuthTokenURL),
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint},
	}

	for {
		time.Sleep(time.Duration(interval) * time.Second)

		form := url.Values{
			"client_id":     {flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID)},
			"client_secret": {flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret)},
			"code":          {deviceCode},
			"grant_type":    {gcpOAuthGrantTypeDevice},
		}
		response, err := http.PostForm(*gcpOAuthTokenPollURL, form)
		if err != nil {
			log.Fatal(err)
		}

		var r struct {
			Error        string `json:"error"`
			AccessToken  string `json:"access_token"`
			ExpiresIn    int    `json:"expires_in"`
			RefreshToken string `json:"refresh_token"`
		}
		json.NewDecoder(response.Body).Decode(&r)

		switch r.Error {
		case "":
			token := &oauth2.Token{RefreshToken: r.RefreshToken}
			client := config.Client(oauth2.NoContext, token)
			client.Timeout = *gcpAPITimeoutFlag
			return client, r.RefreshToken
		case "authorization_pending":
		case "slow_down":
			interval *= 2
		default:
			log.Fatal(r)
		}
	}

	panic("unreachable")
}

// getUserClientFromToken creates a user client with just a refresh token.
func getUserClientFromToken(userRefreshToken string) *http.Client {
	config := &oauth2.Config{
		ClientID:     flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID),
		ClientSecret: flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret),
		Endpoint: oauth2.Endpoint{
			AuthURL:  flagToString(gcpOAuthAuthURLFlag, lib.DefaultConfig.GCPOAuthAuthURL),
			TokenURL: flagToString(gcpOAuthTokenURLFlag, lib.DefaultConfig.GCPOAuthTokenURL),
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint},
	}

	token := &oauth2.Token{RefreshToken: userRefreshToken}
	client := config.Client(oauth2.NoContext, token)
	client.Timeout = *gcpAPITimeoutFlag

	return client
}

// initRobotAccount creates a GCP robot account for this connector.
func initRobotAccount(userClient *http.Client) (string, string) {
	params := url.Values{}
	params.Set("oauth_client_id", flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID))

	url := fmt.Sprintf("%s%s?%s", flagToString(gcpBaseURLFlag, lib.DefaultConfig.GCPBaseURL), "createrobot", params.Encode())
	response, err := userClient.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		log.Fatal("failed to initialize robot account: " + response.Status)
	}

	var robotInit struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		XMPPJID  string `json:"xmpp_jid"`
		AuthCode string `json:"authorization_code"`
	}

	if err = json.NewDecoder(response.Body).Decode(&robotInit); err != nil {
		log.Fatal(err)
	}
	if !robotInit.Success {
		log.Fatal("failed to initialize robot account: " + robotInit.Message)
	}

	return robotInit.XMPPJID, robotInit.AuthCode
}

func verifyRobotAccount(authCode string) string {
	config := &oauth2.Config{
		ClientID:     flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID),
		ClientSecret: flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret),
		Endpoint: oauth2.Endpoint{
			AuthURL:  flagToString(gcpOAuthAuthURLFlag, lib.DefaultConfig.GCPOAuthAuthURL),
			TokenURL: flagToString(gcpOAuthTokenURLFlag, lib.DefaultConfig.GCPOAuthTokenURL),
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint, gcp.ScopeGoogleTalk},
	}

	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatal(err)
	}

	return token.RefreshToken
}

func createRobotAccount(userClient *http.Client) (string, string) {
	xmppJID, authCode := initRobotAccount(userClient)
	token := verifyRobotAccount(authCode)

	return xmppJID, token
}

// createCloudConfig creates a config object that supports cloud and (optionally) local mode.
func createCloudConfig(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName string, localEnable bool) *lib.Config {
	return &lib.Config{
		XMPPJID:                   xmppJID,
		RobotRefreshToken:         robotRefreshToken,
		UserRefreshToken:          userRefreshToken,
		ShareScope:                shareScope,
		ProxyName:                 proxyName,
		XMPPServer:                flagToString(gcpXMPPServerFlag, lib.DefaultConfig.XMPPServer),
		XMPPPort:                  flagToUint16(gcpXMPPPortFlag, lib.DefaultConfig.XMPPPort),
		XMPPPingTimeout:           flagToDurationString(gcpXMPPPingTimeoutFlag, lib.DefaultConfig.XMPPPingTimeout),
		XMPPPingIntervalDefault:   flagToDurationString(gcpXMPPPingIntervalDefaultFlag, lib.DefaultConfig.XMPPPingIntervalDefault),
		GCPBaseURL:                flagToString(gcpBaseURLFlag, lib.DefaultConfig.GCPBaseURL),
		GCPOAuthClientID:          flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID),
		GCPOAuthClientSecret:      flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret),
		GCPOAuthAuthURL:           flagToString(gcpOAuthAuthURLFlag, lib.DefaultConfig.GCPOAuthAuthURL),
		GCPOAuthTokenURL:          flagToString(gcpOAuthTokenURLFlag, lib.DefaultConfig.GCPOAuthTokenURL),
		GCPMaxConcurrentDownloads: flagToUint(gcpMaxConcurrentDownloadsFlag, lib.DefaultConfig.GCPMaxConcurrentDownloads),

		CUPSMaxConnections:           flagToUint(cupsMaxConnectionsFlag, lib.DefaultConfig.CUPSMaxConnections),
		CUPSConnectTimeout:           flagToDurationString(cupsConnectTimeoutFlag, lib.DefaultConfig.CUPSConnectTimeout),
		CUPSJobQueueSize:             flagToUint(cupsJobQueueSizeFlag, lib.DefaultConfig.CUPSJobQueueSize),
		CUPSPrinterPollInterval:      flagToDurationString(cupsPrinterPollIntervalFlag, lib.DefaultConfig.CUPSPrinterPollInterval),
		CUPSPrinterAttributes:        lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:          flagToBool(cupsJobFullUsernameFlag, lib.DefaultConfig.CUPSJobFullUsername),
		CUPSIgnoreRawPrinters:        flagToBool(cupsIgnoreRawPrintersFlag, lib.DefaultConfig.CUPSIgnoreRawPrinters),
		CopyPrinterInfoToDisplayName: flagToBool(copyPrinterInfoToDisplayNameFlag, lib.DefaultConfig.CopyPrinterInfoToDisplayName),
		PrefixJobIDToJobTitle:        flagToBool(prefixJobIDToJobTitleFlag, lib.DefaultConfig.PrefixJobIDToJobTitle),
		DisplayNamePrefix:            flagToString(displayNamePrefixFlag, lib.DefaultConfig.DisplayNamePrefix),
		MonitorSocketFilename:        flagToString(monitorSocketFilenameFlag, lib.DefaultConfig.MonitorSocketFilename),
		SNMPEnable:                   flagToBool(snmpEnableFlag, lib.DefaultConfig.SNMPEnable),
		SNMPCommunity:                flagToString(snmpCommunityFlag, lib.DefaultConfig.SNMPCommunity),
		SNMPMaxConnections:           flagToUint(snmpMaxConnectionsFlag, lib.DefaultConfig.SNMPMaxConnections),
		LocalPrintingEnable:          localEnable,
		CloudPrintingEnable:          true,
	}
}

// createLocalConfig creates a config object that supports local mode.
func createLocalConfig() *lib.Config {
	return &lib.Config{
		CUPSMaxConnections:           flagToUint(cupsMaxConnectionsFlag, lib.DefaultConfig.CUPSMaxConnections),
		CUPSConnectTimeout:           flagToDurationString(cupsConnectTimeoutFlag, lib.DefaultConfig.CUPSConnectTimeout),
		CUPSJobQueueSize:             flagToUint(cupsJobQueueSizeFlag, lib.DefaultConfig.CUPSJobQueueSize),
		CUPSPrinterPollInterval:      flagToDurationString(cupsPrinterPollIntervalFlag, lib.DefaultConfig.CUPSPrinterPollInterval),
		CUPSPrinterAttributes:        lib.DefaultConfig.CUPSPrinterAttributes,
		CUPSJobFullUsername:          flagToBool(cupsJobFullUsernameFlag, lib.DefaultConfig.CUPSJobFullUsername),
		CUPSIgnoreRawPrinters:        flagToBool(cupsIgnoreRawPrintersFlag, lib.DefaultConfig.CUPSIgnoreRawPrinters),
		CopyPrinterInfoToDisplayName: flagToBool(copyPrinterInfoToDisplayNameFlag, lib.DefaultConfig.CopyPrinterInfoToDisplayName),
		PrefixJobIDToJobTitle:        flagToBool(prefixJobIDToJobTitleFlag, lib.DefaultConfig.PrefixJobIDToJobTitle),
		DisplayNamePrefix:            flagToString(displayNamePrefixFlag, lib.DefaultConfig.DisplayNamePrefix),
		MonitorSocketFilename:        flagToString(monitorSocketFilenameFlag, lib.DefaultConfig.MonitorSocketFilename),
		SNMPEnable:                   flagToBool(snmpEnableFlag, lib.DefaultConfig.SNMPEnable),
		SNMPCommunity:                flagToString(snmpCommunityFlag, lib.DefaultConfig.SNMPCommunity),
		SNMPMaxConnections:           flagToUint(snmpMaxConnectionsFlag, lib.DefaultConfig.SNMPMaxConnections),
		LocalPrintingEnable:          true,
		CloudPrintingEnable:          false,
	}
}

func writeConfigFile(config *lib.Config) string {
	if configFilename, err := config.ToFile(); err != nil {
		log.Fatal(err)
		panic("unreachable")
	} else {
		return configFilename
	}
}

func scanNonEmptyString(prompt string) string {
	for {
		var answer string
		fmt.Println(prompt)
		if length, err := fmt.Scan(&answer); err != nil {
			log.Fatal(err)
		} else if length > 0 {
			fmt.Println("")
			return answer
		}
	}
	panic("unreachable")
}

func scanYesOrNo(question string) bool {
	for {
		var answer string
		fmt.Println(question)
		if _, err := fmt.Scan(&answer); err != nil {
			log.Fatal(err)
		} else if parsed, value := stringToBool(answer); parsed {
			fmt.Println("")
			return value
		}
	}
	panic("unreachable")
}

// The first return value is true if a boolean value could be parsed.
// The second return value is the parsed boolean value if the first return value is true.
func stringToBool(val string) (bool, bool) {
	if len(val) > 0 {
		switch strings.ToLower(val[0:1]) {
		case "y", "t", "1":
			return true, true
		case "n", "f", "0":
			return true, false
		default:
			return false, true
		}
	}
	return false, false
}

func initConfigFile() {
	var localEnable bool
	if len(*localPrintingEnableFlag) < 1 {
		fmt.Println("\"Local printing\" means that clients print directly to the connector via local subnet,")
		fmt.Println("and that an Internet connection is neither necessary nor used.")
		localEnable = scanYesOrNo("Enable local printing?")
	} else {
		localEnable = flagToBool(localPrintingEnableFlag, false)
	}

	var cloudEnable bool
	if len(*cloudPrintingEnableFlag) < 1 {
		fmt.Println("\"Cloud printing\" means that clients can print from anywhere on the Internet,")
		fmt.Println("and that printers must be explicitly shared with users.")
		cloudEnable = scanYesOrNo("Enable cloud printing?")
	} else {
		cloudEnable = flagToBool(cloudPrintingEnableFlag, false)
	}

	if !localEnable && !cloudEnable {
		log.Fatal("Try again. Either local or cloud (or both) must be enabled for the connector to do something.")
	}

	var config *lib.Config

	var xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName string
	if cloudEnable {
		var parsed bool
		var retainUserOAuthToken bool
		if parsed, retainUserOAuthToken = stringToBool(*retainUserOAuthTokenFlag); !parsed {
			retainUserOAuthToken = scanYesOrNo(
				"Retain the user OAuth token to enable automatic sharing?")
		}

		if retainUserOAuthToken {
			if len(*shareScopeFlag) > 0 {
				shareScope = *shareScopeFlag
			} else {
				shareScope = scanNonEmptyString("User or group email address to share with:")
			}
		} else {
			fmt.Println(
				"The user account OAuth token will be thrown away; printers will not be shared automatically.")
		}

		proxyName = *proxyNameFlag
		if len(proxyName) < 1 {
			proxyName = scanNonEmptyString("Proxy name for this GCP CUPS Connector:")
		}

		var userClient *http.Client
		urt := flagToString(gcpUserOAuthRefreshTokenFlag, "")
		if urt == "" {
			userClient, urt = getUserClientFromUser()
		} else {
			userClient = getUserClientFromToken(urt)
		}

		if retainUserOAuthToken {
			userRefreshToken = urt
		}

		xmppJID, robotRefreshToken = createRobotAccount(userClient)

		fmt.Println("Acquired OAuth credentials for robot account")
		fmt.Println("")
		config = createCloudConfig(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName, localEnable)

	} else {
		config = createLocalConfig()
	}

	configFilename := writeConfigFile(config)
	fmt.Printf("The config file %s is ready to rock.\n", configFilename)
	if cloudEnable {
		fmt.Println("Keep it somewhere safe, as it contains an OAuth refresh token.")
	}

	socketDirectory := filepath.Dir(flagToString(monitorSocketFilenameFlag, lib.DefaultConfig.MonitorSocketFilename))
	if _, err := os.Stat(socketDirectory); os.IsNotExist(err) {
		fmt.Println("")
		fmt.Printf("When the connector runs, be sure the socket directory %s exists.\n", socketDirectory)
	}
}
