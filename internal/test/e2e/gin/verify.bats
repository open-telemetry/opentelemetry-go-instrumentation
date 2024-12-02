#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/github.com/gin-gonic/gin"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: includes http.request.method attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.request.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "server :: includes url.path attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"url.path\").value.stringValue")
  assert_equal "$result" '"/hello-gin"'
}

@test "server :: includes http.route attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.route\").value.stringValue")
  assert_equal "$result" '"/hello-gin/:id"'
}
