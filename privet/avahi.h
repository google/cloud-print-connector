// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build !darwin

#include <avahi-client/publish.h>
#include <avahi-common/thread-watch.h>
#include <avahi-common/error.h>

#include <stdio.h>  // asprintf
#include <stdlib.h> // free

void startAvahiClient(AvahiThreadedPoll **threaded_poll, AvahiClient **client,
    char **err);
void addAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiClient *client,
    AvahiEntryGroup **group, char *serviceName, unsigned short port, char *ty,
    char *url, char *id, char *cs, char **err);
void updateAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    char *serviceName, char *ty, char *url, char *id, char *cs, char **err);
void removeAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    char **err);
void stopAvahiClient(AvahiThreadedPoll *threaded_poll, AvahiClient *client);
