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
