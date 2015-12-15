/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import "github.com/google/cups-connector/cdd"

type Job struct {
	NativePrinterName string
	Filename          string
	Title             string
	User              string
	JobID             string
	Ticket            *cdd.CloudJobTicket
	UpdateJob         func(string, cdd.PrintJobStateDiff) error
}
