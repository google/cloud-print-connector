/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(size uint) *Semaphore {
	s := make(chan struct{}, size)
	return &Semaphore{s}
}

// Acquire increments the semaphore, blocking if necessary.
func (s *Semaphore) Acquire() {
	s.ch <- struct{}{}
}

// TryAcquire increments the semaphore without blocking.
// Returns false if the semaphore was not acquired.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release decrements the semaphore. If this operation causes
// the semaphore value to be negative, then panics.
func (s *Semaphore) Release() {
	select {
	case _ = <-s.ch:
		return
	default:
		panic("Semaphore was released without being acquired")
	}
}

// Count returns the current value of the semaphore.
func (s *Semaphore) Count() uint {
	return uint(len(s.ch))
}

// Size returns the maximum semaphore value.
func (s *Semaphore) Size() uint {
	return uint(cap(s.ch))
}
