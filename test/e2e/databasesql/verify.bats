#!/usr/bin/env bats

load ../../test_helpers/utilities

LIBRARY_NAME="database/sql"

@test "${LIBRARY_NAME} :: includes db.statement attribute" {
  result=$(span_attributes_for ${LIBRARY_NAME} | jq "select(.key == \"db.statement\").value.stringValue")
  assert_equal "$result" '"SELECT * FROM contacts"'
}

@test "${LIBRARY_NAME} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
