/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/lib"
	"github.com/satori/go.uuid"
	"github.com/urfave/cli"

	"golang.org/x/oauth2"
)

const (
	gcpOAuthDeviceCodeURL   = "https://accounts.google.com/o/oauth2/device/code"
	gcpOAuthTokenPollURL    = "https://www.googleapis.com/oauth2/v3/token"
	gcpOAuthGrantTypeDevice = "http://oauth.net/grant_type/device/1.0"
)

var commonInitFlags = []cli.Flag{
	cli.DurationFlag{
		Name:  "gcp-api-timeout",
		Usage: "GCP API timeout, for debugging",
		Value: 30 * time.Second,
	},
	cli.StringFlag{
		Name:  "gcp-oauth-client-id",
		Usage: "Identifies the CUPS Connector to the Google Cloud Print cloud service",
		Value: lib.DefaultConfig.GCPOAuthClientID,
	},
	cli.StringFlag{
		Name:  "gcp-oauth-client-secret",
		Usage: "Goes along with the Client ID. Not actually secret",
		Value: lib.DefaultConfig.GCPOAuthClientSecret,
	},
	cli.StringFlag{
		Name:  "gcp-user-refresh-token",
		Usage: "GCP user refresh token, useful when managing many connectors",
	},
	cli.StringFlag{
		Name:  "share-scope",
		Usage: "Scope (user or group email address) to automatically share printers with",
	},
	cli.StringFlag{
		Name:  "proxy-name",
		Usage: "Name for this connector instance. Should be unique per Google user account",
	},
	cli.IntFlag{
		Name:  "xmpp-port",
		Usage: "XMPP port number",
		Value: int(lib.DefaultConfig.XMPPPort),
	},
	cli.StringFlag{
		Name:  "xmpp-ping-timeout",
		Usage: "XMPP ping timeout (give up waiting for ping response after this)",
		Value: lib.DefaultConfig.XMPPPingTimeout,
	},
	cli.StringFlag{
		Name:  "xmpp-ping-interval",
		Usage: "XMPP ping interval (ping every this often)",
		Value: lib.DefaultConfig.XMPPPingInterval,
	},
	cli.IntFlag{
		Name:  "gcp-max-concurrent-downloads",
		Usage: "Maximum quantity of PDFs to download concurrently from GCP cloud service",
		Value: int(lib.DefaultConfig.GCPMaxConcurrentDownloads),
	},
	cli.IntFlag{
		Name:  "native-job-queue-size",
		Usage: "Native job queue size",
		Value: int(lib.DefaultConfig.NativeJobQueueSize),
	},
	cli.StringFlag{
		Name:  "native-printer-poll-interval",
		Usage: "Interval, in seconds, between native printer state polls",
		Value: lib.DefaultConfig.NativePrinterPollInterval,
	},
	cli.BoolFlag{
		Name:  "prefix-job-id-to-job-title",
		Usage: "Whether to add the job ID to the beginning of the job title",
	},
	cli.StringFlag{
		Name:  "display-name-prefix",
		Usage: "Prefix to add to GCP printer's display name",
		Value: lib.DefaultConfig.DisplayNamePrefix,
	},
	cli.BoolFlag{
		Name:  "local-printing-enable",
		Usage: "Enable local discovery and printing (aka GCP 2.0 or Privet)",
	},
	cli.BoolFlag{
		Name:  "cloud-printing-enable",
		Usage: "Enable cloud discovery and printing",
	},
	cli.StringFlag{
		Name:  "log-level",
		Usage: "Minimum event severity to log: FATAL, ERROR, WARNING, INFO, DEBUG",
		Value: lib.DefaultConfig.LogLevel,
	},
	cli.IntFlag{
		Name:  "local-port-low",
		Usage: "Local HTTP API server port range, low",
		Value: int(lib.DefaultConfig.LocalPortLow),
	},
	cli.IntFlag{
		Name:  "local-port-high",
		Usage: "Local HTTP API server port range, high",
		Value: int(lib.DefaultConfig.LocalPortHigh),
	},
}

