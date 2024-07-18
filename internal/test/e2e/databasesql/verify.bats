#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/database/sql"

@test "${SCOPE} :: includes db.query.text attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue")
  assert_equal "$result" '"SELECT * FROM contacts"'
}

@test "${SCOPE} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${SCOPE} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${SCOPE} :: parent span ID present and valid in all spans" {
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "${SCOPE} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
