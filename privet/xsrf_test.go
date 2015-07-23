/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"bytes"
	"testing"
	"time"
)

var (
	deviceSecret xsrfSecret = []byte("secretsecretsecretsecret")
	testTime                = time.Unix(1234567890123456789, 0)
	testToken               = "/8IVybcmeL9NPBBJgaZEa0r+GIkVgel99BAiEQ=="
)

func TestNewToken(t *testing.T) {
	x := deviceSecret
	token := x.newTokenProvideTime(testTime)
	if token != testToken {
		t.Errorf("new token was %s should be %s", token, testToken)
	}
}

func TestIsTokenValid(t *testing.T) {
	x := deviceSecret
	if !x.isTokenValidProvideTime(testToken, testTime) {
		t.Errorf("valid token reported as invalid (+0ns)")
	}

	altTime := testTime.Add(time.Minute)
	if !x.isTokenValidProvideTime(testToken, altTime) {
		t.Errorf("valid token reported as invalid (+1m)")
	}

	altTime = testTime.Add(time.Hour)
	if !x.isTokenValidProvideTime(testToken, altTime) {
		t.Errorf("valid token reported as invalid (+1h)")
	}

	altTime = testTime.Add(23 * time.Hour)
	if !x.isTokenValidProvideTime(testToken, altTime) {
		t.Errorf("valid token reported as invalid (+23h)")
	}

	altTime = testTime.Add(25 * time.Hour)
	if x.isTokenValidProvideTime(testToken, altTime) {
		t.Errorf("invalid token reported as valid (+25h)")
	}

	altTime = testTime.Add(-time.Minute)
	if x.isTokenValidProvideTime(testToken, altTime) {
		t.Errorf("invalid token reported as valid (-1m)")
	}
}

func TestIsBadFormatTokenValid(t *testing.T) {
	x := deviceSecret
	if x.isTokenValidProvideTime("", testTime) {
		t.Errorf("empty token reported as valid")
	}
}

func TestInt64ToBytes(t *testing.T) {
	var v int64 = 0
	b := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	if got := int64ToBytes(v); bytes.Compare(b, got) != 0 {
		t.Errorf("expected %v got %v", b, got)
	}

	v = 1234567890123456789
	b = []byte{21, 129, 233, 125, 244, 16, 34, 17}
	if got := int64ToBytes(v); bytes.Compare(b, got) != 0 {
		t.Errorf("expected %v got %v", b, got)
	}
}

func TestBytesToInt64(t *testing.T) {
	var v int64 = 0
	b := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	if got := bytesToInt64(b); v != got {
		t.Errorf("expected %d got %d", v, got)
	}

	v = 1234567890123456789
	b = []byte{21, 129, 233, 125, 244, 16, 34, 17}
	if got := bytesToInt64(b); v != got {
		t.Errorf("expected %d got %d", v, got)
	}

	v = 0
	b = []byte{1, 2, 3, 4}
	if got := bytesToInt64(b); v != got {
		t.Errorf("expected %d got %d", v, got)
	}
}
