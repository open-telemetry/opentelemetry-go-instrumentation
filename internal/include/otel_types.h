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

/* In the SDK the key is a string, but we must thave a limit */
#define OTEL_ATTRIBUTE_KEY_MAX_LENGTH 				(64)
#define OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE 		(1024)
#define OTEL_ATTRIBUTE_VALUE_MAX_BOOL_SLICE_SIZE 	(OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE / sizeof(bool))
#define OTEL_ATTRIBUTE_VALUE_MAX_INT64_SLICE_SIZE 	(OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE / sizeof(s64))
#define OTEL_ATTRIBUTE_VALUE_MAX_FLOAT64_SLICE_SIZE (OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE / sizeof(double))

typedef struct otel_attribute {
	attr_val_type_t vtype;
	char key[OTEL_ATTRIBUTE_KEY_MAX_LENGTH];
	// union
	// {
	// 	s64 int_value;// INT64, BOOL
	// 	double double_value; // FLOAT64
	// 	bool bool_buffer[OTEL_ATTRIBUTE_VALUE_MAX_BOOL_SLICE_SIZE]; // BOOLSLICE
	// 	s64 int_buffer[OTEL_ATTRIBUTE_VALUE_MAX_INT64_SLICE_SIZE]; // INT64SLICE
	// 	double double_buffer[OTEL_ATTRIBUTE_VALUE_MAX_FLOAT64_SLICE_SIZE]; // FLOAT64SLICE
	// 	char buf[OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE];
	// };
	char buf[OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE];
} otel_attribute_t;

static __always_inline long convert_go_otel_attribute(go_otel_key_value_t *go_attr, otel_attribute_t *ebpf_attr)
{
	go_otel_attr_value_t go_attr_value = {0};
	bpf_probe_read(&go_attr_value, sizeof(go_otel_attr_value_t), &go_attr->value);
	if (go_attr_value.vtype == INVALID) {
		return -1;
	}
	ebpf_attr->vtype = go_attr_value.vtype;
	if (get_go_string_from_user_ptr(&go_attr->key, ebpf_attr->key, OTEL_ATTRIBUTE_KEY_MAX_LENGTH) < 0){
		return -1;
	}
	long bytes_copied = 0;
	switch (go_attr_value.vtype)
	{
	case BOOL:
	case INT64:
	case FLOAT64:
		bpf_probe_read(ebpf_attr->buf, sizeof(s64), &go_attr_value.numeric);
		bytes_copied = sizeof(s64);
		break;
	case STRING:
		bytes_copied = get_go_string_from_user_ptr(&go_attr_value.string, ebpf_attr->buf, OTEL_ATTRIBUTE_VALUE_MAX_BUFFER_SIZE);
		break;
	case BOOLSLICE:
		// TODO
		return -1;
	case INT64SLICE:
		// TODO
		return -1;
	case FLOAT64SLICE:
		// TODO
		return -1;
	case STRINGSLICE:
		// TODO
		return -1;
	case INVALID:
	default:
		return -1;
	}

	return bytes_copied;
}

static __always_inline void convert_attributes_slice(struct go_slice *attrs_slice, otel_attribute_t *attrs, u8 max_attrs)
{
	if (attrs_slice == NULL || attrs == NULL){
		return;
	}
	s64 slice_len = 0;
	bpf_probe_read(&slice_len, sizeof(s64), &attrs_slice->len);
	go_otel_key_value_t *go_attrs = NULL;
	bpf_probe_read(&go_attrs, sizeof(go_otel_key_value_t*), &attrs_slice->array);
	u8 attrs_count = ((slice_len > 4) ? 4 : slice_len);
	for (u32 i = 0; i < 4; i++)
	{
		if (i >= slice_len){
			break;
		}
		if (convert_go_otel_attribute(&go_attrs[i], &attrs[i]) < 0){
			break;
		}
	}
}



#endif