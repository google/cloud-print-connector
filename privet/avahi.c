// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build !darwin
// +build !windows

#include "avahi.h"
#include "_cgo_export.h"

const char *SERVICE_TYPE = "_privet._tcp",
      *SERVICE_SUBTYPE   = "_printer._sub._privet._tcp";

// startAvahiClient initializes a poll object, and a client.
const char *startAvahiClient(AvahiThreadedPoll **threaded_poll, AvahiClient **client) {
  *threaded_poll = avahi_threaded_poll_new();
  if (!*threaded_poll) {
    return avahi_strerror(avahi_client_errno(*client));
  }

  int error;
  *client = avahi_client_new(avahi_threaded_poll_get(*threaded_poll),
      AVAHI_CLIENT_NO_FAIL, handleClientStateChange, NULL, &error);
  if (!*client) {
    avahi_threaded_poll_free(*threaded_poll);
    return avahi_strerror(error);
  }

  error = avahi_threaded_poll_start(*threaded_poll);
  if (AVAHI_OK != error) {
    avahi_client_free(*client);
    avahi_threaded_poll_free(*threaded_poll);
    return avahi_strerror(error);
  }
  return NULL;
}

const char *addAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiClient *client,
    AvahiEntryGroup **group, const char *service_name, unsigned short port, AvahiStringList *txt) {
  *group = avahi_entry_group_new(client, handleGroupStateChange, (void *)service_name);
  if (!*group) {
    return avahi_strerror(avahi_client_errno(client));
  }

  int error = avahi_entry_group_add_service_strlst(
      *group, AVAHI_IF_UNSPEC, AVAHI_PROTO_UNSPEC, 0, service_name,
      SERVICE_TYPE, NULL, NULL, port, txt);
  if (AVAHI_OK != error) {
    avahi_entry_group_free(*group);
    return avahi_strerror(error);
  }

  error = avahi_entry_group_add_service_subtype(*group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, service_name, SERVICE_TYPE, NULL, SERVICE_SUBTYPE);
  if (AVAHI_OK != error) {
    avahi_entry_group_free(*group);
    return avahi_strerror(error);
  }

  error = avahi_entry_group_commit(*group);
  if (AVAHI_OK != error) {
    avahi_entry_group_free(*group);
    return avahi_strerror(error);
  }
  return NULL;
}

const char *updateAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    const char *service_name, AvahiStringList *txt) {
  int error = avahi_entry_group_update_service_txt_strlst(group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, service_name, SERVICE_TYPE, NULL, txt);
  if (AVAHI_OK != error) {
    return avahi_strerror(error);
  }
  return NULL;
}

const char *removeAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group) {
  int error = avahi_entry_group_free(group);
  if (AVAHI_OK != error) {
    return avahi_strerror(error);
  }
  return NULL;
}

void stopAvahiClient(AvahiThreadedPoll *threaded_poll, AvahiClient *client) {
  avahi_threaded_poll_stop(threaded_poll);
  avahi_client_free(client);
  avahi_threaded_poll_free(threaded_poll);
}
