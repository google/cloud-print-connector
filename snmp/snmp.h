/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// This makes asprintf work properly under GNU.
#ifdef __GNUC__
# ifndef _GNU_SOURCE
#  define _GNU_SOURCE
# endif // _GNU_SOURCE
#endif //__GNUC__

#include <stddef.h> // size_t
#include <stdio.h>  // asprintf
#include <stdlib.h> // calloc, realloc, free
#include <string.h> // memmove

#include <net-snmp/net-snmp-config.h>
#include <net-snmp/net-snmp-includes.h>

struct oid_value {
	struct oid_value *next;
	oid              *name;
	size_t           name_length;
	char             *value;
};

struct bulkwalk_response {
	struct oid_value *ov_root;
	char             **errors;
	size_t           errors_len;
};

void initialize();
struct bulkwalk_response *bulkwalk(char *peername, char *community);
