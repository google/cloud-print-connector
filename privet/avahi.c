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

void client_callback(AvahiClient *client, AvahiClientState state,
    void *userdata) {
  printf("client ");
  switch (state) {
  case AVAHI_CLIENT_S_REGISTERING:
    printf("registering\n");
    break;
  case AVAHI_CLIENT_S_RUNNING:
    printf("running\n");
    break;
  case AVAHI_CLIENT_S_COLLISION:
    printf("collision\n");
    break;
  case AVAHI_CLIENT_FAILURE:
    printf("failure\n");
    break;
  case AVAHI_CLIENT_CONNECTING:
    // Waiting for avahi-daemon, which isn't running yet.
    printf("connecting\n");
    break;
  }
  handleClientStateChange(state);
}

void entry_group_callback(AvahiEntryGroup *group, AvahiEntryGroupState state,
    void *userdata) {
  printf("group ");
  switch (state) {
  case AVAHI_ENTRY_GROUP_ESTABLISHED:
    printf("established\n");
    break;
  case AVAHI_ENTRY_GROUP_COLLISION:
    printf("collision\n");
    break;
  case AVAHI_ENTRY_GROUP_FAILURE:
    printf("failure\n");
    break;
  case AVAHI_ENTRY_GROUP_UNCOMMITED:
  case AVAHI_ENTRY_GROUP_REGISTERING:
    printf("uncommitted or registering\n");
    break;
  }
}

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
      AVAHI_CLIENT_NO_FAIL, client_callback, NULL, &error);
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
    AvahiEntryGroup **group, char *serviceName, unsigned short port, char *ty,
    char *url, char *id, char *cs, char **err) {
  *err = NULL;

  *group = avahi_entry_group_new(client, entry_group_callback, NULL);
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
      AVAHI_PROTO_UNSPEC, 0, serviceName, SERVICE_TYPE, NULL, NULL, port,
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
      AVAHI_PROTO_UNSPEC, 0, serviceName, SERVICE_TYPE, NULL, SERVICE_SUBTYPE);
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
    char *serviceName, char *ty, char *url, char *id, char *cs, char **err) {
  *err = NULL;

  char *y, *u, *i, *c;
  asprintf(&y, TY, ty);
  asprintf(&u, URL, url);
  asprintf(&i, ID, id);
  asprintf(&c, CS, cs);

  int error = avahi_entry_group_update_service_txt(group, AVAHI_IF_UNSPEC,
      AVAHI_PROTO_UNSPEC, 0, serviceName, SERVICE_TYPE, NULL,
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

/*
int sleeptime = 2;
int main() {
  char *err;

  AvahiThreadedPoll *threaded_poll;
  AvahiClient *client;
  startAvahiClient(&threaded_poll, &client, &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  AvahiEntryGroup *group1;
  addAvahiGroup(threaded_poll, client, &group1, "justa printa", 12345, "yep just me", "google.com", "myid", "online", &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("added group 1.\n");
  sleep(sleeptime);

  AvahiEntryGroup *group2;
  addAvahiGroup(threaded_poll, client, &group2, "justa nutha printa", 54321, "me again", "google.com", "otherid", "offline", &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("added group 2.\n");
  sleep(sleeptime);

  updateAvahiGroup(threaded_poll, group1, "justa printa", "more of yep", "google.com", "myid", "online", &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }
  printf("updated group 1.\n");
  sleep(sleeptime*2);

  removeAvahiGroup(threaded_poll, group1, &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("removed group1.\n");
  sleep(sleeptime);

  removeAvahiGroup(threaded_poll, group2, &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("removed group2.\n");
  sleep(sleeptime);

  addAvahiGroup(threaded_poll, client, &group2, "born again", 666, "haha", "google.com", "otherotherid", "online", &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("re-added a group2.\n");
  sleep(sleeptime);

  removeAvahiGroup(threaded_poll, group2, &err);
  if (err != NULL) {
    printf("%s\n", err);
    free(err);
    return 1;
  }

  printf("removed group2 again.\n");
  sleep(sleeptime*2);

  stopAvahiClient(threaded_poll, client);
}
*/
