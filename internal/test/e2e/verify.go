// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package e2e contains testing utilities for end-to-end tests.
package e2e

import (
	"testing" // nolint:depguard  // This is a testing utility package.

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// VerifyOTLP applies the provided checks to an OTLP trace payload.
func VerifyOTLP(t *testing.T, data *ptrace.Traces, checks []ResourceSpanVerifier) {
	rss := data.ResourceSpans()
	for i := range rss.Len() {
		for _, rc := range checks {
			rs := rss.At(i)
			if rc.Matcher != nil && !rc.Matcher(rs.Resource()) {
				continue
			}

			if rc.VerifyResource != nil {
				t.Run("Resource", func(t *testing.T) {
					rc.VerifyResource(t, rss.At(i).Resource())
				})
			}

			sss := rs.ScopeSpans()
			for j := range sss.Len() {
				for _, sc := range rc.ScopeSpans {
					ss := sss.At(j)
					if sc.Matcher != nil && !sc.Matcher(ss.Scope()) {
						continue
					}

					if sc.VerifyScope != nil {
						t.Run("Scope", func(t *testing.T) {
							sc.VerifyScope(t, ss.Scope())
						})
					}

					if sc.VerifySchemaURL != nil {
						t.Run("SchemaURL", func(t *testing.T) {
							sc.VerifySchemaURL(t, rs.SchemaUrl())
						})
					}

					spans := ss.Spans()
					for k := range spans.Len() {
						for _, check := range sc.Spans {
							if check.Matcher != nil && !check.Matcher(spans.At(k)) {
								continue
							}

							if check.VerifySpan != nil {
								name := spans.At(k).Name()
								t.Run("Span/"+name, func(t *testing.T) {
									check.VerifySpan(t, spans.At(k))
								})
							}
						}
					}
				}
			}
		}
	}
}

type ResourceSpanVerifier struct {
	Matcher        func(resource pcommon.Resource) bool
	VerifyResource func(t *testing.T, resource pcommon.Resource)
	ScopeSpans     []ScopeSpanVerifier
}

type ScopeSpanVerifier struct {
	Matcher         func(scope pcommon.InstrumentationScope) bool
	VerifyScope     func(t *testing.T, scope pcommon.InstrumentationScope)
	VerifySchemaURL func(t *testing.T, schemaURL string)
	Spans           []SpanVerifier
}

type SpanVerifier struct {
	Matcher    func(span ptrace.Span) bool
	VerifySpan func(t *testing.T, span ptrace.Span)
}
