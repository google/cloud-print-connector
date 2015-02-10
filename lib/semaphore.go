/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

type Semaphore struct {
	ch chan bool
}

func NewSemaphore(size uint) *Semaphore {
	s := make(chan bool, size)
	return &Semaphore{s}
}

func (s *Semaphore) Acquire() {
	s.ch <- true
}

func (s *Semaphore) Release() {
	<-s.ch
}

func (s *Semaphore) Count() uint {
	return uint(len(s.ch))
}

func (s *Semaphore) Size() uint {
	return uint(cap(s.ch))
}
