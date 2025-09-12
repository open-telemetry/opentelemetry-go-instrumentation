module go.opentelemetry.io/auto/examples/kafka-go/consumer

go 1.24.0

require (
	github.com/segmentio/kafka-go v0.4.49
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
)

require (
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	go.opentelemetry.io/auto/sdk v1.2.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
)

replace go.opentelemetry.io/auto/sdk => ../../../sdk
