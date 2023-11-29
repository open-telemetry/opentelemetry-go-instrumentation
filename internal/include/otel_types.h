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

// Injected in init
volatile const attr_val_type_t attr_type_invalid;

volatile const attr_val_type_t attr_type_bool;
volatile const attr_val_type_t attr_type_int64;
volatile const attr_val_type_t attr_type_float64;
volatile const attr_val_type_t attr_type_string;

volatile const attr_val_type_t attr_type_boolslice;
volatile const attr_val_type_t attr_type_int64slice;
volatile const attr_val_type_t attr_type_float64slice;
volatile const attr_val_type_t attr_type_stringslice;

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

#define OTEL_ATTRIBUTE_KEY_MAX_LEN      (32)
#define OTEL_ATTRIBUTE_VALUE_MAX_LEN    (128)
#define OTEL_ATTRUBUTE_MAX_COUNT        (16)

typedef struct otel_attirbute {
	u16 val_length;
	u8 vtype;
	u8 reserved;
	char key[OTEL_ATTRIBUTE_KEY_MAX_LEN];
	char value[OTEL_ATTRIBUTE_VALUE_MAX_LEN];
} otel_attirbute_t;

typedef struct otel_attributes {
	otel_attirbute_t attrs[OTEL_ATTRUBUTE_MAX_COUNT];
	u8 valid_attrs;
}__attribute__((packed)) otel_attributes_t;

static __always_inline bool set_attr_value(otel_attirbute_t *attr, go_otel_attr_value_t *go_attr_value)
{
	if (attr == NULL || go_attr_value == NULL){
		return false;
	}

	attr_val_type_t vtype = go_attr_value->vtype;

	if (vtype == attr_type_invalid) {
		bpf_printk("Invalid attribute value type\n");
		return false;
	}

	// Constant size values
	if (vtype == attr_type_bool ||
		vtype == attr_type_int64 ||
		vtype == attr_type_float64) {
		bpf_probe_read(attr->value, sizeof(s64), &go_attr_value->numeric);
		return true;
	}

	// String values
	if (vtype == attr_type_string) {
		if (go_attr_value->string.len <= 0){
			return false;
		}
		if (go_attr_value->string.len >= OTEL_ATTRIBUTE_VALUE_MAX_LEN) {
			bpf_printk("Aattribute string value is too long\n");
			return false;
		}
		return get_go_string_from_user_ptr(&go_attr_value->string, attr->value, OTEL_ATTRIBUTE_VALUE_MAX_LEN);
	}

	// TODO: handle slices
	return false;
}

static __always_inline void convert_go_otel_attributes(void *attrs_buf, s64 slice_len, otel_attributes_t *enc_attrs)
{
	if (attrs_buf == NULL || enc_attrs == NULL){
		return;
	}

	if (slice_len < 1) {
		return;
	}

	s64 num_attrs = slice_len < OTEL_ATTRUBUTE_MAX_COUNT ? slice_len : OTEL_ATTRUBUTE_MAX_COUNT;
	go_otel_key_value_t *go_attr = (go_otel_key_value_t*)attrs_buf;
	go_otel_attr_value_t go_attr_value = {0};
	struct go_string go_str = {0};
	u8 valid_attrs = 0;

	for (u32 go_attr_index = 0; go_attr_index < num_attrs; go_attr_index++) {
		__builtin_memset(&go_attr_value, 0, sizeof(go_otel_attr_value_t));
		// Read the value struct
		bpf_probe_read(&go_attr_value, sizeof(go_otel_attr_value_t), &go_attr[go_attr_index].value);

		if (go_attr_value.vtype == attr_type_invalid) {
			continue;
		}

		// Read the key string
		bpf_probe_read(&go_str, sizeof(struct go_string), &go_attr[go_attr_index].key);
		if (go_str.len <= 0){
			continue;
		}
		if (go_str.len >= OTEL_ATTRIBUTE_KEY_MAX_LEN) {
			// key string is too large
			bpf_printk("Attribute key string is too long\n");
			continue;
		}

		if (!get_go_string_from_user_ptr(&go_str, enc_attrs->attrs[valid_attrs].key, OTEL_ATTRIBUTE_KEY_MAX_LEN)) {
			continue;
		}

		if (!set_attr_value(&enc_attrs->attrs[valid_attrs], &go_attr_value)) {
			continue;
		}

		enc_attrs->attrs[valid_attrs].vtype = go_attr_value.vtype;
		valid_attrs++;
	}

	enc_attrs->valid_attrs = valid_attrs;
}

#endif