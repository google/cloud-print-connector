/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include <stdarg.h> // vfprintf
#include <stdio.h>  // stderr, asprintf
#include <stdlib.h> // free
#include <string.h> // strcat, strlen

void logError(const char *format, ...);