func postWithRetry(url string, data url.Values) (*http.Response, error) {
	backoff := lib.Backoff{}
	for {
		response, err := http.PostForm(url, data)
		if err == nil {
			return response, err
		}
		fmt.Printf("POST to %s failed with error: %s\n", url, err)

		p, retryAgain := backoff.Pause()
		if !retryAgain {
			return response, err
		}
		fmt.Printf("retrying POST to %s in %s\n", url, p)
		time.Sleep(p)
	}
}

// getUserClientFromUser follows the token acquisition steps outlined here:
// https://developers.google.com/identity/protocols/OAuth2ForDevices
func getUserClientFromUser(context *cli.Context) (*http.Client, string, error) {
	form := url.Values{
		"client_id": {context.String("gcp-oauth-client-id")},
		"scope":     {gcp.ScopeCloudPrint},
	}
	response, err := postWithRetry(gcpOAuthDeviceCodeURL, form)
	if err != nil {
		return nil, "", err
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

	return pollOAuthConfirmation(context, r.DeviceCode, r.Interval)
}

func pollOAuthConfirmation(context *cli.Context, deviceCode string, interval int) (*http.Client, string, error) {
	config := oauth2.Config{
		ClientID:     context.String("gcp-oauth-client-id"),
		ClientSecret: context.String("gcp-oauth-client-secret"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  lib.DefaultConfig.GCPOAuthAuthURL,
			TokenURL: lib.DefaultConfig.GCPOAuthTokenURL,
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint},
	}

	for {
		time.Sleep(time.Duration(interval) * time.Second)

		form := url.Values{
			"client_id":     {context.String("gcp-oauth-client-id")},
			"client_secret": {context.String("gcp-oauth-client-secret")},
			"code":          {deviceCode},
			"grant_type":    {gcpOAuthGrantTypeDevice},
		}
		response, err := postWithRetry(gcpOAuthTokenPollURL, form)
		if err != nil {
			return nil, "", err
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
			client.Timeout = context.Duration("gcp-api-timeout")
			return client, r.RefreshToken, nil
		case "authorization_pending":
		case "slow_down":
			interval *= 2
		default:
			return nil, "", err
		}
	}

	panic("unreachable")
}

// getUserClientFromToken creates a user client with just a refresh token.
func getUserClientFromToken(context *cli.Context) *http.Client {
	config := &oauth2.Config{
		ClientID:     context.String("gcp-oauth-client-id"),
		ClientSecret: context.String("gcp-oauth-client-secret"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  lib.DefaultConfig.GCPOAuthAuthURL,
			TokenURL: lib.DefaultConfig.GCPOAuthTokenURL,
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint},
	}

	token := &oauth2.Token{RefreshToken: context.String("gcp-user-refresh-token")}
	client := config.Client(oauth2.NoContext, token)
	client.Timeout = context.Duration("gcp-api-timeout")

	return client
}

// initRobotAccount creates a GCP robot account for this connector.
func initRobotAccount(context *cli.Context, userClient *http.Client) (string, string, error) {
	params := url.Values{}
	params.Set("oauth_client_id", context.String("gcp-oauth-client-id"))

	url := fmt.Sprintf("%s%s?%s", lib.DefaultConfig.GCPBaseURL, "createrobot", params.Encode())
	response, err := userClient.Get(url)
	if err != nil {
		return "", "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Failed to initialize robot account: %s", response.Status)
	}

	var robotInit struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		XMPPJID  string `json:"xmpp_jid"`
		AuthCode string `json:"authorization_code"`
	}

	if err = json.NewDecoder(response.Body).Decode(&robotInit); err != nil {
		return "", "", err
	}
	if !robotInit.Success {
		return "", "", fmt.Errorf("Failed to initialize robot account: %s", robotInit.Message)
	}

	return robotInit.XMPPJID, robotInit.AuthCode, nil
}

func verifyRobotAccount(context *cli.Context, authCode string) (string, error) {
	config := &oauth2.Config{
		ClientID:     context.String("gcp-oauth-client-id"),
		ClientSecret: context.String("gcp-oauth-client-secret"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  lib.DefaultConfig.GCPOAuthAuthURL,
			TokenURL: lib.DefaultConfig.GCPOAuthTokenURL,
		},
		RedirectURL: gcp.RedirectURL,
		Scopes:      []string{gcp.ScopeCloudPrint, gcp.ScopeGoogleTalk},
	}

	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		return "", err
	}

	return token.RefreshToken, nil
}

