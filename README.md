# OpenTelemetry Go Automatic Instrumentation

[![PkgGoDev](https://pkg.go.dev/badge/go.opentelemetry.io/auto)](https://pkg.go.dev/go.opentelemetry.io/auto)

This repository provides [OpenTelemetry] instrumentation for [Go] libraries using [eBPF].

:construction: This project is currently work in progress.

## Compatibility

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
| Ubuntu  | 1.21       | amd64        |
| Ubuntu  | 1.20       | amd64        |

Automatic instrumentation should work on any Linux kernel above 4.4.

OpenTelemetry Go Automatic Instrumentation supports the arm64 architecture.
However, there is no automated testing for this platform.
Be sure to validate support on your own ARM based system.

Users of non-Linux operating systems can use
[the Docker images](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pkgs/container/opentelemetry-go-instrumentation%2Fautoinstrumentation-go)
or create a virtual machine to compile and run OpenTelemetry Go Automatic Instrumentation.

## Get started

You can instrument a Go executable using OpenTelemetry without having
to write additional code. All you need to do is configure a few environment
variables and run the instrumentation with elevated privileges.

The following example shows how to instrument a Go application
automatically on a Linux host, through Docker, and using Kubernetes.

### Prerequisites

To instrument an application automatically, you need the following:

- Linux with kernel version 4.19 or higher
- x64 or ARM processor
- Docker image or compiled binary of OpenTelemetry Go Automatic Instrumentation
- Go 1.18 or higher

To compile the instrumentation binary, run `make build`.

### Instrument an application on the same host

To instrument an application on the same host, follow these steps:

1. Run the target application.

2. Set the following environment variables before running the instrumentation:

  - `OTEL_GO_AUTO_TARGET_EXE`: Full path of the executable you want to
  instrument. For example, `/home/bin/service_executable`
  - `OTEL_SERVICE_NAME`: Name of your service or application
  - `OTEL_EXPORTER_OTLP_ENDPOINT`: Your observability backend. For example,
  `http://localhost:4318`.

  For example:

  ```sh
  sudo OTEL_GO_AUTO_TARGET_EXE=/home/bin/service_executable OTEL_SERVICE_NAME=my_service OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./otel-go-instrumentation`
  ```

3. Run the OpenTelemetry Go Automatic Instrumentation with root privileges.

> **Note**
> If the target application isn't running yet, the instrumentation waits for
> the process to start.

### Instrument an application in Docker Compose

To instrument a containerized application, follow these steps:

1. Create or edit the docker-compose.yaml file. Make sure to add a Docker
network, a shared volume, and a service for the application.

2. Edit the docker-compose file to add a new service for the instrumentation:

  ```yaml
    go-auto:
      image: otel/autoinstrumentation-go
      privileged: true
      pid: "host"
      environment:
        - OTEL_EXPORTER_OTLP_ENDPOINT=http://<address_in_docker_network>:4318
        - OTEL_GO_AUTO_TARGET_EXE=<location_of_target_application_binary>
        - OTEL_SERVICE_NAME=<name_of_your_application>
        - OTEL_PROPAGATORS=tracecontext,baggage
      volumes:
        - <shared_volume_of_application>
        - /proc:/host/proc
  ```

3. Run `docker compose up`.

### Instrument an application in Kubernetes

To instrument an application running in Kubernetes, follow these steps:

1. Add the container image of the OpenTelemetry Go Automatic Instrumentation to your manifest. Make sure that `runAsUser` is set to `0`, `privileged` is set to `true`:

   ```yaml
   - name: autoinstrumentation-go
     image: otel/autoinstrumentation-go
     imagePullPolicy: IfNotPresent
     env:
       - name: OTEL_GO_AUTO_TARGET_EXE
         value: <location_of_target_application_binary>
       - name: OTEL_EXPORTER_OTLP_ENDPOINT
         value: "http://<address_in_network>:4318"
       - name: OTEL_SERVICE_NAME
         value: "<name_of_service>"
     securityContext:
       runAsUser: 0
       privileged: true
   ```
2. Check if the configuration [shareProcessNamespace](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/) is present in the pod spec, if not, please add it.

3. Deploy the application and the instrumentation using the manifest.

### Configuration

See the documentation for
[`InstrumentationOption`](https://pkg.go.dev/go.opentelemetry.io/auto#InstrumentationOption)
factory functions for information about how to configure the OpenTelemetry Go
Automatic Instrumentation.

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
