#!/usr/bin/env bats

load ../../test_helpers/utilities

LIBRARY_NAME="github.com/gorilla/mux"
# TODO: spans currently show net/http as the library name
TEMP_LIBRARY_NAME="net/http"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

# TODO: span name should include http.method per spec
# @test "${LIBRARY_NAME} :: emits a span name '{http.method} {http.target}' (per semconv)" {
@test "${LIBRARY_NAME} :: emits a span name '{http.route}'" {
  result=$(span_names_for ${TEMP_LIBRARY_NAME})
  assert_equal "$result" '"/users/foo"'
}

@test "${LIBRARY_NAME} :: includes http.method attribute" {
  result=$(span_attributes_for ${TEMP_LIBRARY_NAME} | jq "select(.key == \"http.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "${LIBRARY_NAME} :: includes http.target attribute" {
  result=$(span_attributes_for ${TEMP_LIBRARY_NAME} | jq "select(.key == \"http.target\").value.stringValue")
  assert_equal "$result" '"/users/foo"'
}

@test "${LIBRARY_NAME} :: trace ID present in all spans" {
  result=$(spans_from_scope_named ${TEMP_LIBRARY_NAME} | jq ".traceId")
  assert_not_empty "$result"
}

@test "${LIBRARY_NAME} :: span ID present in all spans" {
  result=$(spans_from_scope_named ${TEMP_LIBRARY_NAME} | jq ".spanId")
  assert_not_empty "$result"
}