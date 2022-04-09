# OpenTelemetry Auto-Instrumentation for Go

This project adds [OpenTelemetry instrumentation](https://opentelemetry.io/docs/concepts/instrumenting/#automatic-instrumentation)
to Go applications without having to modify their source code.
We support a wide range of Go versions (1.12+) and even work on stripped binaries.

Our goal is to provide the same level of automatic instrumentation for Go as exists for languages such as Java and Python.

This automatic instrumentation is based on [eBPF](https://ebpf.io/) uprobes. For more information, see our [How it works](docs/how-it-works.md) document.

## Getting Started

Check out our [Getting Started on Kubernetes](docs/getting-started/README.md) guide for easily instrumenting your first Go applications.

## Current Instrumentations

| Library/Framework |
| ----------------- |
| net/http - Server |
| gRPC - Client     |
| gRPC - Server     |

## Project Status

This project is actively maintained by [keyval](https://keyval.dev) and is currently in it's initial days. We would love to receive your ideas, feedback & contributions.

## Contributing

Please refer to the [contributing.md](CONTRIBUTING.md) file for information about how to get involved. We welcome issues, questions, and pull requests.

## License

This project is licensed under the terms of the Apache 2.0 open source license. Please refer to [LICENSE](LICENSE) for the full terms.
