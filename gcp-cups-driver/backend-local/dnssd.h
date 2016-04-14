/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include <dns_sd.h>
#include "dnssd_log.h"

#include <arpa/inet.h>  // ntohs
#include <errno.h>      // errno
#include <stdlib.h>     // calloc, free
#include <string.h>     // strcmp, strdup
#include <sys/select.h> // select
#include <sys/time.h>   // timeval

struct service_s {
	char     *name;
	char     *hostname;
	uint16_t port;
};

struct service_list_s {
	struct service_s      *service;
	struct service_list_s *next;
};

struct service_list_s *discoverPrinters();
struct service_s *resolvePrinter(const char *name);
