/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cdd

type LocalSettingsSettings struct {
	LocalDiscovery            *bool `json:"local_discovery,omitempty"`
	AccessTokenEnabled        *bool `json:"access_token_enabled,omitempty"`
	LocalPrintingEnabled      *bool `json:"printer/local_printing_enabled,omitempty"`
	ConversionPrintingEnabled *bool `json:"printer/conversion_printing_enabled,omitempty"`
	XMPPTimeoutValue          int32 `json:"xmpp_timeout_value"`
}

type LocalSettings struct {
	Current *LocalSettingsSettings `json:"current"`
	Pending *LocalSettingsSettings `json:"pending,omitempty"`
}
