#!/usr/bin/env bats

load ../../test_helpers/utilities

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "net/http :: emits a span name '{http.method} {http.route}' (per semconv)" {
  result=$(span_names_for "net/http")
  assert_equal "$result" '"GET /hello"'
}

@test "net/http :: trace ID present in all spans" {
  result=$(spans_from_scope_named "net/http" | jq ".traceId")
  assert_equal "$result" 'hey'
}
