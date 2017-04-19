/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include "dnssd.h"

const char *SERVICE_TYPE = "_privet._tcp";

void resolveCallback(DNSServiceRef sdRef, DNSServiceFlags flags, uint32_t interfaceIndex, DNSServiceErrorType errorCode, const char *fullname, const char *hostname, uint16_t port, uint16_t txtLen, const unsigned char *txtRecord, void *context) {
	if (errorCode != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to resolve (in callback); errorCode = %d", errorCode);
		return;
	}
	struct service_s *service = context;
	service->hostname = strdup(hostname);
	service->port = ntohs(port);
}

void discoverPrintersBrowseCallback(DNSServiceRef sdRef, DNSServiceFlags flags, uint32_t interfaceIndex, DNSServiceErrorType errorCode, const char *name, const char *serviceType, const char *domain, void *context) {
	if (errorCode != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to browse (in callback); errorCode = %d", errorCode);
		return;
	}
	struct service_s *service = calloc(1, sizeof(struct service_s));
	service->hostname = NULL;

	DNSServiceErrorType error = DNSServiceResolve(&sdRef, 0, interfaceIndex, name, SERVICE_TYPE, domain, resolveCallback, service);
	if (error != kDNSServiceErr_NoError) {
		free(service);
		logError("DNS-SD failed to resolve %s; error = %d", name, error);
		return;
	}
	DNSServiceProcessResult(sdRef);

	if (service->hostname == NULL) {
		free(service);
		logError("DNS-SD got a null hostname");
		return;
	}

	service->name = strdup(name);

	struct service_list_s **services = context;
	while (*services != NULL) {
		services = &((*services)->next);
	}
	*services = calloc(1, sizeof(struct service_list_s));
	(*services)->service = service;
}

struct service_list_s *discoverPrinters() {
	struct service_list_s *services = NULL;

	DNSServiceRef sdRef;
	DNSServiceErrorType error = DNSServiceBrowse(&sdRef, 0, 0, SERVICE_TYPE, NULL, discoverPrintersBrowseCallback, (void *) &services);
	if (error != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to browse services; error = %d", error);
		return NULL;
	}

	int dnssd_fd = DNSServiceRefSockFD(sdRef);
	int nfds = dnssd_fd + 1;
	fd_set readfds;
	FD_ZERO(&readfds);
	FD_SET(dnssd_fd, &readfds);
	struct timeval timeout = {1, 0}; // One second.
	int result = select(nfds, &readfds, NULL, NULL, &timeout);
	if (result > 0) {
		// Process result if results exist; this blocks if no results.
		DNSServiceProcessResult(sdRef);
	} else if (result < 0) {
		logError("System error occurred while select()ing: %s", strerror(errno));
	}

	DNSServiceRefDeallocate(sdRef);

	return services;
}

void resolvePrinterBrowseCallback(DNSServiceRef sdRef, DNSServiceFlags flags, uint32_t interfaceIndex, DNSServiceErrorType errorCode, const char *name, const char *serviceType, const char *domain, void *context) {
	if (errorCode != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to browse (in callback); errorCode = %d", errorCode);
		return;
	}

	struct service_s *service = context;
	if (strcmp(name, service->name)) {
		return;
	}

	DNSServiceErrorType error = DNSServiceResolve(&sdRef, 0, interfaceIndex, name, SERVICE_TYPE, domain, resolveCallback, service);
	if (error != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to resolve %s; error = %d", name, error);
		return;
	}
	DNSServiceProcessResult(sdRef);
}

struct service_s *resolvePrinter(const char *name) {
	struct service_s *service = calloc(1, sizeof(struct service_s));
	service->name = (char *)name;

	DNSServiceRef sdRef;
	DNSServiceErrorType error = DNSServiceBrowse(&sdRef, 0, 0, SERVICE_TYPE, NULL, resolvePrinterBrowseCallback, (void *) service);
	if (error != kDNSServiceErr_NoError) {
		logError("DNS-SD failed to resolve %s; error = %d", name, error);
		return NULL;
	}
	DNSServiceProcessResult(sdRef);
	DNSServiceRefDeallocate(sdRef);

	service->name = NULL;
	return service;
}
