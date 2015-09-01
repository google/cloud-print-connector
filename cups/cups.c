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

// getIPPRequestStatusCode gets the status_code field.
// This field is not visible to cgo (don't know why).
ipp_status_t getIPPRequestStatusCode(ipp_t *ipp) {
	return ipp->request.status.status_code;
}

// getAttributeDateValue gets the ith date value from attr.
const ipp_uchar_t *getAttributeDateValue(ipp_attribute_t *attr, int i) {
	return attr->values[i].date;
}

// getAttributeIntegerValue gets the ith integer value from attr.
int getAttributeIntegerValue(ipp_attribute_t *attr, int i) {
	return attr->values[i].integer;
}

// getAttributeStringValue gets the ith string value from attr.
const char *getAttributeStringValue(ipp_attribute_t *attr, int i) {
	return attr->values[i].string.text;
}

// getAttributeValueRange gets the ith range value from attr.
void getAttributeValueRange(ipp_attribute_t *attr, int i, int *lower,
		int *upper) {
	*lower = attr->values[i].range.lower;
	*upper = attr->values[i].range.upper;
}

// getAttributeValueResolution gets the ith resolution value from attr.
// The values returned are always "per inch" not "per centimeter".
void getAttributeValueResolution(ipp_attribute_t *attr, int i, int *xres,
		int *yres) {
	if (IPP_RES_PER_CM == attr->values[i].resolution.units) {
		*xres = attr->values[i].resolution.xres * 2.54;
		*yres = attr->values[i].resolution.yres * 2.54;
	} else {
		*xres = attr->values[i].resolution.xres;
		*yres = attr->values[i].resolution.yres;
	}
}

#ifndef _CUPS_API_1_7
// Skip attribute validation with older clients.
int ippValidateAttributes(ipp_t *ipp) {
	return 1;
}

// Ignore some fields with older clients.
// The connector doesn't use addrlist anyways.
// Older clients use msec = 30000.
http_t *httpConnect2(const char *host, int port, http_addrlist_t *addrlist, int family,
                     http_encryption_t encryption, int blocking, int msec, int *cancel) {
	return httpConnectEncrypt(host, port, encryption);
}
#endif
