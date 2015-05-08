/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package gcp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/cups-connector/lib"

	"golang.org/x/oauth2"
)

// newClient creates an instance of http.Client, wrapped with OAuth
// credentials.
func newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, refreshToken string, scopes ...string) (*http.Client, error) {
	config := &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthAuthURL,
			TokenURL: oauthTokenURL,
		},
		RedirectURL: RedirectURL,
		Scopes:      scopes,
	}

	token := &oauth2.Token{RefreshToken: refreshToken}
	client := config.Client(oauth2.NoContext, token)

	return client, nil
}

// getWithRetry calls get() and retries once on HTTP failure
// (response code != 200).
func getWithRetry(hc *http.Client, url string) (*http.Response, error) {
	response, err := get(hc, url)
	if response != nil && response.StatusCode == 200 {
		return response, err
	}

	return get(hc, url)
}

// get GETs a URL. Returns the response object (not body), in case the body
// is very large.
//
// The caller must close the returned Response.Body object if err == nil.
func get(hc *http.Client, url string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-CloudPrint-Proxy", lib.ShortName)

	response, err := hc.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET failure: %s", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET HTTP-level failure: %s %s", url, response.Status)
	}

	return response, nil
}

// postWithRetry calls post() and retries once on HTTP failure
// (response code != 200).
func postWithRetry(hc *http.Client, url string, form url.Values) ([]byte, uint, int, error) {
	responseBody, gcpErrorCode, httpStatusCode, err := post(hc, url, form)
	if responseBody != nil && httpStatusCode == 200 {
		return responseBody, gcpErrorCode, httpStatusCode, err
	}

	return post(hc, url, form)
}

// post POSTs to a URL. Returns the body of the response.
//
// Returns the response body, GCP error code, HTTP status, and error.
// On success, only the response body is guaranteed to be non-zero.
func post(hc *http.Client, url string, form url.Values) ([]byte, uint, int, error) {
	requestBody := strings.NewReader(form.Encode())
	request, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, 0, 0, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CloudPrint-Proxy", lib.ShortName)

	response, err := hc.Do(request)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("POST failure: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, 0, response.StatusCode, fmt.Errorf("/%s POST HTTP-level failure: %s", url, response.Status)
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, 0, response.StatusCode, err
	}

	var responseStatus struct {
		Success   bool
		Message   string
		ErrorCode uint
	}
	if err = json.Unmarshal(responseBody, &responseStatus); err != nil {
		return nil, 0, response.StatusCode, err
	}
	if !responseStatus.Success {
		return nil, responseStatus.ErrorCode, response.StatusCode, fmt.Errorf(
			"%s call failed: %s", url, responseStatus.Message)
	}

	return responseBody, 0, response.StatusCode, nil
}
