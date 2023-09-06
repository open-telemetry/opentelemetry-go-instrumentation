# DATA RETRIEVERS

# Returns a list of span names emitted by a given library/scope
	# $1 - library/scope name
span_names_for() {
	spans_from_scope_named $1 | jq '.name'
}

# Returns a list of attributes emitted by a given library/scope
span_attributes_for() {
	# $1 - library/scope name

	spans_from_scope_named $1 | \
		jq ".attributes[]"
}

# Returns a list of all resource attributes
resource_attributes_received() {
	spans_received | jq ".resource.attributes[]?"
}

# Returns an array of all spans emitted by a given library/scope
	# $1 - library/scope name
spans_from_scope_named() {
	spans_received | jq ".scopeSpans[] | select(.scope.name == \"$1\").spans[]"
}

# Returns an array of all spans received
spans_received() {
	json_output | jq ".resourceSpans[]?"
}

# Returns the content of the log file produced by a collector
# and located in the same directory as the BATS test file
# loading this helper script.
json_output() {
	cat "${BATS_TEST_DIRNAME}/traces-orig.json"
}

redact_json() {
	json_output | \
		jq --sort-keys '
			del(
				.resourceSpans[].scopeSpans[].spans[].startTimeUnixNano,
				.resourceSpans[].scopeSpans[].spans[].endTimeUnixNano
			)
			| .resourceSpans[].scopeSpans[].spans[].traceId|= (if
					. // "" | test("^[A-Fa-f0-9]{32}$") then "xxxxx" else (. + "<-INVALID")
				end)
			| .resourceSpans[].scopeSpans[].spans[].spanId|= (if
					. // "" | test("^[A-Fa-f0-9]{16}$") then "xxxxx" else (. + "<-INVALID")
				end)
			| .resourceSpans[].scopeSpans[].spans[].parentSpanId|= (if
					. // "" | test("^[A-Fa-f0-9]{16}$") then "xxxxx" else (. + "")
				end)
			| .resourceSpans[].scopeSpans|=sort_by(.scope.name)
			' > ${BATS_TEST_DIRNAME}/traces.json
}

# ASSERTION HELPERS

# expect a 32-digit hexadecimal string (in quotes)
MATCH_A_TRACE_ID=^"\"[A-Fa-f0-9]{32}\"$"

# expect a 16-digit hexadecimal string (in quotes)
MATCH_A_SPAN_ID=^"\"[A-Fa-f0-9]{16}\"$"

# Fail and display details if the expected and actual values do not
# equal. Details include both values.
#
# Inspired by bats-assert * bats-support, but dramatically simplified
assert_equal() {
	if [[ $1 != "$2" ]]; then
		{
			echo
			echo "-- 💥 values are not equal 💥 --"
			echo "expected : $2"
			echo "actual   : $1"
			echo "--"
			echo
		} >&2 # output error to STDERR
		return 1
	fi
}

assert_regex() {
	if ! [[ $1 =~ $2 ]]; then
		{
			echo
			echo "-- 💥 value does not match regular expression 💥 --"
			echo "value   : $1"
			echo "pattern : $2"
			echo "--"
			echo
		} >&2 # output error to STDERR
		return 1
	fi
}

assert_not_empty() {
	EMPTY=(\"\")
	if [[ "$1" == "${EMPTY}" ]]; then
		{
			echo
			echo "-- 💥 value is empty 💥 --"
			echo "value : $1"
			echo "--"
			echo
		} >&2 # output error to STDERR
		return 1
	fi
}
