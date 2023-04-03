module github.com/open-telemetry/opentelemetry-go-instrumentation

go 1.18

require (
	github.com/cilium/ebpf v0.10.0
	github.com/go-logr/logr v1.2.4
	github.com/go-logr/zapr v1.2.2
	github.com/hashicorp/go-version v1.6.0
	github.com/prometheus/procfs v0.8.0
	go.opentelemetry.io/otel v1.14.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.14.0
	go.opentelemetry.io/otel/sdk v1.14.0
	go.opentelemetry.io/otel/trace v1.14.0
	go.uber.org/zap v1.24.0
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670
	golang.org/x/sys v0.6.0
	google.golang.org/grpc v1.54.0
)

require (
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.14.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
