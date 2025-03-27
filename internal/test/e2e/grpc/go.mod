module go.opentelemetry.io/auto/internal/test/e2e/grpc

go 1.23.0

require (
	go.opentelemetry.io/otel v1.35.0
	google.golang.org/grpc v1.71.0
	google.golang.org/grpc/examples v0.0.0-20250326170016-6819ed796fcd
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace go.opentelemetry.io/auto/sdk => ../../../../sdk
