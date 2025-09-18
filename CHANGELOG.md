# Changelog

All notable changes to OpenTelemetry Go Automatic Instrumentation are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

OpenTelemetry Go Automatic Instrumentation adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Cache offsets for `google.golang.org/grpc` `1.77.0-dev`. ([#2820](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2820))

### Removed

- Build support for [Go 1.23] has been removed.
  Use Go >= 1.24 to develop and build the project. ([#2799](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2799))

## [`go.opentelemetry.io/auto/sdk` v1.2.1] - 2025-09-15

### Fixed

- Fix `uint32` bounding on 32 bit architectures in the `go.opentelemetry.io/auto/sdk` module. ([#2810](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2810))

## [v0.23.0/v1.2.0] - 2025-09-10

<!-- markdownlint-disable MD028 -->
> [!NOTE]
> This is the last release version that will support building the auto-instrumentation CLI using Go 1.23.
> The next release will require development to be done using Go >= 1.24.
<!-- markdownlint-enable MD028 -->

### Added

- Cache offsets for `golang.org/x/net` `0.42.0`. ([#2503](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2503))
- Cache offsets for `google.golang.org/grpc` `1.74.2`. ([#2546](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2546))
- Cache offsets for `google.golang.org/grpc` `1.76.0-dev`. ([#2596](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2596))
- Allow configuration of the resource using a [resource.Detector]. ([#2598](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2598))
  - The `WithResourceDetector` function is added to `go.opentelemetry.io/auto/pipeline/otelsdk`.
  - The `WithEnv` function is updated to parse the `OTEL_RESOURCE_DETECTOR` environment variable.
    Values are expected to be a comma-separated list of resource detector IDs registered with the [`autodetect` package].
- Cache offsets for Go `1.23.12`. ([#2603](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2603))
- Cache offsets for Go `1.24.6`. ([#2603](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2603))
- Cache offsets for `golang.org/x/net` `0.43.0`. ([#2615](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2615))
- Cache offsets for Go `1.25.0`. ([#2651](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2651))
- Cache offsets for `google.golang.org/grpc` `1.75.0`. ([#2686](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2686))
- Cache offsets for `github.com/segmentio/kafka-go` `0.4.49`. ([#2699](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2699))
- Cache offsets for `go.opentelemetry.io/otel` `v1.38.0`. ([#2726](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2726))
- Cache offsets for Go `1.24.7`. ([#2747](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2747))
- Cache offsets for Go `1.25.1`. ([#2747](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2747))
- Cache offsets for `golang.org/x/net` `0.44.0`. ([#2773](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2773))
- Cache offsets for `google.golang.org/grpc` `1.72.3`. ([#2787](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2787))
- Cache offsets for `google.golang.org/grpc` `1.73.1`. ([#2787](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2787))
- Cache offsets for `google.golang.org/grpc` `1.74.3`. ([#2787](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2787))
- Cache offsets for `google.golang.org/grpc` `1.75.1`. ([#2787](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2787))

[resource.Detector]: https://pkg.go.dev/go.opentelemetry.io/otel/sdk/resource#Detector
[`autodetect` package]: https://pkg.go.dev/go.opentelemetry.io/contrib/detectors/autodetect

### Changed

- Upgrade `go.opentelemetry.io/auto` semconv to `v1.37.0`. ([#2763](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2763))
- Upgrade `go.opentelemetry.io/auto/sdk` semconv to `v1.37.0`. ([#2763](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2763))

### Fixed

- Add `telemetry.distro.version` resource attribute to the `otelsdk` handler. ([#2383](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2383))
- `active_spans_by_span_ptr` eBPF map used in the traceglobal probe changed to LRU. ([#2509](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2509))

## [v0.22.1] - 2025-07-01

### Added

- Cache offsets for `google.golang.org/grpc` `1.71.3`. ([#2374](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2374))
- Cache offsets for `google.golang.org/grpc` `1.72.2`. ([#2374](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2374))
- Cache offsets for `golang.org/x/net` `0.41.0`. ([#2402](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2402))
- Cache offsets for `google.golang.org/grpc` `1.73.0`. ([#2402](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2402))
- Cache offsets for Go `1.23.10`. ([#2402](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2402))
- Cache offsets for Go `1.24.4`. ([#2402](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2402))
- Cache offsets for `go.opentelemetry.io/otel` `v1.37.0`. ([#2450](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2450))
- Cache offsets for `google.golang.org/grpc` `1.75.0-dev`. ([#2450](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2450))

### Fixed

- Build go binaries using the provided `TARGETARCH` of the Dockerfile.
  This fixes the bug where images for alternate architectures (e.g. `arm64`) were built using the `amd64` architecture. ([#2411](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2411))
- Do not fail run when a module has a version of `(devel)`. ([#2437](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2437))

## [v0.22.0] - 2025-05-23

### Added

- Cache offsets for `google.golang.org/grpc` `1.72.0-dev`. ([#1849](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1849))
- The `go.opentelemtry.io/auto/pipeline` package is added.
  This package contains interface definitions for types that want to handle the telemetry generated by auto-instrumentation. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `go.opentelemtry.io/auto/pipeline/otelsdk` package is added.
  This package a default handler that uses the OpenTelemetry Go SDK to handle telemetry generated by auto-instrumentation. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `WithHandler` function is added to configure `Instrumentation` in `go.opentelemtry.io/auto` with the desired handler implementation. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The auto binary (built from `auto/cli`) can now be passed the target process PID directly using the `-target-pid` CLI option. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))
- The auto binary (built from `auto/cli`) can now be passed the path of the target process executable directly using the `-target-exe` CLI option. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))
- The auto binary (built from `auto/cli`) now resolves the target PID from the environment variable `"OTEL_GO_AUTO_TARGET_PID"` if no target options are passed. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))
- The auto binary (built from `auto/cli`) will now only resolve the target process executable from the environment variable `"OTEL_GO_AUTO_TARGET_EXE"` if no target options are passed and `"OTEL_GO_AUTO_TARGET_PID"` is not set. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))
- Cache offsets for `golang.org/x/net` `0.36.0`. ([#1940](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1940))
- Cache offsets for `google.golang.org/grpc` `1.71.0`. ([#1940](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1940))
- Cache offsets for Go `1.23.7`. ([#1940](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1940))
- Cache offsets for Go `1.24.1`. ([#1940](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1940))
- Cache offsets for `go.opentelemetry.io/otel` `v1.35.0`. ([#1948](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1948))
- Cache offsets for `golang.org/x/net` `0.37.0`. ([#1948](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1948))
- Cache offsets for `golang.org/x/net` `0.38.0`. ([#2063](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2063))
- Cache offsets for `google.golang.org/grpc` `1.71.1`. ([#2078](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2078))
- Cache offsets for Go `1.23.8`. ([#2081](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2081))
- Cache offsets for Go `1.24.2`. ([#2081](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2081))
- Cache offsets for `google.golang.org/grpc` `1.73.0-dev`. ([#2091](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2091))
- Cache offsets for `golang.org/x/net` `0.39.0`. ([#2107](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2107))
- The new `Multiplexer` type is added to `go.opentelemetry.io/auto/pipeline/otelsdk`.
  This type is used to support multiple process instrumentation using the same telemetry pipeline. ([#2016](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2016))
- Cache offsets for `google.golang.org/grpc` `1.72.0`. ([#2190](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2190))
- Cache offsets for `golang.org/x/net` `0.40.0`. ([#2281](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2281))
- Cache offsets for Go `1.23.9`. ([#2292](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2292))
- Cache offsets for Go `1.24.3`. ([#2292](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2292))
- Cache offsets for `github.com/segmentio/kafka-go` `0.4.48`. ([#2319](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2319))
- Cache offsets for `google.golang.org/grpc` `1.71.2`. ([#2319](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2319))
- Cache offsets for `google.golang.org/grpc` `1.72.1`. ([#2319](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2319))
- Cache offsets for `google.golang.org/grpc` `1.74.0-dev`. ([#2337](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2337))
- Cache offsets for `go.opentelemetry.io/otel` `v1.36.0`. ([#2352](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2352))

### Changed

- The `WithEnv` function no longer parses `OTEL_GO_AUTO_GLOBAL`.
  This is included by default. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `WithEnv` function no longer parses `OTEL_SERVICE_NAME` or `OTEL_TRACES_EXPORTER`.
  Use the `Handler` from `go.opentelemtry.io/auto/pipeline/otelsdk` with its own `WithEnv` to replace functionality. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- Instrument spans created with the OpenTelemetry trace API from an empty context. ([#2001](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2001))
- Upgrade OpenTelemetry semantic conventions to `v1.30.0`. ([#2032](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2032))
- Modify how the pattern is fetch from `net/http.Request`.
  Now it uses `Request.Pattern` instead of `Request.pat.str` unless using go1.22, which continue using `Request.pat.str`. ([#2090](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2090))

### Removed

- Build support for Go 1.22 has been removed.
  Use Go >= 1.23 to develop and build the project. ([#1841](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1841))
- The `WithGlobal` function is removed from `go.opentelemtry.io/auto`.
  This option is on by default. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `WithServiceName` function is removed from `go.opentelemtry.io/auto`.
  Use `WithServiceName` in `go.opentelemtry.io/auto/pipeline/otelsdk` along with `WithHandler` to replace functionality. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `WithTraceExporter` function is removed from `go.opentelemtry.io/auto`.
  Use `WithTraceExporter` in `go.opentelemtry.io/auto/pipeline/otelsdk` along with `WithHandler` to replace functionality. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- The `WithResourceAttributes` function is removed from `go.opentelemtry.io/auto`.
  Use `WithResourceAttributes` in `go.opentelemtry.io/auto/pipeline/otelsdk` along with `WithHandler` to replace functionality. ([#1859](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1859))
- Resolution of the environment variable `"OTEL_GO_AUTO_TARGET_EXE"` has been removed from `WithEnv`.
  Note, the built binary (`auto/cli`) still supports resolution and use of this value.
  If using the `auto` package directly, you will need to resolve this value yourself and pass the discovered process PID using `WithPID`. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))
- The `WithTarget` function is removed.
  The `auto` package no longer supports process discovery (note: the built binary (`auto/cli`) still supports process discovery).
  Once a target process has been identified, use `WithPID` to configure `Instrumentation` instead. ([#1890](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1890))

### Fixed

- Fix spans parsing from eBPF for the legacy (go version < 1.24 otel-go < 1.33) otel global instrumentation. ([#1960](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1960))
- The `process.runtime.version` resource attribute is now the exact value returned from `debug` to match what OpenTelemetry semantic conventions recommend. ([#1985](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1985))
- Stop adding `process.runtime.description` to `Resource` to follow OpenTelemetry semantic conventions. ([#1986](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1986))
- Reset Kafka producer span underlying memory before each span. ([#1937](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1937))
- Stop pinning collector image in e2e tests. ([#2072](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2072))
- Fallback to avoid context propagation in `kafka-go` instrumentation if the kernel does not support `bpf_probe_write_user`. ([#2105](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2105))
- Make sure Go strings being read from eBPF are null terminated. ([#1936](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1936))
- Handle dynamic goroutine stack resizes in the `autosdk` and `otel/trace` probes. ([#2263](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/2263))

## [v0.21.0] - 2025-02-18

<!-- markdownlint-disable MD028 -->
> [!WARNING]  
> The `net/http` instrumentation support for versions `< 1.19` has been dropped.

> [!WARNING]  
> The `database/sql` instrumentation support for versions `< 1.19` has been dropped.

> [!NOTE]  
> This is the last release version that will support building the auto-instrumentation CLI using Go 1.22.
> The next release will require development to be done using Go >= 1.23.
<!-- markdownlint-enable MD028 -->

### Added

- Update instrumentation for `net/http` to support Go `1.24` SwissMap. ([#1636](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1636))
- Cache offsets for Go `1.22.12`. ([#1743](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1743))
- Cache offsets for Go `1.23.6`. ([#1743](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1743))
- Cache offsets for `golang.org/x/net` `0.35.0`. ([#1783](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1783))
- Cache offsets for Go `1.24.0`. ([#1795](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1795), [#1798](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1798))

### Changed

- Unused support for instrumentation of Go < 1.19 has been dropped. ([#1815](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1815))

## [v0.20.0] - 2025-01-29

### Added

- Support `SELECT`, `INSERT`, `UPDATE`, and `DELETE` for database span names and `db.operation.name` attribute. ([#1253](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1253))
- Support the full tracing API with the `otelglobal` probe. ([#1405](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1405))
- Support `go.opentelemetry.io/otel@v1.33.0`. ([#1417](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1417))
- Support `google.golang.org/grpc` `1.69.0`. ([#1417](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1417))
- Update `google.golang.org/grpc` probe to work with versions `>= 1.69.0`. ([#1431](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1431))
- Support `google.golang.org/grpc` `1.67.3`. ([#1452](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1452))
- Support `google.golang.org/grpc` `1.68.2`. ([#1462](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1462))
- Support `google.golang.org/grpc` `1.69.2`. ([#1467](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1467))
- Support `google.golang.org/grpc` `1.71.0-dev`. ([#1467](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1467))
- Support `golang.org/x/net` `0.33.0`. ([#1471](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1471))
- Use `OTEL_GO_AUTO_PARSE_DB_STATEMENT` environment variable in the `httpPlusdb` demo. ([#1523](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1523))
- Include gRPC error message for client spans. ([#1528](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1528))
- Support `golang.org/x/net` `0.34.0`. ([#1552](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1552))
- Support `google.golang.org/grpc` `1.69.4`. ([#1590](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1590))
- Support `go.opentelemetry.io/otel@v1.34.0`. ([#1638](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1638))
- Support Go `1.22.11`. ([#1638](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1638))
- Support Go `1.23.5`. ([#1638](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1638))
- Support `google.golang.org/grpc` `1.70.0`. ([#1682](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1682))

### Changed

- Update the `rolldice` example to better show the functionality of the project. ([#1566](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1566))
- Preemptively drop support for the `traceglobal` probe when `Go >= 1.24` is used. ([#1573](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1573))
- Support non-cached offsets.
  If the target process uses an unknown version of an instrumented package but has DAWRF data included, the offset is now found on startup instead of returning an error. ([#1593](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1593))

### Fixed

- Respect `OTEL_EXPORTER_OTLP_PROTOCOL` when `OTEL_TRACES_EXPORTER` is not set. ([#1572](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1572))
- Support stripped binaries, including those built with CGO libraries. ([#1641](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1641))

## [v0.19.0-alpha] - 2024-12-05

### Added

- Support span attribute limits to `go.opentelemtry.io/auto/sdk`. ([#1315](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1315))
- Support span link limits to `go.opentelemtry.io/auto/sdk`. ([#1320](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1320))
- Support span event limits to `go.opentelemtry.io/auto/sdk`. ([#1324](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1324))
- Support attribute value limits to `go.opentelemtry.io/auto/sdk`. ([#1325](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1325))
- Support Go `1.22.10`. ([#1367](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1367))
- Support Go `1.23.4`. ([#1367](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1367))
- Support `golang.org/x/net` `0.32.0`. ([#1382](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1382))
- Support `google.golang.org/grpc` `1.67.2`. ([#1382](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1382))
- Support `google.golang.org/grpc` `1.68.1`. ([#1382](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1382))
- Support `google.golang.org/grpc` `1.70.0-dev`. ([#1382](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1382))

### Fixed

- The parsing of the tracers map for `go.opentelemetry.io/otel@v1.32.0` is fixed. ([#1319](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1319))

## [v0.18.0-alpha] - 2024-11-20

### Changed

- Split the functionality of `Instrumentation.Run` to `Instrumentation.Load` and `Instrumentation.Run`.
  `Load` will report any errors in the loading and attaching phase of the probes. ([#1245](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1245))

### Added

- Include `server.address` and `server.port` in gRPC spans (>=v1.60.0). ([#1242](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1242))
- Support Go standard libraries for 1.22.9 and 1.23.3. ([#1250](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1250))
- Support `google.golang.org/grpc` `1.68.0`. ([#1251](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1251))
- Support `golang.org/x/net` `0.31.0`. ([#1254](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1254))
- Support `go.opentelemetry.io/otel@v1.32.0`. ([#1302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1302))

### Fixed

- Don't include `db.query.text` attribute in `database/sql` if the query string is empty or not collected. ([#1246](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1246))
- Handle a `ConfigProvider` which doesn't provide a sampling config in the initial configuration by applying the default sampler. ([#1292](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1292))

### Removed

- The deprecated `go.opentelemetry.io/auto/sdk/telemetry` package is removed. ([#1252](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1252))
- The deprecated `go.opentelemetry.io/auto/sdk/telemetry/test` module is removed. ([#1252](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1252))
- Remove the `WithLoadedIndicator` `InstrumentationOption` since the `Instrumentation.Load` will indicate whether the probes are loaded in a synchronous way. ([#1245](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1245))

## [v0.17.0-alpha] - 2024-11-05

### Changed

- The SDK provided in `go.opentelemtry.io/auto/sdk` now defaults to No-Op behavior for unimplemented methods of the OpenTelemetry API.
  This is changed from causing a compilation error for unimplemented methods. ([#1230](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1230))
- The `GetTracerProvider` function in `go.opentelemtry.io/auto/sdk` is renamed to `TracerProvider`. ([#1231](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1231))

### Fixed

- Sporadic shutdown deadlock. ([#1220](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1220))
- Only support status codes for versions of `google.golang.org/grpc` >= `1.40`. ([#1235](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1235))

### Deprecated

- The `go.opentelemetry.io/auto/sdk/telemetry` package is deprecated.
  This package will be removed in the next release. ([#1238](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1238))
- The `go.opentelemetry.io/auto/sdk/telemetry/test` module is deprecated.
  This module will be removed in the next release. ([#1238](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1238))

## [v0.16.0-alpha] - 2024-10-22

### Added

- Support `golang.org/x/net` `v0.30.0`. ([#1149](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1149))
- Support `google.golang.org/grpc` `1.65.1`. ([#1174](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1174))
- Support `go.opentelemetry.io/otel@v1.31.0`. ([#1178](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1178))
- Support `google.golang.org/grpc` `1.69.0-dev`. ([#1203](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1203))
- Implement traceID ratio and parent-based samplers. ([#1150](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1150))
- The `go.opentelemetry.io/auto/sdk` module.
  This module is used directly when you want to explicilty use auto-instrumentation to process OTel API telemetry.
  It is also provided so the default OTel global API will use this when auto-instrumentation is loaded (WIP). ([#1210](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1210))

### Fixed

- The `"golang.org/x/net/http2".FrameHeader.StreamID` offset for version `0.8.0` is corrected. ([#1208](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1208))
- The `"golang.org/x/net/http2".MetaHeadersFrame.Fields` offset for version `0.8.0` is corrected. ([#1208](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1208))

## [v0.15.0-alpha] - 2024-10-01

### Added

- Support Go `v1.21.13`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support Go `v1.22.6`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support `golang.org/x/net` `v0.28.0`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support `google.golang.org/grpc` `1.67.0-dev`. ([#1007](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1007))
- Support Go `1.23.0`. ([#1007](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1007))
- Introduce `config.Provider` as an option to set the initial configuration and update it in runtime. ([#1010](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1010))
- Support `go.opentelemetry.io/otel@v1.29.0`. ([#1032](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1032))
- Support `google.golang.org/grpc` `1.66.0`. ([#1046](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1046))
- `Sampler` interface that can be passed to `Instrumentation` via the new `WithSampler` option.
  This is wireframe configuration, it currently has not effect.
  It will be used to allows customization of what sampler is used by the `Instrumentation`.
  Implementation, of this feature is expected in the next release. ([#982](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/982))
- The `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG` environment variables are now supported when the `WithEnv` option is used. ([#982](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/982))
- Support `golang.org/x/net` `v0.29.0`. ([#1051](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1051))
- Support Go `1.22.7`. ([#1051](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1051))
- Support Go `1.23.1`. ([#1051](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1051))
- Log version information in the CLI. ([#1077](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1077))
- Support `google.golang.org/grpc` `1.66.1`. ([#1078](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1078))
- Add gRPC status code attribute for client spans (`rpc.grpc.status_code`). ([#1044](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1044))
- Support `google.golang.org/grpc` `1.68.0-dev`. ([#1044](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1044))
- Support `go.opentelemetry.io/otel@v1.30.0`. ([#1044](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1044))
- The `WithLogger` `InstrumentationOption` is added as a replacement for `WithLogLevel`.
  An `slog.Logger` can now be configured by the user any way they want and then passed to the `Instrumentation` for its logging with this option. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- Support `google.golang.org/grpc` `1.66.2`. ([#1083](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1083))
- Support `google.golang.org/grpc` `1.67.0`. ([#1116](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1116))
- Support `google.golang.org/grpc` `1.66.3`. ([#1143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1143))
- Support `google.golang.org/grpc` `1.67.1`. ([#1143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1143))
- Support Go `1.22.8`. ([#1143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1143))
- Support Go `1.23.2`. ([#1143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1143))
- Add gRPC status code attribute for server spans (`rpc.grpc.status_code`). ([#1127](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1127))

### Changed

- The `WithSampler` option function now accepts the new `Sampler` interface instead of `trace.Sampler`. ([#982](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/982))

### Fixed

- Fix dirty shutdown caused by panic. ([#980](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/980))
- Flush pending span exports on shutdown. ([#1028](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1028))

### Removed

- `WithLogLevel` is removed.
  Use `WithLogger` instead. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `LogLevelDebug` constant is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `LogLevelInfo` constant is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `LogLevelWarn` constant is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `LogLevelError` constant is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `LogLevel` type is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- The unused `ParseLogLevel` function is removed. ([#1080](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1080))
- Drop agent build support for Go 1.21. ([#1115](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1115))

## [v0.14.0-alpha] - 2024-07-15

### Added

- Add support to log level through command line flag. ([#842](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/842))
- The `WithLogLevel` function and `LogLevel` type are added to set the log level for `Instrumentation`. ([#842](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/842))
- The `otelglobal` probe now collects the user provided tracer name, version and schemaURL. ([#844](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/844))
- Add `codespell` make target. ([#863](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/863))
- Initial support for `trace-flags`. ([#868](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/868))
- Support `google.golang.org/grpc` `1.66.0-dev`. ([#872](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/872))
- Add telemetry distro name & version resource attributes. ([#897](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/897))
- Support `google.golang.org/grpc` `1.65.0`. ([#904](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/904))
- Support Go `v1.21.12`. ([#905](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/905))
- Support Go `v1.22.5`. ([#905](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/905))
- Support `go.opentelemetry.io/otel@v1.28.0`. ([#905](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/905))
- Support `google.golang.org/grpc` `1.63.3`. ([#916](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/916))
- Support `google.golang.org/grpc` `1.64.1`. ([#916](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/916))
- Support `golang.org/x/net` `v0.27.0`. ([#917](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/917))

### Changed

- Upgrade semconv from `v1.24.0` to `v1.26.0` in `github.com/segmentio/kafka-go` instrumentation. ([#909](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/909))
  - The `messaging.operation` attribute key is renamed to `messaging.operation.type`.
  - The `messaging.kafka.destination.partition` key is renamed to `messaging.destination.partition.id`
- Upgrade semconv from `v1.21.0` to `v1.26.0` in `database/sql` instrumentation. ([#911](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/911))
  - The `db.statement` attribute key is renamed to `db.query.text`.
- Upgrade semconv from `v1.24.0` to `v1.26.0` in `google.golang.org/grpc` instrumentation. ([#912](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/912))
  - The `net.peer.name` attribute key in client instrumentation is renamed to `server.address`.
- Upgrade semconv to `v1.26.0` in `net/http` instrumentation. ([#913](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/913))
- Upgrade `go.opentelemetry.io/auto` semconv to `v1.26.0`. ([#914](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/914))

### Fixed

- The HTTP client now uses the `Host` field from the URL if the `Request.Host` is not present. ([#888](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/888))

## [v0.13.0-alpha] - 2024-06-04

### Added

- `github.com/segmentio/kafka-go` instrumentation. ([#709](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/709))
- Support `go.opentelemetry.io/otel@v1.26.0`. ([#796](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/796))
- Support HTTP server path template added in Go version 1.22.
- The `http.route` attribute is included and the span name updated to use this information. ([#740](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/740))
- Support `golang.org/x/net` v0.25.0. ([#821](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/821))
- Support Go `v1.21.10`. ([#824](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/824))
- Support Go `v1.22.3`. ([#824](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/824))
- Support `google.golang.org/grpc` `1.65.0-dev`. ([#827](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/827))
- Support `google.golang.org/grpc` `1.64.0`. ([#843](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/843))
- `WithLoadedIndicator` `InstrumentationOption` to configure an Instrumentation to notify the caller once all the eBPF probes are loaded. ([#848](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/848))
- Add env var equivalent to the WithGlobal InstrumentationOption. ([#849](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/849))
- Support `go.opentelemetry.io/otel@v1.27.0`. ([#850](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/850))
- Support `golang.org/x/net` v0.26.0. ([#871](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/871))
- Support Go `v1.21.11`. ([#871](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/871))
- Support Go `v1.22.4`. ([#871](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/871))

### Fixed

- Change HTTP client span name to `{http.request.method}`. ([#775](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/775))
- Don't set empty URL path in HTTP client probe. ([#810](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/810))
- Don't fail HTTP client probe attribute resolution on empty URL path. ([#810](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/810))
- Extract `process.runtime.version` and `process.runtime.name` from instrumented process. ([#811](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/811))
- Support Go versions from apps defining GOEXPERIMENT. ([#813](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/813))
- Update `net/http` instrumentation to comply with semantic conventions v1.25.0. ([#790](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/790))
- Update existing 3rd party licenses. ([#864](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/864))

## [v0.12.0-alpha] - 2024-04-10

### Added

- Support `golang.org/x/net/http2@v0.23.0`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support `google.golang.org/grpc@v1.61.2`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support `google.golang.org/grpc@v1.62.2`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support `google.golang.org/grpc@v1.63.0`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support `google.golang.org/grpc@v1.63.1`. ([#761](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/761))
- Support `google.golang.org/grpc@v1.63.2`. ([#761](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/761))
- Support Go `v1.21.9`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support Go `v1.22.2`. ([#744](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/744))
- Support `golang.org/x/net/http2@v0.24.0`. ([#746](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/746))
- Support `go.opentelemetry.io/otel@v1.25.0`. ([#748](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/748))
- Update project Go version used to build to 1.21 ([#747](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/747))

### Fixed

- Handle Ctrl-C signal while searching for the target PID ([#731](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/731))
- Pass PID to `UprobeOptions` ([#742](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/742))
- Remove Gin duplicate probe ([#780](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/780))

## [v0.11.0-alpha] - 2024-03-26

### Added

- Test build using [Go 1.22]. (#672)
- Base Dockerfile and build caching ([#683](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/683))
- Add `server.address`, `server.port` and `network.protocol.version` to HTTP client spans ([#696](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/696))
- Update http server attributes to latest semantic conventions ([#708](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/708))
- Don't use external memory for http client header injection ([#705](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/705))

### Fixed

- Don't call `manager.Close()` when `Analyze()` fails. ([#638](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/638))
- Close `proc` file when done discovering PID. ([#649](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/649))
- Use `debug` packages to parse Go and modules' versions. ([#653](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/653))
- Clean up warn in otelglobal `SetStatus()` when grabbing the status code. ([#675](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/675))
- Reset `proc` offset after a failed iteration. ([#681](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/681))
- Avoid using runtime.NumCPU to get the number of CPUs on the system before remote mmap ([#680](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/680))
- Cleanup eBPF maps only when we stop using the memory ([#682](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/682))
- Fix start offset calculation in mmaped memory area ([#738](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/738))

## [v0.10.1-alpha] - 2024-01-10

### Added

- Support version `v0.20.0` of `golang.org/x/net`. ([#587](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/587))
- Support version `v1.20.13` and `v1.21.6` of Go. ([#589](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/589))
- Add support for manual instrumentation with Span `SetName`. ([#590](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/590))
- Add support for manual instrumentation with Span `SetStatus` ([#591](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/591)])

### Fixed

- Log any failures to close running probes. ([#586](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/586))
- Log explanatory error message on Linux kernel lockdown ([#290](https://github.com/open-telemetry/opentelemetry-go-instrumentation/issues/290))
- (otelglobal) Fixed case where multiple span.SetAttributes() calls would overwrite existing attributes ([#634](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/634))

## [v0.10.0-alpha] - 2024-01-03

### Added

- Add `net.host.name`, `net.protocol.name`, `net.peer.name`, and `net.peer.port` attributes to HTTP server spans. ([#470](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/470)
- Support version `v1.60.1` of `google.golang.org/grpc`. ([#568](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/568))

### Fixed

- Correct the target probe argument positions for the `v1.60.0` and greater versions of the `google.golang.org/grpc` server instrumentation. ([#574](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/574), [#576](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/576))
- Do not instrument the OpenTelemetry default global implementation if the user has already set a delegate. ([#569](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/569))

## [v0.9.0-alpha] - 2023-12-14

### Added

- The CLI flag `global-impl` is added.
  This flag, when used, enables the instrumentation of the OpenTelemetry default global implementation (<https://pkg.go.dev/go.opentelemetry.io/otel>).
  This means that all trace telemetry from this implementation that would normally be dropped will instead be recorded with the auto-instrumentation pipeline. ([#523](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/523))
- Add `WithResourceAttributes` `InstrumentationOption` to configure `Instrumentation` to add additional resource attributes. ([#522](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/522))
- Support versions `v0.18.0` and `v0.19.0` of `golang.org/x/net`. ([#524](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/524))
- Add the status code to HTTP client instrumentation. ([#527](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/527))
- Support versions `v1.20.12`, `v1.21.4`, and `v1.21.5` of Go standard library. ([#535](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/535))
- Support version `v1.60.0` of `google.golang.org/grpc`. ([#555](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/555))

### Changed

- The instrumentation scope name for the `database/sql` instrumentation is now `go.opentelemtry.io/auto/database/sql`. (#507)
- The instrumentation scope name for the `gin` instrumentation is now `go.opentelemtry.io/auto/github.com/gin-gonic/gin`. (#507)
- The instrumentation scope name for the `google.golang.org/grpc/client` instrumentation is now `go.opentelemtry.io/auto/google.golang.org/grpc`. (#507)
- The instrumentation scope name for the `google.golang.org/grpc/server` instrumentation is now `go.opentelemtry.io/auto/google.golang.org/grpc`. (#507)
- The instrumentation scope name for the `net/http/client` instrumentation is now `go.opentelemtry.io/auto/net/http`. (#507)
- The instrumentation scope name for the `net/http/server` instrumentation is now `go.opentelemtry.io/auto/net/http`. (#507)
- The instrumentation for `client.Do` was changed to instrumentation for `Transport.roundTrip`. ([#529](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/529))

### Fixed

- Support commit hash version for dependencies.
  If a dependency falls within a known version range used by instrumentation, and its offset structure has not changed, instrumentation will default to the known offset value instead of failing to run. ([#503](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/503))

## [v0.8.0-alpha] - 2023-11-14

### Added

- Add the `WithEnv` `InstrumentationOption` to configure `Instrumentation` to parse the environment.
  The `Instrumentation` will no longer by default parse the environment.
  This option needs to be used to enable environment parsing, and the order it is passed influences the environment precedence.
  All options passed before this one will be overridden if there are conflicts, and those passed after will override the environment. ([#417](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/417))
- Add the `WithTraceExporter` `InstrumentationOption` to configure the trace `SpanExporter` used by an `Instrumentation`. ([#426](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/426))
- Add HTTP status code attribute to `net/http` server instrumentation. ([#428](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/428))
- The instrumentation scope now includes the version of the auto-instrumentation project. ([#442](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/442))
- Add a new `WithSampler` method allowing end-users to provide their own implementation of OpenTelemetry sampler directly through the package API. ([#468](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/468)).
- Add uprobes to `execDC` in order to instrument SQL DML. ([#475](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/475))

### Changed

- Documentation no longer says that `SYS_PTRACE` capability is needed. ([#388](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/388))
- The `NewInstrumentation` no longer parses environment variables by default.
  Use the new `WithEnv` option to enable environment parsing. ([#417](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/417))
- `NewInstrumentation` now requires a `context.Context` as its first argument.
  This context is used in the instantiation of exporters. ([#426](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/426))
- `Instrumentation` now uses an OTLP over HTTP/protobuf exporter (changed from gRPC/protobuf). ([#426](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/426))

### Fixed

- Parse Go versions that contain `GOEXPERIMENT` suffixes. ([#389](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/389))
- Include the schema URL for the semantic convention used in the exported resource. ([#426](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/426))
- Support Go module `replace` directives for the `golang.org/x/net` within the `google.golang.org/grpc` server instrumentation. ([#450](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/450))

### Removed

- The deprecated `go.opentelemetry.io/auto/examples/rolldice` module is removed. ([#423](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/423))

## [v0.7.0-alpha] - 2023-10-15

### Added

- Add `WithServiceName` config option for instrumentation. ([#353](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/353))
- Add `WithPID` config option for instrumentation. ([#355](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/355))

### Changed

- Fix bug in the `net/http` server instrumentation which always created a new span context. ([#266](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/266))
- Fix runtime panic if OTEL_GO_AUTO_TARGET_EXE is not set. ([#339](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/339))
- Improve eBPF context propagation stability ([#368](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/368))

### Deprecated

- The `go.opentelemetry.io/auto/examples/rolldice` module is deprecated.
  It will be moved into the `go.opentelemetry.io/auto/examples` module in the following release. ([#304](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/304))

### Removed

- The deprecated `go.opentelemetry.io/auto/offsets-tracker` module is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gorilla/mux` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/test/e2e/gorillamux` module is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/inject` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/errors` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/process` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/process/ptrace` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/opentelemetry` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/net/http/client` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/net/http/server` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gin-gonic/gin` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gorilla/mux` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc/server` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/utils` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/context` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/allocator` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/bpffs` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/instrumentors/events` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/pkg/log` package is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/test/e2e/gin` module is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/test/e2e/gorillamux` module is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated `go.opentelemetry.io/auto/test/e2e/nethttp` module is removed. ([#302](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/302))
- The deprecated instrumentation support for `github.com/gorilla/mux` is removed. ([#303](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/303))

## [v0.3.0-alpha] - 2023-09-12

### Added

- Add database/sql instrumentation ([#240](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/240))
- Support Go 1.21. ([#264](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/264))
- Add `Instrumentation` to `go.opentelemetry.io/auto` to manage and run the auto-instrumentation provided by the project. ([#284](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/284))
  - Use the `NewInstrumentation` to create a `Instrumentation` with the desired configuration by passing zero or more `InstrumentationOption`s.
  - Use `WithTarget` when creating an `Instrumentation` to specify its target binary.

### Changed

- The function signature of `"go.opentelemetry.io/auto/offsets-tracker/downloader".DownloadBinary` has changed.
  It now has an additional flag indicating whether it'll build a dummy app for Go stdlib packages or not. ([#256](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256))
- The function signature of `"go.opentelemetry.io/auto/offsets-tracker/target".New` has changed.
  It now accepts a flag to determine if the returned `Data` is from the Go stdlib or not. ([#256](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256))
- Use UPROBE_RETURN to declare the common uprobe return logic (finding the corresponding context, setting up end time, and sending the event via perf buffer) ([#257](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257))
- BASE_SPAN_PROPERTIES as common fields (start time, end time, SpanContext and ParentSpanContext) for all instrumentations events (consistent between C and Go structs). ([#257](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257))
- Header guards in eBPF code. ([#257](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257))

### Fixed

- Fix context propagation across different goroutines. ([#118](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/118))
- The offset tracker can once again build binaries for the Go stdlib. ([#256](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256))

### Deprecated

- The `go.opentelemetry.io/auto/offsets-tracker` module is deprecated.
  It will be removed in the following release. ([#281](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/281))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gorilla/mux` package is deprecated.
  It will be removed in the following release. ([#262](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/262))
- The `go.opentelemetry.io/auto/test/e2e/gorillamux` module is deprecated.
  It will be removed in the following release. ([#262](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/262))
- The `go.opentelemetry.io/auto/pkg/inject` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/errors` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/process` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/process/ptrace` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/opentelemetry` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/net/http/client` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/net/http/server` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gin-gonic/gin` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gorilla/mux` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc/server` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/utils` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/context` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/allocator` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/bpffs` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/instrumentors/events` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/pkg/log` package is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/test/e2e/gin` module is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/test/e2e/gorillamux` module is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))
- The `go.opentelemetry.io/auto/test/e2e/nethttp` module is deprecated.
  It will be removed in the following release. ([#282](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/282))

## [v0.2.2-alpha] - 2023-07-12

### Added

- The `net/http` client instrumentor. ([#91](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/91))
- Context propagation to the `net/http` server instrumentation. ([#92](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/92))
- Simplified example of an HTTP service in `go.opentelemtry.io/auto/examples/rolldice`. ([#195](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/195))

### Changed

- Upgrade OpenTelemetry semantic conventions to v1.18.0. ([#162](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/162))
- Remove the HTTP path from span names in `net/http`, `gin-gonic/gin`, and `gorilla/mux` instrumentations. ([#161](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/161))
- Update generated offsets. ([#186](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/186))
- Reduce Docker image size by using different base image. ([#182](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/182))
- Support for multiple processes in BPF FS. ([#211](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/211))

## [v0.2.1-alpha] - 2023-05-15

### Fixed

- Fix gRPC instrumentation memory access issue on newer kernels. ([#150](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/150))

### Changed

- Only pull docker image if not present for the emojivoto example. ([#149](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/149))
- Update HTTP span names to include method and route to match semantic conventions. ([#143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/143))
- Fix missing spans in gorillamux instrumentation. ([#86](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/86))
- Update HTTP span names to include method and route to match semantic conventions. ([#143](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/143))
- Add DockerHub to release destinations. ([#152](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/152))

## [v0.2.0-alpha] - 2023-05-03

### Added

- Add [gin-gonic/gin](https://github.com/gin-gonic/gin) instrumentation. ([#100](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/100))
- Add ARM64 support. ([#82](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/82))
- Add `OTEL_GO_AUTO_SHOW_VERIFIER_LOG` environment variable to control whether
  the verifier log is shown. ([#128](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/128))

### Changed

- Use verion spans in `offsets_results.json` instead of storing each version. ([#45](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/45))
- Change `OTEL_TARGET_EXE` environment variable to `OTEL_GO_AUTO_TARGET_EXE`.
  ([#97](https://github.com/open-telemetry/opentelemetry-go-instrumentation/issues/97))

## [v0.1.0-alpha] - 2023-04-17

This is the first release of OpenTelemetry Go Automatic Instrumentation.

[Unreleased]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/compare/sdk/v1.2.1...HEAD
[`go.opentelemetry.io/auto/sdk` v1.2.1]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/sdk/v1.2.1
[v0.23.0/v1.2.0]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.23.0
[v0.22.1]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.22.1
[v0.22.0]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.22.0
[v0.21.0]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.21.0
[v0.20.0]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.20.0
[v0.19.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.19.0-alpha
[v0.18.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.18.0-alpha
[v0.17.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.17.0-alpha
[v0.16.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.16.0-alpha
[v0.15.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.15.0-alpha
[v0.14.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.14.0-alpha
[v0.13.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.13.0-alpha
[v0.12.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.12.0-alpha
[v0.11.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.11.0-alpha
[v0.10.1-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.10.1-alpha
[v0.10.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.10.0-alpha
[v0.9.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.9.0-alpha
[v0.8.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.8.0-alpha
[v0.7.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.7.0-alpha
[v0.3.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.3.0-alpha
[v0.2.2-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.2-alpha
[v0.2.1-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.1-alpha
[v0.2.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.0-alpha
[v0.1.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.1.0-alpha
[Go 1.23]: https://go.dev/doc/go1.23
[Go 1.22]: https://go.dev/doc/go1.22
