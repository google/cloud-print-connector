/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Since CUPS 1.6, the ipp struct properties have been private, with accessor
// functions added (STR #3928). This line makes the properties not private, so
// that the connector can be compiled against pre and post 1.6 libraries.
#define _IPP_PRIVATE_STRUCTURES 1

#include <cups/cups.h>
#include <cups/ppd.h>
#include <stddef.h>      // size_t
#include <stdlib.h>      // free, calloc, malloc
#include <sys/socket.h>  // AF_UNSPEC
#include <sys/utsname.h> // uname
#include <time.h>        // time_t

/* https://bugs.launchpad.net/bugs/1859685 */
#include "cups-private.h"

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

ipp_status_t getIPPRequestStatusCode(ipp_t *ipp);
const ipp_uchar_t *getAttributeDateValue(ipp_attribute_t *attr, int i);
int getAttributeIntegerValue(ipp_attribute_t *attr, int i);
const char *getAttributeStringValue(ipp_attribute_t *attr, int i);
void getAttributeValueRange(ipp_attribute_t *attr, int i, int *lower, int *upper);
void getAttributeValueResolution(ipp_attribute_t *attr, int i, int *xres, int *yres);

#ifndef _CUPS_API_1_7
int ippValidateAttributes(ipp_t *ipp);
http_t *httpConnect2(const char *host, int port, http_addrlist_t *addrlist, int family,
                     http_encryption_t encryption, int blocking, int msec, int *cancel);

# define HTTP_ENCRYPTION_IF_REQUESTED HTTP_ENCRYPT_IF_REQUESTED
# define HTTP_ENCRYPTION_NEVER        HTTP_ENCRYPT_NEVER
# define HTTP_ENCRYPTION_REQUIRED     HTTP_ENCRYPT_REQUIRED
# define HTTP_ENCRYPTION_ALWAYS       HTTP_ENCRYPT_ALWAYS
# define HTTP_STATUS_OK               HTTP_OK
# define HTTP_STATUS_NOT_MODIFIED     HTTP_NOT_MODIFIED
# define IPP_OP_CUPS_GET_PRINTERS     CUPS_GET_PRINTERS
# define IPP_OP_GET_JOB_ATTRIBUTES    IPP_GET_JOB_ATTRIBUTES
# define IPP_STATUS_OK                IPP_OK
# define IPP_STATUS_ERROR_NOT_FOUND   IPP_NOT_FOUND
#endif
