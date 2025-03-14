module go.opentelemetry.io/auto/internal/test/e2e/grpc

go 1.23.0

require (
	go.opentelemetry.io/otel v1.35.0
	google.golang.org/grpc v1.71.0
	google.golang.org/grpc/examples v0.0.0-20250313062406-775150f68c17
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250311190419-81fb87f6b8bf // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

replace go.opentelemetry.io/auto/sdk => ../../../../sdk
