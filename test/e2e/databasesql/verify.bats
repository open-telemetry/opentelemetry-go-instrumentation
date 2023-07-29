#!/usr/bin/env bats

load ../../test_helpers/utilities

LIBRARY_NAME="database/sql"

@test "${LIBRARY_NAME} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