func createRobotAccount(context *cli.Context, userClient *http.Client) (string, string, error) {
	xmppJID, authCode, err := initRobotAccount(context, userClient)
	if err != nil {
		return "", "", err
	}
	token, err := verifyRobotAccount(context, authCode)
	if err != nil {
		return "", "", err
	}

	return xmppJID, token, nil
}

func scanString(prompt string) (string, error) {
	fmt.Println(prompt)
	reader := bufio.NewReader(os.Stdin)
	if answer, err := reader.ReadString('\n'); err != nil {
		return "", err
	} else {
		answer = answer[:len(answer)-1] // remove newline
		fmt.Println("")
		return answer, nil
	}
}

func scanYesOrNo(question string) (bool, error) {
	for {
		var answer string
		fmt.Println(question)
		if _, err := fmt.Scan(&answer); err != nil {
			return false, err
		} else if parsed, value := stringToBool(answer); parsed {
			fmt.Println("")
			return value, nil
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

func initConfigFile(context *cli.Context) error {
	var err error

	var localEnable bool
	if runtime.GOOS == "windows" {
		// Remove this if block when Privet support is added to Windows.
		localEnable = false
	} else if context.IsSet("local-printing-enable") {
		localEnable = context.Bool("local-printing-enable")
	} else {
		fmt.Println("\"Local printing\" means that clients print directly to the connector via")
		fmt.Println("local subnet, and that an Internet connection is neither necessary nor used.")
		localEnable, err = scanYesOrNo("Enable local printing?")
		if err != nil {
			return err
		}
	}

	var cloudEnable bool
	if runtime.GOOS == "windows" {
		// Remove this if block when Privet support is added to Windows.
		cloudEnable = true
	} else if localEnable == false {
		cloudEnable = true
	} else if context.IsSet("cloud-printing-enable") {
		cloudEnable = context.Bool("cloud-printing-enable")
	} else {
		fmt.Println("\"Cloud printing\" means that clients can print from anywhere on the Internet,")
		fmt.Println("and that printers must be explicitly shared with users.")
		cloudEnable, err = scanYesOrNo("Enable cloud printing?")
		if err != nil {
			return err
		}
	}

	var config *lib.Config

	var xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName string
	if cloudEnable {
		if context.IsSet("proxy-name") {
			proxyName = context.String("proxy-name")
		} else {
			proxyName = uuid.NewV4().String()
		}

		var userClient *http.Client
		var urt string
		if context.IsSet("gcp-user-refresh-token") {
			userClient = getUserClientFromToken(context)
		} else {
			userClient, urt, err = getUserClientFromUser(context)
			if err != nil {
				return err
			}
		}

		xmppJID, robotRefreshToken, err = createRobotAccount(context, userClient)
		if err != nil {
			return err
		}

		fmt.Println("Acquired OAuth credentials for robot account")
		fmt.Println("")

		if context.IsSet("share-scope") {
			shareScope = context.String("share-scope")
		} else {
			shareScope, err = scanString("Enter the email address of a user or group with whom all printers will automatically be shared or press enter to disable automatic sharing:")
			if err != nil {
				return err
			}
		}

		if shareScope != "" {
			if context.IsSet("gcp-user-refresh-token") {
				userRefreshToken = context.String("gcp-user-refresh-token")
			} else {
				userRefreshToken = urt
			}
		}

		config = createCloudConfig(context, xmppJID, robotRefreshToken, userRefreshToken, shareScope, proxyName, localEnable)

	} else {
		config = createLocalConfig(context)
	}

	configFilename, err := config.Sparse(context).ToFile(context)
	if err != nil {
		return err
	}
	fmt.Printf("The config file %s is ready to rock.\n", configFilename)
	if cloudEnable {
		fmt.Println("Keep it somewhere safe, as it contains an OAuth refresh token.")
	}

	socketDirectory := filepath.Dir(context.String("monitor-socket-filename"))
	if _, err := os.Stat(socketDirectory); os.IsNotExist(err) {
		fmt.Println("")
		fmt.Printf("When the connector runs, be sure the socket directory %s exists.\n", socketDirectory)
	} else if err != nil {
		return err
	}
	return nil
}
