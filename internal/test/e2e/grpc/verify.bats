#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/google.golang.org/grpc"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: emits a span name 'SayHello'" {
  result=$(server_span_names_for ${SCOPE} | uniq)
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "server :: includes rpc.system attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.system\").value.stringValue" | uniq)
  assert_equal "$result" '"grpc"'
}

@test "server :: includes rpc.service attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.service\").value.stringValue" | uniq)
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "server :: includes server.address attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"server.address\").value.stringValue" | uniq)
  assert_equal "$result" '"127.0.0.1"'
}

@test "server :: includes server.port attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"server.port\").value.intValue" | uniq)
  assert_equal "$result" '"1701"'
}

@test "server :: trace ID present and valid in all spans" {
  trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "server :: span ID present and valid in all spans" {
  span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "server :: parent span ID present and valid in all spans" {
  parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
  parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "server :: error code is present for unsuccessful span" {
  # gRPC error code 12 - Unimplemented
  result=$(server_span_attributes_for ${SCOPE} | jq 'select(.key == "rpc.grpc.status_code" and .value.intValue == "12")')
  assert_not_empty "$result"
}

@test "client, server, OTel :: spans have same trace ID" {
  # only check the first and 2nd client span (the 3rd is an error)
  otel_trace_id=$( \
	  spans_from_scope_named "go.opentelemetry.io/auto/internal/test/e2e/grpc" \
	  | jq 'select(.name == "SayHello")' \
	  | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0] \
  )
  client_trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0])
  server_trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_equal "$otel_trace_id" "$server_trace_id"
  assert_equal "$server_trace_id" "$client_trace_id"

  otel_trace_id=$( \
	  spans_from_scope_named "go.opentelemetry.io/auto/internal/test/e2e/grpc" \
	  | jq 'select(.name == "SayHello")' \
	  | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1] \
  )
  client_trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1])
  server_trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_equal "$otel_trace_id" "$server_trace_id"
  assert_equal "$server_trace_id" "$client_trace_id"
}

@test "client, server, OTel :: parent span ID" {
  otel_parent_span_id=$( \
	  spans_from_scope_named "go.opentelemetry.io/auto/internal/test/e2e/grpc" \
	  | jq 'select(.name == "SayHello")' \
	  | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[0] \
  )
  server_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".spanId"| jq -Rn '[inputs]' | jq -r .[0])
  assert_equal "$server_span_id" "$otel_parent_span_id"

  server_parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[0])
  # only check the first and 2nd client span (the 3rd is an error)
  client_span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId"| jq -Rn '[inputs]' | jq -r .[0])
  assert_equal "$client_span_id" "$server_parent_span_id"

  otel_parent_span_id=$( \
	  spans_from_scope_named "go.opentelemetry.io/auto/internal/test/e2e/grpc" \
	  | jq 'select(.name == "SayHello")' \
	  | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[1] \
  )
  server_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".spanId"| jq -Rn '[inputs]' | jq -r .[1])
  assert_equal "$server_span_id" "$otel_parent_span_id"

  server_parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId" | jq -Rn '[inputs]' | jq -r .[1])
  # only check the first and 2nd client span (the 3rd is an error)
  client_span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId"| jq -Rn '[inputs]' | jq -r .[1])
  assert_equal "$client_span_id" "$server_parent_span_id"
}

@test "client :: emits a span name 'SayHello'" {
  result=$(client_span_names_for ${SCOPE} | uniq)
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "client :: includes rpc.system attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.system\").value.stringValue" | uniq)
  assert_equal "$result" '"grpc"'
}

@test "client :: includes rpc.service attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.service\").value.stringValue" | uniq)
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "client :: includes server.port attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"server.port\").value.intValue" | uniq)
  assert_equal "$result" '"1701"'
}

@test "client :: trace ID present and valid in all spans" {
  trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
  trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "client :: span ID present and valid in all spans" {
  span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[0])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
  span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId" | jq -Rn '[inputs]' | jq -r .[1])
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "client :: error code is present" {
  result=$(client_span_attributes_for ${SCOPE} | jq -r "select(.key == \"rpc.grpc.status_code\").value.intValue" | sort -u)
  # gRPC error code 0 - OK
  # gRPC error code 12 - Unimplemented
  # gRPC error code 14 - Unavailable

  expected_result=$(printf "%s\n" "0" "12" "14")
  assert_equal "$result" "$expected_result"
}

@test "client :: includes status message" {
  result=$(client_spans_from_scope_named ${SCOPE} | jq -r '
    select(.status.message != null)
    | .status.message
  ' | sort -u)
  
  expected_result=$(printf "%s\n" \
    "connection error: desc = \"transport: Error while dialing: dial tcp [::1]:1701: connect: connection refused\"" \
    "unimplmented")

  assert_equal "$result" "$expected_result"
}
