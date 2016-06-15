/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package gcp

import "github.com/google/cloud-print-connector/cdd"

type Job struct {
	GCPPrinterID  string
	GCPJobID      string
	FileURL       string
	OwnerID       string
	Title         string
	SemanticState *cdd.PrintJobState
}
