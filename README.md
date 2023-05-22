# OpenTelemetry Go Automatic Instrumentation

This repository provides [OpenTelemetry] instrumentation for [Go] libraries using [eBPF].

## Project Status

:construction: This project is currently work in progress.

### Compatibility

OpenTelemetry Go Automatic Instrumentation is compatible with all current supported versions of the [Go language](https://golang.org/doc/devel/release#policy).

> Each major Go release is supported until there are two newer major releases.
> For example, Go 1.5 was supported until the Go 1.7 release, and Go 1.6 was supported until the Go 1.8 release.

For versions of Go that are no longer supported upstream, this repository will stop ensuring compatibility with these versions in the following manner:

- A minor release will be made to add support for the new supported release of Go.
- The following minor release will remove compatibility testing for the oldest (now archived upstream) version of Go.
   This, and future, releases may include features only supported by the currently supported versions of Go.

Currently, OpenTelemetry Go Automatic Instrumentation is tested for the following environments.

| OS      | Go Version | Architecture |
| ------- | ---------- | ------------ |
| Ubuntu  | 1.20       | amd64        |

Automatic instrumentation should work on any Linux kernel above 4.4.

OpenTelemetry Go Automatic Instrumentation supports the arm64 architecture.
However, there is no automated testing for this platform.
Be sure to validate support on your own ARM based system.

Users of non-Linux operating systems can use
[the Docker images](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pkgs/container/opentelemetry-go-instrumentation%2Fautoinstrumentation-go)
or create a virtual machine to compile and run OpenTelemetry Go Automatic Instrumentation.

## Contributing

See the [contributing documentation](./CONTRIBUTING.md).

## License

OpenTelemetry Go Automatic Instrumentation is licensed under the terms of the [Apache Software License version 2.0].
See the [license file](./LICENSE) for more details.

Third-party licesnes and copyright notices can be found in the [LICENSES directory](./LICENSES).

[OpenTelemetry]: https://opentelemetry.io/
[Go]: https://go.dev/
[eBPF]: https://ebpf.io/
[Apache Software License version 2.0]: https://www.apache.org/licenses/LICENSE-2.0
