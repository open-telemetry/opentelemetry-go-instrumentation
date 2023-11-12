// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef _OTEL_TYPES_H
#define _OTEL_TYPES_H

#include "go_types.h"
#include "common.h"

/* Defintions should mimic structs defined in go.opentelemetry.io/otel/attribute */
typedef u64 attr_val_type_t;

#define INVALID 0
#define BOOL 1
#define INT64 2
#define FLOAT64 3
#define STRING 4
#define BOOLSLICE 5
#define INT64SLICE 6
#define FLOAT64SLICE 7
#define STRINGSLICE 8

typedef struct go_otel_attr_value {
	attr_val_type_t  vtype;
	u64              numeric;
	struct go_string string;
	struct go_iface	 slice;
} go_otel_attr_value_t;

typedef struct go_otel_key_value {
	struct go_string     key;
	go_otel_attr_value_t value;
} go_otel_key_value_t;

/* The following structs are the C-formated structs to be used by the eBPF code */
#define OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH 			(256)
#define OTEL_ATTRIBUTE_NUMERIC_VALUES_BUFFER_SIZE 		(32)
#define OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE		(1024)
#define OTEL_ATTRUBUTE_MAX_COUNT 						OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH / 2

typedef struct otel_attr_header {
	u16 val_length;
	u8 vtype;
	u8 reserved;
} otel_attr_header_t;

typedef struct otel_attributes {
	otel_attr_header_t headers[OTEL_ATTRUBUTE_MAX_COUNT];
	char keys[OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH];
	s64 numeric_values[OTEL_ATTRIBUTE_NUMERIC_VALUES_BUFFER_SIZE];
	char str_values[OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE];
} otel_attributes_t;


static __always_inline long convert_go_otel_attributes(struct go_slice *attrs_slice, otel_attributes_t *enc_attrs)
{
	if (attrs_slice == NULL || enc_attrs == NULL){
		return -1;
	}

	bpf_memset((unsigned char*)enc_attrs->headers, 0, sizeof(enc_attrs->headers));
	bpf_memset((unsigned char*)enc_attrs->keys, 0, sizeof(enc_attrs->keys));
	bpf_memset((unsigned char*)enc_attrs->numeric_values, 0, sizeof(enc_attrs->numeric_values));
	bpf_memset((unsigned char*)enc_attrs->str_values, 0, sizeof(enc_attrs->str_values));

	s64 slice_len = 0;
	bpf_probe_read(&slice_len, sizeof(s64), &attrs_slice->len);
	go_otel_key_value_t *go_attr = NULL;
	bpf_probe_read(&go_attr, sizeof(go_otel_key_value_t*), &attrs_slice->array);

	u16 keys_off = 0, str_values_off = 0, numeric_index = 0;
	s64 key_len = 0;
	go_otel_attr_value_t go_attr_value = {0};
	struct go_string go_str = {0};
	s64 bytes_copied = 0;
	for (u32 i = 0;
	 		i < OTEL_ATTRUBUTE_MAX_COUNT &&
			i < slice_len && 
			keys_off < OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH - 1 && 
			str_values_off < OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE - 1 &&
			numeric_index < OTEL_ATTRIBUTE_NUMERIC_VALUES_BUFFER_SIZE;
			i++)
	{
		__builtin_memset(&go_attr_value, 0, sizeof(go_otel_attr_value_t));
		bpf_probe_read(&go_attr_value, sizeof(go_otel_attr_value_t), &go_attr->value);
		if (go_attr_value.vtype == INVALID) {
			break;
		}
		bpf_probe_read(&go_str, sizeof(struct go_string), &go_attr->key);
		if (go_str.len <= 0){
			break;
		}
		if (go_str.len >= OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH - keys_off - 1) {
			break;
		}
		keys_off &= (OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH - 1);
		key_len = get_go_string_from_user_ptr(&go_str, (char*)&enc_attrs->keys[keys_off], OTEL_ATTRIBUTE_KEYS_BUFFER_MAX_LENGTH);
		if (key_len < 0){
			break;
		}
		keys_off += key_len;
		// Keep the null terminator between keys
		keys_off++;

		enc_attrs->headers[i].vtype = go_attr_value.vtype;
		switch (go_attr_value.vtype)
		{
		case BOOL:
		case INT64:
		case FLOAT64:
			bpf_probe_read(&enc_attrs->numeric_values[numeric_index], sizeof(s64), &go_attr_value.numeric);
			numeric_index++;
			break;
		case STRING:
			go_str = go_attr_value.string;
			if (go_str.len <= 0){
				return -1;
			}
			if (go_str.len >= OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE - str_values_off - 1) {
				return 0;
			}
			str_values_off &= (OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE - 1);
			bytes_copied = get_go_string_from_user_ptr(&go_str, &enc_attrs->str_values[str_values_off], OTEL_ATTRIBUTE_STRING_VALUES_BUFFER_SIZE);
			if (bytes_copied < 0){
				return -1;
			}
			str_values_off += bytes_copied;
			// Keep the null terminator between strings
			str_values_off++;
			break;
		// TODO: handle slices
		case BOOLSLICE:
		case INT64SLICE:
		case FLOAT64SLICE:
		case STRINGSLICE:
		case INVALID:
		default:
			break;;
		}
		go_attr++;
	}
	return 0;
}

#endif