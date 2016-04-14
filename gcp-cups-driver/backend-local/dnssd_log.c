/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include "dnssd_log.h"

const char *LEVEL_EMERG = "EMERG",
			*LEVEL_ALERT      = "ALERT",
			*LEVEL_CRIT      	= "CRIT",
			*LEVEL_ERROR      = "ERROR",
			*LEVEL_WARNING    = "WARNING",
			*LEVEL_NOTICE     = "NOTICE",
			*LEVEL_INFO       = "INFO",
			*LEVEL_DEBUG      = "DEBUG",
			*LEVEL_DEBUG2     = "DEBUG2";

void logLevel(const char *level, const char *format, va_list args) {
	char *new_format = NULL;
	int err = asprintf(&new_format, "%s: %s\n", level, format);
	if (err < 0) {
		fprintf(stderr, "%s: The function asprintf() failed to malloc", LEVEL_CRIT);
		return;
	}

	vfprintf(stderr, new_format, args);

	free(new_format);
}

void logError(const char *format, ...) {
	va_list args;
	va_start(args, format);
	logLevel(LEVEL_ERROR, format, args);
	va_end(args);
}
