# Example of Auto instrumentation of HTTP server + Redis

This example only test [go-redis/v8](https://pkg.go.dev/github.com/go-redis/redis/v8).

**It is highly recommended to deploy the demo using docker compose**.


## Docker compose

Setup the example:

```
docker compose up
```

Add a key-value pair to redis using below command:

```
curl -X POST http://localhost:8080/set -d '{"key":"name", "value":"Alice"}'
```

Every hit to the server should generate a trace that we can observe in [Jaeger UI](http://localhost:16686/).


## Local deployment

### Setup OpenTelemetry Collector and Jaeger

You can setup a local [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector) and start it.

Assuming you've exposed port `4318`, and configured the [Jaeger](http://jaegertracing.io/docs) backend service in collector.


### Setup auto-instrumentation binary

Build the binary

```
make build
```

You will get binary `otel-go-instrumentation` in current directory. Then start instrumenting the target app

```
sudo OTEL_GO_AUTO_TARGET_EXE=</path/to/executable_app> OTEL_SERVICE_NAME=eBPFApp OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 OTEL_GO_AUTO_INCLUDE_DB_STATEMENT=true ./otel-go-instrumentation
```

### Setup and run the demo
Build and run

```
go build -o main

./main
```

Set

```
curl -X POST http://localhost:8080/set -d '{"key":"name", "value":"Alice"}'
```

Get

```
curl -X POST http://localhost:8080/get -d '{"key":"name"}'
```

Sadd

```
curl -X POST http://localhost:8080/sadd -d '{"key":"mySet", "values":["val1", "val2", "val3", "val4"]}'
```

You can observe the trace in [Jaeger UI](http://localhost:16686/).
