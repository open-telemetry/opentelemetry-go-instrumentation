// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package utils is an overly-generic utility package that provides a catch-all
// for instrumentation utilities.
//
// New functionality should not be added to this package. Instead it should be
// added to an appropriately named and scoped package for the functionality
// being added to follow Go programming best practices.
package utils

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
)

func Attributes(dest pcommon.Map, attrs ...attribute.KeyValue) {
	for _, attr := range attrs {
		setAttr(dest, attr)
	}
}

func setAttr(dest pcommon.Map, attr attribute.KeyValue) {
	switch attr.Value.Type() {
	case attribute.BOOL:
		dest.PutBool(string(attr.Key), attr.Value.AsBool())
	case attribute.INT64:
		dest.PutInt(string(attr.Key), attr.Value.AsInt64())
	case attribute.FLOAT64:
		dest.PutDouble(string(attr.Key), attr.Value.AsFloat64())
	case attribute.STRING:
		dest.PutStr(string(attr.Key), attr.Value.AsString())
	case attribute.BOOLSLICE:
		s := dest.PutEmptySlice(string(attr.Key))
		for _, v := range attr.Value.AsBoolSlice() {
			s.AppendEmpty().SetBool(v)
		}
	case attribute.INT64SLICE:
		s := dest.PutEmptySlice(string(attr.Key))
		for _, v := range attr.Value.AsInt64Slice() {
			s.AppendEmpty().SetInt(v)
		}
	case attribute.FLOAT64SLICE:
		s := dest.PutEmptySlice(string(attr.Key))
		for _, v := range attr.Value.AsFloat64Slice() {
			s.AppendEmpty().SetDouble(v)
		}
	case attribute.STRINGSLICE:
		s := dest.PutEmptySlice(string(attr.Key))
		for _, v := range attr.Value.AsStringSlice() {
			s.AppendEmpty().SetStr(v)
		}
	}
}
