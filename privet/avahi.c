// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build !darwin

#include "avahi.h"
#include "_cgo_export.h"

const char *SERVICE_TYPE = "_privet._tcp",
      *SERVICE_SUBTYPE   = "_printer._sub._privet._tcp";

const char *TXTVERS = "txtvers=1",
      *TY           = "ty=%s",
      *URL          = "url=%s",
      *TYPE         = "type=printer",
      *ID           = "id=%s",
      *CS           = "cs=%s";

// startAvahiClient initializes a poll object, and a client.
void startAvahiClient(AvahiThreadedPoll **threaded_poll, AvahiClient **client,
    char **err) {
  *err = NULL;

  *threaded_poll = avahi_threaded_poll_new();
  if (!*threaded_poll) {
    asprintf(err, "failed to create avahi threaded_poll: %s",
        avahi_strerror(avahi_client_errno(*client)));
    return;
  }

  int error;
  *client = avahi_client_new(avahi_threaded_poll_get(*threaded_poll),
      AVAHI_CLIENT_NO_FAIL, handleClientStateChange, NULL, &error);
  if (!*client) {
    asprintf(err, "failed to create avahi client: %s", avahi_strerror(error));
    avahi_threaded_poll_free(*threaded_poll);
    return;
  }

  error = avahi_threaded_poll_start(*threaded_poll);
  if (AVAHI_OK != error) {
    asprintf(err, "failed to start avahi threaded_poll: %s", avahi_strerror(error));
    avahi_client_free(*client);
    avahi_threaded_poll_free(*threaded_poll);
    return;
  }
}

void addAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiClient *client,
    AvahiEntryGroup **group, char *service_name, unsigned short port, char *ty,
    char *url, char *id, char *cs, char **err) {
  *err = NULL;

  *group = avahi_entry_group_new(client, handleGroupStateChange, service_name);
  if (!*group) {
    asprintf(err, "avahi_entry_group_new error: %s", avahi_strerror(avahi_client_errno(client)));
    return;
  }

  char *y, *u, *i, *c;
  asprintf(&y, TY, ty);
  asprintf(&u, URL, url);
  asprintf(&i, ID, id);
  asprintf(&c, CS, cs);

  int error = avahi_entry_group_add_service(*group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, service_name, SERVICE_TYPE, NULL, NULL, port,
      TXTVERS, y, u, TYPE, i, c, NULL);
  free(y);
  free(u);
  free(i);
  free(c);
  if (AVAHI_OK != error) {
    asprintf(err, "add avahi service failed: %s", avahi_strerror(error));
    avahi_entry_group_free(*group);
    return;
  }

  error = avahi_entry_group_add_service_subtype(*group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, service_name, SERVICE_TYPE, NULL, SERVICE_SUBTYPE);
  if (AVAHI_OK != error) {
    asprintf(err, "add avahi service subtype failed: %s", avahi_strerror(error));
    avahi_entry_group_free(*group);
    return;
  }

  error = avahi_entry_group_commit(*group);
  if (AVAHI_OK != error) {
    asprintf(err, "add avahi service failed: %s", avahi_strerror(error));
    avahi_entry_group_free(*group);
    return;
  }
}

void updateAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    char *service_name, char *ty, char *url, char *id, char *cs, char **err) {
  *err = NULL;

  char *y, *u, *i, *c;
  asprintf(&y, TY, ty);
  asprintf(&u, URL, url);
  asprintf(&i, ID, id);
  asprintf(&c, CS, cs);

  int error = avahi_entry_group_update_service_txt(group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, service_name, SERVICE_TYPE, NULL,
      TXTVERS, y, u, TYPE, i, c, NULL);
  free(y);
  free(u);
  free(i);
  free(c);
  if (AVAHI_OK != error) {
    asprintf(err, "update avahi service failed: %s", avahi_strerror(error));
    return;
  }
}

void removeAvahiGroup(AvahiThreadedPoll *threaded_poll, AvahiEntryGroup *group,
    char **err) {
  *err = NULL;

  int error = avahi_entry_group_free(group);
  if (AVAHI_OK != error) {
    asprintf(err, "remove avahi group failed: %s", avahi_strerror(error));
    return;
  }
}

void stopAvahiClient(AvahiThreadedPoll *threaded_poll, AvahiClient *client) {
  avahi_threaded_poll_stop(threaded_poll);
  avahi_client_free(client);
  avahi_threaded_poll_free(threaded_poll);
}
