module go.opentelemetry.io/auto/examples/rolldice/user

go 1.24.0

require (
	github.com/mattn/go-sqlite3 v1.14.33
	go.opentelemetry.io/otel v1.39.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
)

replace go.opentelemetry.io/auto/sdk => ../../../sdk
