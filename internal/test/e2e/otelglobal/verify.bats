#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="trace-example"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "go-auto :: include tracer version in scope" {
  result=$(spans_received | jq ".scopeSpans[].scope.version")
  assert_equal "$result" '"v1.23.42"'
}

@test "go-auto :: include schema url" {
  result=$(spans_received | jq ".scopeSpans[].schemaUrl")
  assert_equal "$result" '"https://some_schema"'
}

@test "server :: valid int attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"int_key\").value.intValue")
  assert_equal "$result" '"42"'
}

@test "server :: valid string attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"string_key\").value.stringValue")
  assert_equal "$result" '"forty-two"'
}

@test "server :: valid string attribute in child span" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"inner.key\").value.stringValue")
  assert_equal "$result" '"inner.value"'
}

@test "server :: valid bool attribute in child span" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"cat.on_keyboard\").value.boolValue")
  assert_equal "$result" 'true'
}

@test "server :: valid bool attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"bool_key\").value.boolValue")
  assert_equal "$result" 'true'
}

@test "server :: valid float attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"float_key\").value.doubleValue")
  assert_equal "$result" '42.3'
}

@test "server :: trace ID present and valid in child span" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"child override\")" | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "server :: trace ID present and valid in parent span" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"parent\")" | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "server :: span ID present and valid in child span" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"child override\")" | jq ".spanId")
  assert_regex "$trace_id" ${MATCH_A_SPAN_ID}
}

@test "server :: span ID present and valid in parent span" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"parent\")" | jq ".spanId")
  assert_regex "$trace_id" ${MATCH_A_SPAN_ID}
}

@test "server :: parent span ID present and valid in child span" {
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"child override\")" | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "server :: span status present in parent" {
  parent_status_code=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"parent\")" | jq ".status.code")
  assert_equal "$parent_status_code" '1'
}

@test "server :: span status present in child" {
  child_status_code=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"child override\")" | jq ".status.code")
  assert_equal "$child_status_code" '2'
  child_status_description=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"child override\")" | jq ".status.message")
  assert_equal "$child_status_description" '"i deleted the prod db sry"'
}
