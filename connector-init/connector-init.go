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
package main

import (
	"cups-connector/lib"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/oauth2"
)

func getUserClient() *http.Client {
	options := oauth2.Options{
		ClientID:     lib.ClientID,
		ClientSecret: lib.ClientSecret,
		RedirectURL:  lib.RedirectURL,
		Scopes:       []string{lib.ScopeCloudPrint},
	}
	oauthConfig, err := oauth2.NewConfig(&options, lib.AuthURL, lib.TokenURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Login to Google as the user that will share printers, then visit this URL:")
	fmt.Println("")
	fmt.Println(oauthConfig.AuthCodeURL("", "offline", "auto"))
	fmt.Println("")
	fmt.Println("After authenticating, enter the provided code here:")

	var authCode string
	if _, err = fmt.Scan(&authCode); err != nil {
		log.Fatal(err)
	}

	transport, err := oauthConfig.NewTransportWithCode(authCode)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("")
	fmt.Println("Acquired OAuth credentials for user account")

	return &http.Client{Transport: transport}
}

func initRobotAccount(userClient *http.Client, proxy string) (string, string) {
	params := url.Values{}
	params.Set("oauth_client_id", lib.ClientID)
	params.Set("proxy", proxy)
	response, err := userClient.Get(lib.CreateRobotURL + "?" + params.Encode())
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

func verifyRobotAccount(authCode string) *oauth2.Token {
	options := oauth2.Options{
		ClientID:     lib.ClientID,
		ClientSecret: lib.ClientSecret,
		RedirectURL:  lib.RedirectURL,
		Scopes:       []string{lib.ScopeCloudPrint, lib.ScopeGoogleTalk},
	}
	oauthConfig, err := oauth2.NewConfig(&options, lib.AuthURL, lib.TokenURL)
	if err != nil {
		log.Fatal(err)
	}

	token, err := oauthConfig.Exchange(authCode)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Acquired OAuth credentials for robot account")

	return token
}

func createRobotAccount(userClient *http.Client, proxy string) (string, *oauth2.Token) {
	xmppJID, authCode := initRobotAccount(userClient, proxy)
	token := verifyRobotAccount(authCode)

	return xmppJID, token
}

func createConfigFile(xmppJID string, token *oauth2.Token, proxy string, infoToDisplayName bool) {
	config := lib.Config{
		RefreshToken:                 token.RefreshToken,
		XMPPJID:                      xmppJID,
		Proxy:                        proxy,
		MaxConcurrentFetch:           lib.DefaultMaxConcurrentFetch,
		CUPSQueueSize:                lib.DefaultCUPSQueueSize,
		CUPSPollIntervalPrinter:      lib.DefaultCUPSPollIntervalPrinter,
		CUPSPollIntervalJob:          lib.DefaultCUPSPollIntervalJob,
		CUPSPrinterAttributes:        lib.DefaultPrinterAttributes,
		CopyPrinterInfoToDisplayName: infoToDisplayName,
	}

	if err := config.ToFile(); err != nil {
		log.Fatal(err)
	}
}

func getProxy() string {
	for {
		var proxy string
		fmt.Printf("Proxy name for this CloudPrint-CUPS server: ")
		if _, err := fmt.Scan(&proxy); err != nil {
			log.Fatal(err)
		} else if len(proxy) > 0 {
			return proxy
		}
	}

	panic("unreachable")
}

func getInfoToDisplayName() bool {
	for {
		var infoToDisplayName string
		fmt.Printf("Copy CUPS printer-info attribute to GCP defaultDisplayName? ")
		if _, err := fmt.Scan(&infoToDisplayName); err != nil {
			log.Fatal(err)
		} else if len(infoToDisplayName) > 0 {
			switch strings.ToLower(infoToDisplayName[0:1]) {
			case "y", "t", "1":
				return true
			case "n", "f", "0":
				return false
			}
		}
	}

	panic("unreachable")
}

func main() {
	flag.Parse()

	userClient := getUserClient()
	proxy := getProxy()
	infoToDisplayName := getInfoToDisplayName()
	xmppJID, token := createRobotAccount(userClient, proxy)
	createConfigFile(xmppJID, token, proxy, infoToDisplayName)

	fmt.Println("")
	fmt.Printf("The config file %s is ready to rock.\n", *lib.ConfigFilename)
	fmt.Println("Keep it somewhere safe, as it contains an OAuth token.")
}
