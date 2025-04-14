// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package autosdk contains the verification code for the autosdk e2e test.
package autosdk

import (
	"testing" // nolint:depguard  // This is a testing utility package.
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

const (
	scopeName = "go.opentelemetry.io/auto/internal/test/e2e/autosdk"
	scopeVer  = "v1.23.42"
)

// Y2K (January 1, 2000).
var y2k = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Verifications are the verifications to be run for the autosdk e2e test.
var Verifications = []e2e.ResourceSpanVerifier{
	{
		VerifyResource: func(t *testing.T, res pcommon.Resource) {
			t.Run("ServiceName", func(t *testing.T) {
				val, ok := res.Attributes().Get("service.name")
				if !ok || val.Str() != "sample-app" {
					t.Errorf("expected service.name to be 'sample-app', got %q", val.Str())
				}
			})
		},
		ScopeSpans: []e2e.ScopeSpanVerifier{
			{
				Matcher: func(scope pcommon.InstrumentationScope) bool {
					return scope.Name() == scopeName
				},
				VerifyScope: func(t *testing.T, scope pcommon.InstrumentationScope) {
					t.Run("Version", func(t *testing.T) {
						if scope.Version() != scopeVer {
							t.Errorf(
								"expected scope version to be %q, got %q",
								scopeVer,
								scope.Version(),
							)
						}
					})
				},
				VerifySchemaURL: func(t *testing.T, url string) {
					if url != semconv.SchemaURL {
						t.Errorf("expected schema URL to be %q, got %q", semconv.SchemaURL, url)
					}
				},

				Spans: []e2e.SpanVerifier{
					{
						Matcher: func(span ptrace.Span) bool {
							return span.Name() == "main"
						},
						VerifySpan: func(t *testing.T, span ptrace.Span) {
							t.Run("TraceID", func(t *testing.T) {
								if span.TraceID().IsEmpty() {
									t.Error("expected TraceID to be set")
								}
							})

							t.Run("SpanID", func(t *testing.T) {
								if span.SpanID().IsEmpty() {
									t.Error("expected SpanID to be set")
								}
							})

							t.Run("StartTime", func(t *testing.T) {
								if !span.StartTimestamp().AsTime().Equal(y2k) {
									t.Errorf(
										"expected StartTime to be %v, got %v",
										y2k,
										span.StartTimestamp().AsTime(),
									)
								}
							})

							t.Run("EndTime", func(t *testing.T) {
								want := y2k.Add(5 * time.Second)
								if !span.EndTimestamp().AsTime().Equal(want) {
									t.Errorf(
										"expected EndTime to be %v, got %v",
										want,
										span.EndTimestamp().AsTime(),
									)
								}
							})

							t.Run("SpanKind", func(t *testing.T) {
								want := ptrace.SpanKindInternal
								if k := span.Kind(); k != want {
									t.Errorf("expected SpanKind to be %q, got %q", want, k)
								}
							})

							t.Run("StatusCode", func(t *testing.T) {
								want := ptrace.StatusCodeError
								if c := span.Status().Code(); c != want {
									t.Errorf("expected StatusCode to be %q, got %q", want, c)
								}
							})

							t.Run("StatusMessage", func(t *testing.T) {
								want := "application error"
								if m := span.Status().Message(); m != want {
									t.Errorf("expected StatusMessage to be %q, got %q", want, m)
								}
							})

							t.Run("Events", func(t *testing.T) {
								events := span.Events()
								if events.Len() != 1 {
									t.Fatalf("expected 1 events, got %d", events.Len())
								}
								event := events.At(0)

								want := "exception"
								if event.Name() != want {
									t.Errorf(
										"expected event name to be %q, got %q",
										want,
										events.At(0).Name(),
									)
								}

								wantTS := y2k.Add(2 * time.Second)
								if !event.Timestamp().AsTime().Equal(wantTS) {
									t.Errorf(
										"expected event timestamp to be %v, got %v",
										wantTS,
										events.At(0).Timestamp().AsTime(),
									)
								}

								attrs := event.Attributes()

								wantAttr := "impact"
								if attr, ok := attrs.Get(wantAttr); !ok || attr.Int() != 11 {
									t.Errorf(
										"expected event attribute %q to be %d, got %d",
										wantAttr,
										11,
										attr.Int(),
									)
								}

								wantAttr = "exception.type"
								if attr, ok := attrs.Get(wantAttr); !ok ||
									attr.Str() != "*errors.errorString" {
									t.Errorf(
										"expected event attribute %q to be %s, got %s",
										wantAttr,
										"*errors.errorString",
										attr.Str(),
									)
								}

								wantAttr = "exception.message"
								if attr, ok := attrs.Get(wantAttr); !ok || attr.Str() != "broken" {
									t.Errorf(
										"expected event attribute %q to be %s, got %s",
										wantAttr,
										"broken",
										attr.Str(),
									)
								}

								wantAttr = "exception.stacktrace"
								if _, ok := attrs.Get(wantAttr); !ok {
									t.Errorf("expected event attribute %q to be in event", wantAttr)
								}
							})
						},
					},
					{
						Matcher: func(span ptrace.Span) bool {
							return span.Name() == "sig"
						},
						VerifySpan: func(t *testing.T, span ptrace.Span) {
							t.Run("TraceID", func(t *testing.T) {
								if span.TraceID().IsEmpty() {
									t.Error("expected TraceID to be set")
								}
							})

							t.Run("SpanID", func(t *testing.T) {
								if span.SpanID().IsEmpty() {
									t.Error("expected SpanID to be set")
								}
							})

							t.Run("ParentSpanID", func(t *testing.T) {
								if span.ParentSpanID().IsEmpty() {
									t.Error("expected ParentSpanID to be set")
								}
							})

							t.Run("StartTime", func(t *testing.T) {
								want := y2k.Add(10 * time.Microsecond)
								if !span.StartTimestamp().AsTime().Equal(want) {
									t.Errorf(
										"expected StartTime to be %v, got %v",
										want,
										span.StartTimestamp().AsTime(),
									)
								}
							})

							t.Run("EndTime", func(t *testing.T) {
								want := y2k.Add(110 * time.Microsecond)
								if !span.EndTimestamp().AsTime().Equal(want) {
									t.Errorf(
										"expected EndTime to be %v, got %v",
										want,
										span.EndTimestamp().AsTime(),
									)
								}
							})

							t.Run("SpanKind", func(t *testing.T) {
								want := ptrace.SpanKindInternal
								if k := span.Kind(); k != want {
									t.Errorf("expected SpanKind to be %q, got %q", want, k)
								}
							})
						},
					},
					{
						Matcher: func(span ptrace.Span) bool {
							return span.Name() == "Run"
						},
						VerifySpan: func(t *testing.T, span ptrace.Span) {
							t.Run("TraceID", func(t *testing.T) {
								if span.TraceID().IsEmpty() {
									t.Error("expected TraceID to be set")
								}
							})

							t.Run("SpanID", func(t *testing.T) {
								if span.SpanID().IsEmpty() {
									t.Error("expected SpanID to be set")
								}
							})

							t.Run("ParentSpanID", func(t *testing.T) {
								if span.ParentSpanID().IsEmpty() {
									t.Error("expected ParentSpanID to be set")
								}
							})

							t.Run("StartTime", func(t *testing.T) {
								want := y2k.Add(500 * time.Microsecond)
								if !span.StartTimestamp().AsTime().Equal(want) {
									t.Errorf(
										"expected StartTime to be %v, got %v",
										want,
										span.StartTimestamp().AsTime(),
									)
								}
							})

							t.Run("EndTime", func(t *testing.T) {
								want := y2k.Add(1 * time.Second)
								if !span.EndTimestamp().AsTime().Equal(want) {
									t.Errorf(
										"expected EndTime to be %v, got %v",
										want,
										span.EndTimestamp().AsTime(),
									)
								}
							})

							t.Run("SpanKind", func(t *testing.T) {
								want := ptrace.SpanKindServer
								if k := span.Kind(); k != want {
									t.Errorf("expected SpanKind to be %q, got %q", want, k)
								}
							})

							t.Run("Attributes", func(t *testing.T) {
								attrs := span.Attributes()
								wantAttr := "user"
								if attr, ok := attrs.Get(wantAttr); !ok || attr.Str() != "Alice" {
									t.Errorf(
										"expected attribute %q to be %q, got %q",
										wantAttr,
										"alice",
										attr.Str(),
									)
								}

								wantAttr = "admin"
								if attr, ok := attrs.Get(wantAttr); !ok || !attr.Bool() {
									t.Errorf(
										"expected attribute %q to be %t, got %t",
										wantAttr,
										true,
										attr.Bool(),
									)
								}
							})

							t.Run("Links", func(t *testing.T) {
								links := span.Links()
								if links.Len() != 1 {
									t.Fatalf("expected 1 links, got %d", links.Len())
								}
								link := links.At(0)

								if link.TraceID().IsEmpty() {
									t.Error("expected TraceID to be set")
								}

								if link.SpanID().IsEmpty() {
									t.Error("expected SpanID to be set")
								}

								wantAttr := "data"
								if attr, ok := link.Attributes().Get(wantAttr); !ok ||
									attr.Str() != "Hello World" {
									t.Errorf(
										"expected link attribute %q to be %q, got %q",
										wantAttr,
										"Hello World",
										attr.Str(),
									)
								}
							})
						},
					},
				},
			},
		},
	},
}
