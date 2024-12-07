#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/database/sql"

@test "${SCOPE} :: includes db.query.text attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[0])
  assert_equal "$result" '"SELECT * FROM contacts"'
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[1])
  assert_equal "$result" "\"INSERT INTO contacts (first_name) VALUES ('Mike')\""
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[2])
  assert_equal "$result" "\"UPDATE contacts SET last_name = 'Santa' WHERE first_name = 'Mike'\""
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[3])
  assert_equal "$result" "\"DELETE FROM contacts WHERE first_name = 'Mike'\""
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[4])
  assert_equal "$result" "\"DROP TABLE contacts\""
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"db.query.text\").value.stringValue" | jq -Rn '[inputs]' | jq -r .[5])
  assert_equal "$result" "\"syntax error\""
}

@test "${SCOPE} :: span name is set correctly" {
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[0])
  assert_equal "$result" '"SELECT contacts"'
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[1])
  assert_equal "$result" '"INSERT contacts"'
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[2])
  assert_equal "$result" '"UPDATE contacts"'
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[3])
  assert_equal "$result" '"DELETE contacts"'
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[4])
  assert_equal "$result" '"DB"'
  result=$(span_names_for ${SCOPE} | jq -Rn '[inputs]' | jq -r .[5])
  assert_equal "$result" '"DB"'
}

@test "${SCOPE} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[2])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[3])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[4])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[5])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${SCOPE} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[2])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[3])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[4])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[5])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${SCOPE} :: parent span ID present and valid in all spans" {
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[2])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[3])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[4])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[5])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "${SCOPE} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
