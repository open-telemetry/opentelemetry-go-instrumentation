# Changelog

All notable changes to OpenTelemetry Go Automatic Instrumentation are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

OpenTelemetry Go Automatic Instrumentation adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add `WithServiceName` config option for instrumentation. ([#353](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/353))

### Changed

- Fix runtime panic if OTEL_GO_AUTO_TARGET_EXE is not set. ([#339](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/339))

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

[Unreleased]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/compare/v0.3.0-alpha...HEAD
[v0.3.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.3.0-alpha
[v0.2.2-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.2-alpha
[v0.2.1-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.1-alpha
[v0.2.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.0-alpha
[v0.1.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.1.0-alpha
