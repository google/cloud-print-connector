/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

#include "snmp.h"

const oid    PRINTER_OID[]   = {1,3,6,1,2,1,43};
const size_t PRINTER_OID_LEN = 7;
const long   NON_REPEATERS   = 0,
             MAX_REPETITIONS = 64; // 128 causes some printers to simply not respond.
const size_t MAX_VALUE_LEN   = 1000;

// initialize tames the Net-SNMP library before it is used.
void initialize() {
	// Omit type when converting OID variable value to string.
	netsnmp_ds_set_boolean(NETSNMP_DS_LIBRARY_ID, NETSNMP_DS_LIB_QUICK_PRINT, 1);
	// Omit type error when converting OID variable value to string.
	netsnmp_ds_set_boolean(NETSNMP_DS_LIBRARY_ID, NETSNMP_DS_LIB_QUICKE_PRINT, 1);
	// Don't try to open a .conf file for every getbulk request.
	netsnmp_ds_set_boolean(NETSNMP_DS_LIBRARY_ID, NETSNMP_DS_LIB_DONT_LOAD_HOST_FILES, 1);
	// Disable Net-SNMP logging; this library logs errors.
	netsnmp_register_loghandler(NETSNMP_LOGHANDLER_NONE, 0);
}

// session_error gets the last error in this session,
// in a brief, human-readable format.
//
// Caller frees returned string.
char *session_error(void *session) {
	int liberr, syserr;
	char *errstr;
	snmp_sess_error(session, &liberr, &syserr, &errstr);
	return errstr;
}

// open_error gets the last error in this session,
// after snmp_sess_open failes.
//
// Caller frees returned string.
char *open_error(struct snmp_session *session) {
	int liberr, syserr;
	char *errstr;
	snmp_error(session, &liberr, &syserr, &errstr);
	return errstr;
}

// request executes an SNMP GETBULK request.
char *request(void *sessp, long max_repetitions, oid *name, size_t name_length, struct snmp_pdu **response) {
	struct snmp_pdu *request = snmp_pdu_create(SNMP_MSG_GETBULK);
	request->non_repeaters = NON_REPEATERS;
	request->max_repetitions = max_repetitions;
	snmp_add_null_var(request, name, name_length);
	int status = snmp_sess_synch_response(sessp, request, response);
	if (status != STAT_SUCCESS) {
		char *errstr = session_error(sessp);
		char *err = NULL;
		int failure = asprintf(&err, "SNMP request error: %s", errstr);
		if (failure == -1) {
			err = errstr;
		} else {
			free(errstr);
		}
		return err;
	}

	return NULL;
}

void add_responses(struct variable_list *vars, struct oid_value ***next_ov, oid *name, size_t *name_length) {
	int found = 0;
	for (struct variable_list *var = vars; var; var = var->next_variable) {
		if (var->name_length < PRINTER_OID_LEN ||
				0 != netsnmp_oid_equals(PRINTER_OID, PRINTER_OID_LEN, var->name, PRINTER_OID_LEN)) {
			// This OID does not have the printer OID prefix, so we're done.
			*name_length = 0;
			return;
		}

		oid *objid = malloc(var->name_length * sizeof(oid));
		memmove(objid, var->name, var->name_length * sizeof(oid));

		size_t value_length = 1, out_length = 0;;
		char *value = malloc(value_length);
		sprint_realloc_value((unsigned char **)&value, &value_length, &out_length, 1, var->name, var->name_length, var);

		**next_ov = calloc(1, sizeof(struct oid_value));
		(**next_ov)->name = objid;
		(**next_ov)->name_length = var->name_length;
		(**next_ov)->value = value;
		*next_ov = &(**next_ov)->next;

		*name_length = var->name_length;
		memmove(name, var->name, *name_length * sizeof(oid));
	}

	return;
}

void add_error(struct bulkwalk_response *response, char *error) {
	response->errors_len ++;
	response->errors = realloc(response->errors, response->errors_len * sizeof(char *));
	response->errors[response->errors_len-1] = error;
}

// bulkwalk executes the SNMP GETBULK operation.
// Caller frees returned response.
struct bulkwalk_response *bulkwalk(char *peername, char *community) {
	struct bulkwalk_response *response = calloc(1, sizeof(struct bulkwalk_response));

	void *sessp;
	struct snmp_session session, *sptr;

	snmp_sess_init(&session);
	session.version = SNMP_VERSION_2c;
	session.community = (unsigned char *) community;
	session.community_len = strlen(community);
	session.peername = peername;

	if ((sessp = snmp_sess_open(&session)) == NULL) {
		char *errstr = open_error(&session);
		char *err = NULL;
		int failure = asprintf(&err, "Open SNMP session error: %s", errstr);
		if (failure == -1) {
			err = errstr;
		} else {
			free(errstr);
		}
		add_error(response, err);
		return response;
	}

	oid name[MAX_OID_LEN];
	size_t name_length = PRINTER_OID_LEN;
	memmove(name, PRINTER_OID, name_length * sizeof(oid));

	long max_repetitions = MAX_REPETITIONS;
	struct oid_value **next_ov = &response->ov_root;

	while (1) {
		struct snmp_pdu *subtree;
		char *err = request(sessp, max_repetitions, name, name_length, &subtree);
		if (err != NULL) {
			add_error(response, err);
			snmp_free_pdu(subtree);
			break;
		}

		if (subtree->errstat == SNMP_ERR_NOERROR) {
			add_responses(subtree->variables, &next_ov, name, &name_length);
			snmp_free_pdu(subtree);
			if (name_length == 0) {
				break;
			}
			continue;
		}

		if (subtree->errstat == SNMP_ERR_TOOBIG) {
			// We tried to request too many OIDs at once; request fewer.
			if (max_repetitions <= 1) {
				snmp_free_pdu(subtree);
				break;
			}
			max_repetitions /= 2;
			snmp_free_pdu(subtree);
			continue;
		}

		char *errstr = session_error(sessp);
		err = NULL;
		int failure = asprintf(&err, "SNMP response error (%ld): %s", subtree->errstat, errstr);
		if (failure == -1) {
			err = errstr;
		} else {
			free(errstr);
		}
		add_error(response, err);
		snmp_free_pdu(subtree);
		break;
	}

	snmp_sess_close(sessp);

	return response;
}
