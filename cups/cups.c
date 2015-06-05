/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include "cups.h"

const char
	*JOB_STATE                  = "job-state",
	*JOB_MEDIA_SHEETS_COMPLETED = "job-media-sheets-completed",
	*POST_RESOURCE              = "/",
	*REQUESTED_ATTRIBUTES       = "requested-attributes",
	*JOB_URI_ATTRIBUTE          = "job-uri",
	*IPP                        = "ipp";

// Allocates a new char**, initializes the values to NULL.
char **newArrayOfStrings(int size) {
	return calloc(size, sizeof(char *));
}

// Sets one value in a char**.
void setStringArrayValue(char **stringArray, int index, char *value) {
	stringArray[index] = value;
}

// Frees a char** and associated non-NULL char*.
void freeStringArrayAndStrings(char **stringArray, int size) {
	int i;
	for (i = 0; i < size; i++) {
		if (stringArray[i] != NULL)
			free(stringArray[i]);
	}
	free(stringArray);
}

// Wraps ippGetResolution() until bug fixed:
// https://code.google.com/p/go/issues/detail?id=7270
int ippGetResolutionWrapper(ipp_attribute_t *attr, int element, int *yres, int *units) {
	return ippGetResolution(attr, element, yres, (ipp_res_t *)units);
}
