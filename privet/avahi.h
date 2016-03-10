// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux freebsd

#include <avahi-client/publish.h>
#include <avahi-common/error.h>
#include <avahi-common/strlst.h>
#include <avahi-common/thread-watch.h>

#include <stdlib.h> // free

const char *startAvahiClient(AvahiThreadedPoll **threaded_poll, AvahiClient **client);
const char *addAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiClient *client,
    AvahiEntryGroup **group, const char *serviceName, unsigned short port, AvahiStringList *txt);
const char *updateAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    const char *serviceName, AvahiStringList *txt);
const char *removeAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group);
void stopAvahiClient(AvahiThreadedPoll *threaded_poll, AvahiClient *client);
