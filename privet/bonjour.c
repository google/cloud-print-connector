// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build darwin

#include "bonjour.h"
#include "_cgo_export.h"

// streamErrorToString converts a CFStreamError to a string.
char *streamErrorToString(CFStreamError *error) {
	const char *errorDomain;
	switch (error->domain) {
	case kCFStreamErrorDomainCustom:
		errorDomain = "custom";
		break;
	case kCFStreamErrorDomainPOSIX:
		errorDomain = "POSIX";
		break;
	case kCFStreamErrorDomainMacOSStatus:
		errorDomain = "MacOS status";
		break;
	default:
		errorDomain = "unknown";
		break;
	}

	char *err = NULL;
	asprintf(&err, "domain %s code %d", errorDomain, error->error);
	return err;
}

void registerCallback(CFNetServiceRef service, CFStreamError *streamError, void *info) {
	CFStringRef printerName = (CFStringRef)info;
	char *printerNameC = malloc(sizeof(char) * (CFStringGetLength(printerName) + 1));
	char *streamErrorC = streamErrorToString(streamError);
	char *error = NULL;
	asprintf(&error, "Error while announcing Bonjour service for printer %s: %s",
			printerNameC, streamErrorC);

	logBonjourError(error);

	CFRelease(printerName);
	free(printerNameC);
	free(streamErrorC);
	free(error);
}

// startBonjour starts and returns a bonjour service.
//
// Returns a registered service. Returns NULL and sets err on failure.
CFNetServiceRef startBonjour(const char *name, const char *type, unsigned short int port, const char *ty, const char *url, const char *id, const char *cs, char **err) {
	CFStringRef nameCF = CFStringCreateWithCString(NULL, name, kCFStringEncodingASCII);
	CFStringRef typeCF = CFStringCreateWithCString(NULL, type, kCFStringEncodingASCII);
	CFStringRef tyCF = CFStringCreateWithCString(NULL, ty, kCFStringEncodingASCII);
	CFStringRef urlCF = CFStringCreateWithCString(NULL, url, kCFStringEncodingASCII);
	CFStringRef idCF = CFStringCreateWithCString(NULL, id, kCFStringEncodingASCII);
	CFStringRef csCF = CFStringCreateWithCString(NULL, cs, kCFStringEncodingASCII);

	CFMutableDictionaryRef dict = CFDictionaryCreateMutable(NULL, 0,
			&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFDictionarySetValue(dict, CFSTR("txtvers"), CFSTR("1"));
	CFDictionarySetValue(dict, CFSTR("ty"), typeCF);
	CFDictionarySetValue(dict, CFSTR("url"), urlCF);
	CFDictionarySetValue(dict, CFSTR("type"), CFSTR("printer"));
	CFDictionarySetValue(dict, CFSTR("id"), idCF);
	CFDictionarySetValue(dict, CFSTR("cs"), csCF);
	CFDataRef txt = CFNetServiceCreateTXTDataWithDictionary(NULL, dict);

	CFNetServiceRef service = CFNetServiceCreate(NULL, CFSTR("local"), typeCF, nameCF, port);
	CFNetServiceSetTXTData(service, txt);
	// context now owns n, and will release n when service is released.
	CFNetServiceClientContext context = {0, (void *) nameCF, NULL, CFRelease, NULL};
	CFNetServiceSetClient(service, registerCallback, &context);
	CFNetServiceScheduleWithRunLoop(service, CFRunLoopGetCurrent(), kCFRunLoopCommonModes);

	CFOptionFlags options = kCFNetServiceFlagNoAutoRename;
	CFStreamError error;

	if (!CFNetServiceRegisterWithOptions(service, options, &error)) {
		char *errorString = streamErrorToString(&error);
		asprintf(err, "Failed to register Bonjour service: %s", errorString);
		free(errorString);
		CFRelease(service);
		service = NULL;
	}

	CFRelease(typeCF);
	CFRelease(tyCF);
	CFRelease(urlCF);
	CFRelease(idCF);
	CFRelease(csCF);
	CFRelease(dict);
	CFRelease(txt);

	return service;
}

// updateBonjour updates the TXT record of service.
void updateBonjour(CFNetServiceRef service, const char *ty, const char *url, const char *id, const char *cs) {
	CFStringRef tyCF = CFStringCreateWithCString(NULL, ty, kCFStringEncodingASCII);
	CFStringRef urlCF = CFStringCreateWithCString(NULL, url, kCFStringEncodingASCII);
	CFStringRef idCF = CFStringCreateWithCString(NULL, id, kCFStringEncodingASCII);
	CFStringRef csCF = CFStringCreateWithCString(NULL, cs, kCFStringEncodingASCII);

	CFMutableDictionaryRef dict = CFDictionaryCreateMutable(NULL, 0,
			&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFDictionarySetValue(dict, CFSTR("txtvers"), CFSTR("1"));
	CFDictionarySetValue(dict, CFSTR("ty"), tyCF);
	CFDictionarySetValue(dict, CFSTR("url"), urlCF);
	CFDictionarySetValue(dict, CFSTR("type"), CFSTR("printer"));
	CFDictionarySetValue(dict, CFSTR("id"), idCF);
	CFDictionarySetValue(dict, CFSTR("cs"), csCF);
	CFDataRef txt = CFNetServiceCreateTXTDataWithDictionary(NULL, dict);

	CFNetServiceSetTXTData(service, txt);

	CFRelease(tyCF);
	CFRelease(urlCF);
	CFRelease(idCF);
	CFRelease(csCF);
	CFRelease(dict);
	CFRelease(txt);
}

// stopBonjour stops service and frees associated resources.
void stopBonjour(CFNetServiceRef service) {
	CFNetServiceUnscheduleFromRunLoop(service, CFRunLoopGetCurrent(), kCFRunLoopCommonModes);
	CFNetServiceSetClient(service, NULL, NULL);
	CFNetServiceCancel(service);
	CFRelease(service);
}
