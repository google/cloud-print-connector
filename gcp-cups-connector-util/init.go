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

	gcpUserOAuthRefreshTokenFlag = flag.String(
		"gcp-user-refresh-token", "",
		"GCP user refresh token, useful when managing many connectors")
	gcpAPITimeoutFlag = flag.Duration(
		"gcp-api-timeout", 5*time.Second,
		"GCP API timeout, for debugging")
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

// getUserClientFromUser steps the user through the process of acquiring an OAuth refresh token.
func getUserClientFromUser(retainUserOAuthToken bool) (*http.Client, string) {
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

	fmt.Println("Login to Google as the user that will own the printers, then visit this URL:")
	fmt.Println("")
	fmt.Println(config.AuthCodeURL("state", oauth2.AccessTypeOffline))
	fmt.Println("")

	authCode := scanNonEmptyString("After authenticating, paste the provided code here:")
	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Acquired OAuth credentials for user account")

	var userRefreshToken string
	if retainUserOAuthToken {
		userRefreshToken = token.RefreshToken
	}

	client := config.Client(oauth2.NoContext, token)
	client.Timeout = *gcpAPITimeoutFlag

	return client, userRefreshToken
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

func createConfigFile(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxy string, localEnable, cloudEnable bool) {
	config := lib.Config{
		xmppJID,
		robotRefreshToken,
		userRefreshToken,
		shareScope,
		proxy,
		flagToUint(gcpMaxConcurrentDownloadsFlag, lib.DefaultConfig.GCPMaxConcurrentDownloads),
		flagToUint(cupsMaxConnectionsFlag, lib.DefaultConfig.CUPSMaxConnections),
		flagToDurationString(cupsConnectTimeoutFlag, lib.DefaultConfig.CUPSConnectTimeout),
		flagToUint(cupsJobQueueSizeFlag, lib.DefaultConfig.CUPSJobQueueSize),
		flagToDurationString(cupsPrinterPollIntervalFlag, lib.DefaultConfig.CUPSPrinterPollInterval),
		lib.DefaultConfig.CUPSPrinterAttributes,
		flagToBool(cupsJobFullUsernameFlag, lib.DefaultConfig.CUPSJobFullUsername),
		flagToBool(cupsIgnoreRawPrintersFlag, lib.DefaultConfig.CUPSIgnoreRawPrinters),
		flagToBool(copyPrinterInfoToDisplayNameFlag, lib.DefaultConfig.CopyPrinterInfoToDisplayName),
		flagToString(monitorSocketFilenameFlag, lib.DefaultConfig.MonitorSocketFilename),
		flagToString(gcpBaseURLFlag, lib.DefaultConfig.GCPBaseURL),
		flagToString(gcpXMPPServerFlag, lib.DefaultConfig.XMPPServer),
		flagToUint16(gcpXMPPPortFlag, lib.DefaultConfig.XMPPPort),
		flagToDurationString(gcpXMPPPingTimeoutFlag, lib.DefaultConfig.XMPPPingTimeout),
		flagToDurationString(gcpXMPPPingIntervalDefaultFlag, lib.DefaultConfig.XMPPPingIntervalDefault),
		flagToString(gcpOAuthClientIDFlag, lib.DefaultConfig.GCPOAuthClientID),
		flagToString(gcpOAuthClientSecretFlag, lib.DefaultConfig.GCPOAuthClientSecret),
		flagToString(gcpOAuthAuthURLFlag, lib.DefaultConfig.GCPOAuthAuthURL),
		flagToString(gcpOAuthTokenURLFlag, lib.DefaultConfig.GCPOAuthTokenURL),
		flagToBool(snmpEnableFlag, lib.DefaultConfig.SNMPEnable),
		flagToString(snmpCommunityFlag, lib.DefaultConfig.SNMPCommunity),
		flagToUint(snmpMaxConnectionsFlag, lib.DefaultConfig.SNMPMaxConnections),
		localEnable,
		cloudEnable,
	}

	if err := config.ToFile(); err != nil {
		log.Fatal(err)
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
				shareScope = scanNonEmptyString("User or group email address, or domain name, to share with:")
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
		userRefreshToken = flagToString(gcpUserOAuthRefreshTokenFlag, "")
		if userRefreshToken == "" {
			userClient, userRefreshToken = getUserClientFromUser(retainUserOAuthToken)
		} else {
			userClient = getUserClientFromToken(userRefreshToken)
		}

		xmppJID, robotRefreshToken = createRobotAccount(userClient)

		fmt.Println("Acquired OAuth credentials for robot account")
		fmt.Println("")
	}

	createConfigFile(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName, localEnable, cloudEnable)
	fmt.Printf("The config file %s is ready to rock.\n", *lib.ConfigFilename)
	if cloudEnable {
		fmt.Println("Keep it somewhere safe, as it contains an OAuth refresh token.")
	}

	socketDirectory := filepath.Dir(flagToString(monitorSocketFilenameFlag, lib.DefaultConfig.MonitorSocketFilename))
	if _, err := os.Stat(socketDirectory); os.IsNotExist(err) {
		fmt.Println("")
		fmt.Printf("When the connector runs, be sure the socket directory %s exists.\n", socketDirectory)
	}
}
