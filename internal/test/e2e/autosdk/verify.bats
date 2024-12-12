#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/internal/test/e2e/autosdk"

@test "autosdk :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "autosdk :: include tracer name in scope" {
  result=$(spans_received | jq ".scopeSpans[].scope.name")
  assert_equal "$result" "\"$SCOPE\""
}

@test "autosdk :: include tracer version in scope" {
  result=$(spans_received | jq ".scopeSpans[].scope.version")
  assert_equal "$result" '"v1.23.42"'
}

@test "autosdk :: include schema url" {
  result=$(spans_received | jq ".scopeSpans[].schemaUrl")
  assert_equal "$result" '"https://some_schema"'
}

@test "autosdk :: main span :: trace ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"main\")" | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "autosdk :: main span :: span ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"main\")" | jq ".spanId")
  assert_regex "$trace_id" ${MATCH_A_SPAN_ID}
}

@test "autosdk :: main span :: start time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"main\")" | jq ".startTimeUnixNano")
  assert_regex "$timestamp" "946684800000000000"
}

@test "autosdk :: main span :: end time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"main\")" | jq ".endTimeUnixNano")
  assert_regex "$timestamp" "946684805000000000"
}

@test "autosdk :: main span :: event" {
  event=$(span_events ${SCOPE} "main")

  assert_equal $(echo "$event" | jq ".timeUnixNano") '"946684802000000000"'
  assert_equal $(echo "$event" | jq ".name") '"exception"'

  attrs=$(echo "$event" | jq ".attributes[]")

  impact=$(echo "$attrs" | jq "select(.key == \"impact\").value.intValue")
  assert_equal "$impact" '"11"'

  type=$(echo "$attrs" | jq "select(.key == \"exception.type\").value.stringValue")
  assert_equal "$type" '"*errors.errorString"'

  msg=$(echo "$attrs" | jq "select(.key == \"exception.message\").value.stringValue")
  assert_equal "$msg" '"broken"'

  st=$(echo "$attrs" | jq "select(.key == \"exception.stacktrace\")")
  assert_not_empty "$st"
}

@test "autosdk :: main span :: status" {
  status=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"main\")" | jq ".status")
  assert_equal "$(echo $status | jq ".code")" "2"
  assert_equal "$(echo $status | jq ".message")" '"application error"'
}

@test "autosdk :: sig span :: trace ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "autosdk :: sig span :: span ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".spanId")
  assert_regex "$trace_id" ${MATCH_A_SPAN_ID}
}

@test "autosdk :: sig span :: parent span ID" {
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "autosdk :: sig span :: start time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".startTimeUnixNano")
  assert_regex "$timestamp" "946684800000010000"
}

@test "autosdk :: sig span :: end time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".endTimeUnixNano")
  assert_regex "$timestamp" "946684800000110000"
}

@test "autosdk :: Run span :: trace ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "autosdk :: Run span :: span ID" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".spanId")
  assert_regex "$trace_id" ${MATCH_A_SPAN_ID}
}

@test "autosdk :: Run span :: parent span ID" {
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "autosdk :: Run span :: start time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".startTimeUnixNano")
  assert_regex "$timestamp" "946684800000500000"
}

@test "autosdk :: Run span :: end time" {
  timestamp=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".endTimeUnixNano")
  assert_regex "$timestamp" "946684801000000000"
}

@test "autosdk :: Run span :: kind" {
  kind=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"Run\")" | jq ".kind")
  assert_equal "$kind" "2"
}

@test "autosdk :: Run span :: attribute :: user" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"user\").value.stringValue")
  assert_equal "$result" '"Alice"'
}

@test "autosdk :: Run span :: attribute :: admin" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"admin\").value.boolValue")
  assert_equal "$result" 'true'
}

@test "autosdk :: Run span :: link :: traceID" {
  want=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".traceId")
  got=$(span_links ${SCOPE} "Run" | jq ".traceId")
  assert_equal "$got" "$want"
}

@test "autosdk :: Run span :: link :: spanID" {
  want=$(spans_from_scope_named ${SCOPE} | jq "select(.name == \"sig\")" | jq ".spanId")
  got=$(span_links ${SCOPE} "Run" | jq ".spanId")
  assert_equal "$got" "$want"
}

@test "autosdk :: Run span :: link :: attributes" {
  got=$(span_links ${SCOPE} "Run" | jq ".attributes[] | select(.key == \"data\").value.stringValue")
  assert_equal "$got" '"Hello World"'
}

@test "autosdk :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
