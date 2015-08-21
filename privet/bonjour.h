// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build darwin

#include <CFNetwork/CFNetServices.h>
#include <CoreFoundation/CFString.h>
#include <CoreFoundation/CFStream.h>

#include <stdio.h>  // asprintf
#include <stdlib.h> // free

CFNetServiceRef startBonjour(const char *name, const char *type,
		unsigned short int port, const char *ty, const char *url, const char *id,
		const char *cs, char **err);
void updateBonjour(CFNetServiceRef service, const char *ty, const char *url,
		const char *id, const char *cs);
void stopBonjour(CFNetServiceRef service);
