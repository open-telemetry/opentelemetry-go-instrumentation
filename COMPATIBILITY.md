# Compatibility

## Default `otel` Global Providers Compatibility

Auto-instrumentation can be configured to capture the telemetry sent to the
[`otel`] default global tracer provider.

Supported versions of [`otel`]:

- `v0.14.0` to `v1.33.0`

[`otel`]: https://pkg.go.dev/go.opentelemetry.io/otel

## Instrumented Library Compatibility

Tracing instrumentation is provided for the following Go libraries.

- [`database/sql`](#databasesql)
- [`github.com/segmentio/kafka-go`](#githubcomsegmentiokafka-go)
- [`google.golang.org/grpc`](#googlegolangorggrpc)
- [`net/http`](#nethttp)
- [`github.com/go-redis/redis/v8`](#githubcomgo-redisredisv8)

### database/sql

[Package documentation](https://pkg.go.dev/database/sql)

Supported version ranges:

- `go1.12` to `go1.23.4`

### github.com/segmentio/kafka-go

[Package documentation](https://pkg.go.dev/github.com/segmentio/kafka-go)

Supported version ranges:

- `v0.4.1` to `v0.4.47`

### google.golang.org/grpc

[Package documentation](https://pkg.go.dev/google.golang.org/grpc)

Supported version ranges:

- `v1.14.0` to `v1.69.0`

### net/http

[Package documentation](https://pkg.go.dev/net/http)

Supported version ranges:

- `go1.12` to `go1.23.4`

### github.com/go-redis/redis/v8

[Package documentation](https://pkg.go.dev/github.com/go-redis/redis/v8)

- `v8.0.0` to `v8.11.5`