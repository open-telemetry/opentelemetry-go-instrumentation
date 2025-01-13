module go.opentelemetry.io/auto/internal/test/e2e/autosdk

go 1.22.0

require (
	go.opentelemetry.io/auto/sdk v1.1.1-0.20250113173622-5ccf0c1ed7ba
	go.opentelemetry.io/otel v1.33.1-0.20250113154543-79b1fc1b9dc3
	go.opentelemetry.io/otel/trace v1.33.1-0.20250113154543-79b1fc1b9dc3
)

replace go.opentelemetry.io/auto/sdk => ../../../../sdk/
