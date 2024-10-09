# Changelog

All notable changes to OpenTelemetry Go Automatic Instrumentation are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

OpenTelemetry Go Automatic Instrumentation adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Support `google.golang.org/grpc` `1.65.1`. ([#1174](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1174))

## [v0.15.0-alpha] - 2024-10-01

### Added

- Support Go `v1.21.13`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support Go `v1.22.6`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support `golang.org/x/net` `v0.28.0`. ([#988](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/988))
- Support `google.golang.org/grpc` `1.67.0-dev`. ([#1007](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1007))
- Support Go `1.23.0`.  ([#1007](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/1007))
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
  This flag, when used, enables the instrumentation of the OpenTelemetry default global implementation (https://pkg.go.dev/go.opentelemetry.io/otel).
  This means that all trace telemetry from this implementation that would normally be dropped will instead be recorded with the auto-instrumentation pipeline. ([#523]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/523)
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
  It now has an additional flag indicating whether it'll build a dummy app for Go stdlib packages or not. ([#256]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256)
- The function signature of `"go.opentelemetry.io/auto/offsets-tracker/target".New` has changed.
  It now accepts a flag to determine if the returned `Data` is from the Go stdlib or not. ([#256]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256)
- Use UPROBE_RETURN to declare the common uprobe return logic (finding the corresponding context, setting up end time, and sending the event via perf buffer) ([#257]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257)
- BASE_SPAN_PROPERTIES as common fields (start time, end time, SpanContext and ParentSpanContext) for all instrumentations events (consistent between C and Go structs). ([#257]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257)
- Header guards in eBPF code. ([#257]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/257)

### Fixed

- Fix context propagation across different goroutines. ([#118](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/118))
- The offset tracker can once again build binaries for the Go stdlib. ([#256]https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/256)

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

### Changed

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

[Unreleased]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/compare/v0.15.0-alpha...HEAD
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

[Go 1.22]: https://go.dev/doc/go1.22
