/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include <CFNetwork/CFNetServices.h>
#include <CoreFoundation/CFString.h>
#include <CoreFoundation/CFStream.h>

#include <stdio.h>  // asprintf
#include <stdlib.h> // free

CFNetServiceRef startBonjour(char *name, char *type, unsigned short int port, char *ty, char *url, char *id, char *cs, char **err);
void updateBonjour(CFNetServiceRef service, char *ty, char *url, char *id, char *cs);
void stopBonjour(CFNetServiceRef service);
