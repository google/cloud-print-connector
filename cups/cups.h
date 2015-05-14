/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include <cups/cups.h>
#include <stdlib.h> // free, calloc

extern const char
	*JOB_STATE,
	*JOB_MEDIA_SHEETS_COMPLETED,
	*POST_RESOURCE,
	*REQUESTED_ATTRIBUTES,
	*JOB_URI_ATTRIBUTE,
	*IPP;

char **newArrayOfStrings(int size);

void setStringArrayValue(char **stringArray, int index, char *value);

void freeStringArrayAndStrings(char **stringArray, int size);

int ippGetResolutionWrapper(ipp_attribute_t *attr, int element, int *yres, int *units);
