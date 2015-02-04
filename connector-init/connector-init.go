/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package main

import (
	"cups-connector/gcp"
	"cups-connector/lib"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

var (
	defaultPrinterAttributes = []string{
		"printer-name",
		"printer-info",
		"printer-is-accepting-jobs",
		"printer-location",
		"printer-make-and-model",
		"printer-state",
		"printer-state-reasons",
	}

	retainUserOauthTokenFlag = flag.String(
		"retain-user-oauth-token", "",
		"Whether to retain the user's OAuth token to enable automatic sharing (true/false)")
	shareScopeFlag = flag.String(
		"share-scope", "",
		"Scope (user, group, domain) to share printers with")
	proxyNameFlag = flag.String(
		"proxy-name", "",
		"User-chosen name of this proxy. Should be unique per Google user account")
	gcpMaxConcurrentDownloadsFlag = flag.Uint(
		"gcp-max-concurrent-downloads", 5,
		"Maximum quantity of PDFs to download concurrently")
	cupsJobQueueSizeFlag = flag.Uint(
		"cups-job-queue-size", 3,
		"CUPS job queue size")
	cupsPrinterPollIntervalFlag = flag.Duration(
		"cups-printer-poll-interval", time.Minute,
		"Interval, in seconds, between CUPS printer status polls")
	cupsJobFullUsernameFlag = flag.Bool(
		"cups-job-full-username", false,
		"Whether to use the full username (joe@example.com) in CUPS jobs")
	cupsIgnoreRawPrintersFlag = flag.Bool(
		"cups-ignore-raw-printers", true,
		"Whether to ignore raw printers")
	copyPrinterInfoToDisplayNameFlag = flag.Bool(
		"copy-printer-info-to-display-name", true,
		"Whether to copy the CUPS printer's printer-info attribute to the GCP printer's defaultDisplayName")
	monitorSocketFilenameFlag = flag.String(
		"socket-filename", "/var/run/cups-connector/monitor.sock",
		"Filename of unix socket for connector-check to talk to connector")
	gcpBaseURLFlag = flag.String(
		"gcp-base-url", "https://www.google.com/cloudprint/",
		"GCP API base URL")
	gcpOAuthClientIDFlag = flag.String(
		"gcp-oauth-client-id", "539833558011-35iq8btpgas80nrs3o7mv99hm95d4dv6.apps.googleusercontent.com",
		"GCP OAuth client ID")
	gcpOAuthClientSecretFlag = flag.String(
		"gcp-oauth-client-secret", "V9BfPOvdiYuw12hDx5Y5nR0a",
		"GCP OAuth client secret")
	gcpOAuthAuthURLFlag = flag.String(
		"gcp-oauth-auth-url", "https://accounts.google.com/o/oauth2/auth",
		"GCP OAuth auth URL")
	gcpOAuthTokenURLFlag = flag.String(
		"gcp-oauth-token-url", "https://accounts.google.com/o/oauth2/token",
		"GCP OAuth token URL")
	gcpXMPPServerFlag = flag.String(
		"gcp-xmpp-server", "talk.google.com",
		"GCP XMPP server FQDN")
	gcpXMPPPortFlag = flag.Uint(
		"gcp-xmpp-port", 443,
		"GCP XMPP port number")
	gcpXMPPPingTimeoutFlag = flag.String(
		"gcp-xmpp-ping-timeout", "5s",
		"GCP XMPP ping timeout (give up waiting for ping response after this)")
	gcpXMPPPingIntervalDefaultFlag = flag.String(
		"gcp-xmpp-ping-interval-default", "2m",
		"GCP XMPP ping interval default (ping every this often)")
)

// getUserClient steps the user through the process of acquiring an OAuth refresh token.
func getUserClient(retainUserOauthToken bool) (*http.Client, string) {
	config := &oauth2.Config{
		ClientID:     *gcpOAuthClientIDFlag,
		ClientSecret: *gcpOAuthClientSecretFlag,
		Endpoint: oauth2.Endpoint{
			AuthURL:  *gcpOAuthAuthURLFlag,
			TokenURL: *gcpOAuthTokenURLFlag,
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint},
	}

	fmt.Println("Login to Google as the user that will own the printers, then visit this URL:")
	fmt.Println("")
	fmt.Println(config.AuthCodeURL("state", oauth2.AccessTypeOffline))
	fmt.Println("")

	authCode := scanNonEmptyString("After authenticating, enter the provided code here: ")
	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("")
	fmt.Println("Acquired OAuth credentials for user account")

	var userRefreshToken string
	if retainUserOauthToken {
		userRefreshToken = token.RefreshToken
	}

	return config.Client(oauth2.NoContext, token), userRefreshToken
}

// initRobotAccount creates a GCP robot account for this connector.
func initRobotAccount(userClient *http.Client, proxy string) (string, string) {
	params := url.Values{}
	params.Set("oauth_client_id", *gcpOAuthClientIDFlag)
	params.Set("proxy", proxy)

	url := fmt.Sprintf("%s%s?%s", *gcpBaseURLFlag, "createrobot", params.Encode())
	response, err := userClient.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if response.StatusCode != 200 {
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

	fmt.Println("Requested OAuth credentials for robot account")

	return robotInit.XMPPJID, robotInit.AuthCode
}

func verifyRobotAccount(authCode string) string {
	config := &oauth2.Config{
		ClientID:     *gcpOAuthClientIDFlag,
		ClientSecret: *gcpOAuthClientSecretFlag,
		Endpoint: oauth2.Endpoint{
			AuthURL:  *gcpOAuthAuthURLFlag,
			TokenURL: *gcpOAuthTokenURLFlag,
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

func createRobotAccount(userClient *http.Client, proxy string) (string, string) {
	xmppJID, authCode := initRobotAccount(userClient, proxy)
	token := verifyRobotAccount(authCode)

	return xmppJID, token
}

func createConfigFile(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxy string) {
	config := lib.Config{
		xmppJID,
		robotRefreshToken,
		userRefreshToken,
		shareScope,
		proxy,
		*gcpMaxConcurrentDownloadsFlag,
		*cupsJobQueueSizeFlag,
		cupsPrinterPollIntervalFlag.String(),
		defaultPrinterAttributes,
		*cupsJobFullUsernameFlag,
		*cupsIgnoreRawPrintersFlag,
		*copyPrinterInfoToDisplayNameFlag,
		*monitorSocketFilenameFlag,
		*gcpBaseURLFlag,
		*gcpXMPPServerFlag,
		uint16(*gcpXMPPPortFlag),
		*gcpXMPPPingTimeoutFlag,
		*gcpXMPPPingIntervalDefaultFlag,
		*gcpOAuthClientIDFlag,
		*gcpOAuthClientSecretFlag,
		*gcpOAuthAuthURLFlag,
		*gcpOAuthTokenURLFlag,
	}

	if err := config.ToFile(); err != nil {
		log.Fatal(err)
	}
}

func scanNonEmptyString(prompt string) string {
	for {
		var answer string
		fmt.Printf(prompt)
		if length, err := fmt.Scan(&answer); err != nil {
			log.Fatal(err)
		} else if length > 0 {
			return answer
		}
	}
	panic("unreachable")
}

func scanYesOrNo(question string) bool {
	for {
		var answer string
		fmt.Printf(question)
		if _, err := fmt.Scan(&answer); err != nil {
			log.Fatal(err)
		} else if parsed, value := stringToBool(answer); parsed {
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

func main() {
	flag.Parse()

	var parsed bool

	var retainUserOauthToken bool
	if parsed, retainUserOauthToken = stringToBool(*retainUserOauthTokenFlag); !parsed {
		retainUserOauthToken = scanYesOrNo(
			"Would you like to retain the user OAuth token to enable automatic sharing? ")
	}

	var shareScope string
	if retainUserOauthToken {
		if len(*shareScopeFlag) > 0 {
			shareScope = *shareScopeFlag
		} else {
			shareScope = scanNonEmptyString("User or group email address, or domain name, to share with: ")
		}
	} else {
		fmt.Println(
			"The user account OAuth token will be thrown away; printers will not be shared automatically.")
	}

	proxyName := *proxyNameFlag
	if len(proxyName) < 1 {
		proxyName = scanNonEmptyString("Proxy name for this CloudPrint-CUPS server: ")
	}

	userClient, userRefreshToken := getUserClient(retainUserOauthToken)
	fmt.Println("")

	xmppJID, robotRefreshToken := createRobotAccount(userClient, proxyName)

	fmt.Println("Acquired OAuth credentials for robot account")
	fmt.Println("")

	createConfigFile(xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName)
	fmt.Printf("The config file %s is ready to rock.\n", *lib.ConfigFilename)
	fmt.Println("Keep it somewhere safe, as it contains an OAuth token.")

	socketDirectory := filepath.Dir(*monitorSocketFilenameFlag)
	if _, err := os.Stat(socketDirectory); os.IsNotExist(err) {
		fmt.Println("")
		fmt.Printf("When the connector runs, be sure the socket directory %s exists.\n", socketDirectory)
	}
}
