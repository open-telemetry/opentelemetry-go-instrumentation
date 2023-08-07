# Changelog

All notable changes to OpenTelemetry Go Automatic Instrumentation are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

OpenTelemetry Go Automatic Instrumentation adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Fix context propagation across different goroutines. ([#118](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pull/118))

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

[Unreleased]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/compare/v0.2.2-alpha...HEAD
[v0.2.2-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.2-alpha
[v0.2.1-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.1-alpha
[v0.2.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.2.0-alpha
[v0.1.0-alpha]: https://github.com/open-telemetry/opentelemetry-go-instrumentation/releases/tag/v0.1.0-alpha
