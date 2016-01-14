/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package common

import "os"

func Env(name string) (string, bool) {
	deviceURI := os.Getenv(name)
	if deviceURI == "" {
		Crit("%s env variable is not set", name)
		return "", false
	}
	return deviceURI, true
}
