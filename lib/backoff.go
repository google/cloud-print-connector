/*
Copyright 2016 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import (
	"math/rand"
	"time"
)

const (
	initialRetryInterval = 500 * time.Millisecond
	maxInterval          = 1 * time.Minute
	maxElapsedTime       = 15 * time.Minute
	multiplier           = 1.5
	randomizationFactor  = 0.5
)

// Backoff provides a mechanism for determining a good amount of time before
// retrying an operation.
type Backoff struct {
	interval    time.Duration
	elapsedTime time.Duration
}

// Pause returns the amount of time to wait before retrying an operation and true if
// it is ok to try again or false if the operation should be abandoned.
func (b *Backoff) Pause() (time.Duration, bool) {
	if b.interval == 0 {
		// first time
		b.interval = initialRetryInterval
		b.elapsedTime = 0
	}

	// interval from [1 - randomizationFactor, 1 + randomizationFactor)
	randomizedInterval := time.Duration((rand.Float64()*(2*randomizationFactor) + (1 - randomizationFactor)) * float64(b.interval))
	b.elapsedTime += randomizedInterval

	if b.elapsedTime > maxElapsedTime {
		return 0, false
	}

	// Increase interval up to the interval cap
	b.interval = time.Duration(float64(b.interval) * multiplier)
	if b.interval > maxInterval {
		b.interval = maxInterval
	}

	return randomizedInterval, true
}
