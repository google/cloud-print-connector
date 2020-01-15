#ifndef _CUPS_IPP_PRIVATE_H_
#ifndef CUPS_CUPS_PRIVATE_H_

#define CUPS_CUPS_PRIVATE_H_

/*
 * https://bugs.launchpad.net/bugs/1859685
 *
 * This header is a workaround for LP: #1859685 while the cloud-print-connector
 * doesn't change its implementation to use the correct functions now that cups
 * has deactivated the _IPP_PRIVATE_STRUCTURES workaround.
 *
 * This file contains parts of ipp-private.h from cups v2.3.1-7-g1f2a315c2
 */

typedef union _ipp_request_u
{
	struct
	{
		ipp_uchar_t version[2];
		int op_status;
		int request_id;
	} any;
	struct
	{
		ipp_uchar_t version[2];
		ipp_op_t operation_id;
		int request_id;
	} op;
	struct
	{
		ipp_uchar_t version[2];
		ipp_status_t status_code;
		int request_id;
	} status;
	struct
	{
		ipp_uchar_t version[2];
		ipp_status_t status_code;
		int request_id;
	} event;
} _ipp_request_t;

typedef union _ipp_value_u
{
	int integer;
	char boolean;
	ipp_uchar_t date[11];
	struct
	{
		int xres, yres;
		ipp_res_t units;
	} resolution;
	struct
	{
		int lower, upper;
	} range;
	struct
	{
		char *language;
		char *text;
	} string;
	struct
	{
		int length;
		void *data;
	} unknown;
	ipp_t *collection;
} _ipp_value_t;

struct _ipp_attribute_s
{
	ipp_attribute_t *next;
	ipp_tag_t group_tag, value_tag;
	char *name;
	int num_values;
	_ipp_value_t values[1];
};

struct _ipp_s
{
	ipp_state_t state;
	_ipp_request_t request;
	ipp_attribute_t *attrs;
	ipp_attribute_t *last;
	ipp_attribute_t *current;
	ipp_tag_t curtag;
	ipp_attribute_t *prev;
	int use;
	int atend, curindex;
};

#endif /* CUPS_CUPS_PRIVATE_H_ */
#endif /* _CUPS_IPP_PRIVATE_H_ */
