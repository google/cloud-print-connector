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
	"time"

	"github.com/google/cloud-print-connector/lib"
	"github.com/google/cloud-print-connector/log"

	"golang.org/x/oauth2"
)

/*
glibc < 2.20 and OSX 10.10 have problems when C.getaddrinfo is called many
times concurrently. When the connector shares more than about 230 printers, and
GCP is called once per printer in concurrent goroutines, http.Client.Do starts
to fail with a lookup error.

This solution, a semaphore, limits the quantity of concurrent HTTP requests,
which also limits the quantity of concurrent calls to net.LookupHost (which
calls C.getaddrinfo()).

I would rather wait for the Go compiler to solve this problem than make this a
configurable option, hence this long-winded comment.

https://github.com/golang/go/issues/3575
https://github.com/golang/go/issues/6336
*/
var lock *lib.Semaphore = lib.NewSemaphore(100)

// newClient creates an instance of http.Client, wrapped with OAuth credentials.
func newClient(oauthClientID, oauthClientSecret, oauthAuthURL, oauthTokenURL, refreshToken string, scopes ...string) (*http.Client, error) {
	config := oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthAuthURL,
			TokenURL: oauthTokenURL,
		},
		RedirectURL: RedirectURL,
		Scopes:      scopes,
	}

	token := oauth2.Token{RefreshToken: refreshToken}
	client := config.Client(oauth2.NoContext, &token)

	return client, nil
}

// getWithRetry calls get() and retries on HTTP temp failure
// (response code 500-599).
func getWithRetry(hc *http.Client, url string) (*http.Response, error) {
	backoff := lib.Backoff{}
	for {
		response, err := get(hc, url)
		if response != nil && response.StatusCode == http.StatusOK {
			return response, err
		} else if response != nil && response.StatusCode >= 500 && response.StatusCode <= 599 {
			p, retryAgain := backoff.Pause()
			if !retryAgain {
				log.Debugf("HTTP error %s, retry timeout hit", err)
				return response, err
			}
			log.Debugf("HTTP error %s, retrying after %s", err, p)
			time.Sleep(p)
		} else {
			log.Debugf("Permanent HTTP error %s, will not retry", err)
			return response, err
		}
	}
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

	lock.Acquire()
	response, err := hc.Do(request)
	lock.Release()
	if err != nil {
		return response, fmt.Errorf("GET failure: %s", err)
	}
	if response.StatusCode != http.StatusOK {
		return response, fmt.Errorf("GET HTTP-level failure: %s %s", url, response.Status)
	}

	return response, nil
}

// postWithRetry calls post() and retries on HTTP temp failure
// (response code 500-599).
func postWithRetry(hc *http.Client, url string, form url.Values) ([]byte, uint, int, error) {
	backoff := lib.Backoff{}
	for {
		responseBody, gcpErrorCode, httpStatusCode, err := post(hc, url, form)
		if responseBody != nil && httpStatusCode == http.StatusOK {
			return responseBody, gcpErrorCode, httpStatusCode, err
		} else if responseBody != nil && httpStatusCode >= 500 && httpStatusCode <= 599 {
			p, retryAgain := backoff.Pause()
			if !retryAgain {
				log.Debugf("HTTP error %s, retry timeout hit", err)
				return responseBody, gcpErrorCode, httpStatusCode, err
			}
			log.Debugf("HTTP error %s, retrying after %s", err, p)
			time.Sleep(p)
		} else {
			log.Debugf("Permanent HTTP error %s, will not retry", err)
			return responseBody, gcpErrorCode, httpStatusCode, err
		}
	}
}

// post POSTs to a URL. Returns the body of the response.
//
// Returns the response body, GCP error code, HTTP status, and error.
// None of the returned fields is guaranteed to be non-zero.
func post(hc *http.Client, url string, form url.Values) ([]byte, uint, int, error) {
	requestBody := strings.NewReader(form.Encode())
	request, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, 0, 0, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CloudPrint-Proxy", lib.ShortName)

	lock.Acquire()
	response, err := hc.Do(request)
	lock.Release()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("POST failure: %s", err)
	}

	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, 0, response.StatusCode, err
	}

	if response.StatusCode != http.StatusOK {
		return responseBody, 0, response.StatusCode, fmt.Errorf("/%s POST HTTP-level failure: %s", url, response.Status)
	}

	var responseStatus struct {
		Success   bool
		Message   string
		ErrorCode uint
	}
	if err = json.Unmarshal(responseBody, &responseStatus); err != nil {
		return responseBody, 0, response.StatusCode, err
	}
	if !responseStatus.Success {
		return responseBody, responseStatus.ErrorCode, response.StatusCode, fmt.Errorf(
			"%s call failed: %s", url, responseStatus.Message)
	}

	return responseBody, responseStatus.ErrorCode, response.StatusCode, nil
}
