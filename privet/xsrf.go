/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"time"
)

const (
	deviceSecretLength = 24             // 24 bytes == 192 bits
	tokenTimeout       = 24 * time.Hour // Specified in GCP Privet doc.
)

// xsrfSecret generates and validates XSRF tokens.
type xsrfSecret []byte

func newXSRFSecret() xsrfSecret {
	// Generate a random device secret.
	deviceSecret := make([]byte, deviceSecretLength)
	rand.Read(deviceSecret)
	return deviceSecret
}

func (x xsrfSecret) newToken() string {
	t := time.Now()
	return x.newTokenProvideTime(t)
}

func (x xsrfSecret) newTokenProvideTime(t time.Time) string {
	tb := int64ToBytes(t.Unix())
	sum := sha1.Sum(append(x, tb...))
	token := append(sum[:], tb...)
	return base64.StdEncoding.EncodeToString(token)
}

func (x xsrfSecret) isTokenValid(token string) bool {
	return x.isTokenValidProvideTime(token, time.Now())
}

func (x xsrfSecret) isTokenValidProvideTime(token string, now time.Time) bool {
	tokenBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return false
	}

	tb := tokenBytes[sha1.Size:]
	t := time.Unix(bytesToInt64(tb), 0)
	if now.Sub(t) > tokenTimeout {
		return false
	}
	if t.Sub(now) > 0 {
		return false
	}

	sum := sha1.Sum(append(x, tb...))
	if 0 != bytes.Compare(sum[:], tokenBytes[:sha1.Size]) {
		return false
	}

	return true
}

func int64ToBytes(v int64) []byte {
	b := make([]byte, 8)
	for i := range b {
		b[i] = byte(v >> uint(8*i))
	}
	return b
}

func bytesToInt64(b []byte) int64 {
	if len(b) != 8 {
		return 0
	}
	var v int64
	for i := range b {
		v |= int64(b[i]) << uint(8*i)
	}
	return v
}
