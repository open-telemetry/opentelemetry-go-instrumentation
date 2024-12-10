module go.opentelemetry.io/auto/internal/test/e2e/autosdk

go 1.22.0

require (
	go.opentelemetry.io/auto/sdk v1.1.0
	go.opentelemetry.io/otel v1.32.0
	go.opentelemetry.io/otel/trace v1.32.0
)

replace go.opentelemetry.io/auto/sdk => ../../../../sdk/
