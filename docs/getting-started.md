# Getting Started with OpenTelemetry Go Automatic Instrumentation

You can instrument a Go executable using OpenTelemetry without writing additional code.
All you need to do is configure a few environment variables and run the instrumentation with elevated privileges.

This guide demonstrates how to automatically instrument a Go application in Kubernetes, using Docker, and on a Linux host.

## Instrument an Application in Kubernetes

To instrument an application running in Kubernetes, follow these steps:

1. **Update your Kubernetes manifest**:

   - Add the OpenTelemetry Go Automatic Instrumentation container image.
   - Ensure `runAsUser` is set to `0` and `privileged` is set to `true`.

   Example:

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

2. **Verify `shareProcessNamespace` is enabled**:

   - Check if the `shareProcessNamespace` configuration is present in the pod spec.
     Add it if missing. Refer to the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/).

3. **Deploy the application** and the instrumentation using the updated manifest.

## Instrument an Application in Docker Compose

To instrument a containerized application, follow these steps:

1. **Modify your `docker-compose.yaml` file**:

   - Add a Docker network, a shared volume, and a service for your application.

2. **Add a new service for the instrumentation**:

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

   For more environment variables, refer to the [OpenTelemetry SDK configuration documentation](https://opentelemetry.io/docs/languages/sdk-configuration/).

3. **Start the instrumentation** by running:

   ```sh
   docker compose up
   ```

## Instrument an Application on the Same Host

Follow these steps to instrument an application running on the same host:

### Prerequisites

Ensure you have the following:

- **Linux**: Kernel version 4.19 or higher
- **Processor**: x64 or ARM
- **Go**: Version 1.18 or higher
- **Instrumentation Binary**: Compile the OpenTelemetry Go Automatic Instrumentation binary by running:

  ```sh
  make build
  ```

### Steps

1. **Start the target application.**

2. **Set environment variables** before running the instrumentation:

   - `OTEL_GO_AUTO_TARGET_EXE`: Full path to the executable to instrument. Example: `/home/bin/service_executable`
   - `OTEL_SERVICE_NAME`: Name of your service or application
   - `OTEL_EXPORTER_OTLP_ENDPOINT`: Observability backend endpoint. Example: `http://localhost:4318`

3. **Run the OpenTelemetry Go Automatic Instrumentation** with root privileges.

   > **Note**: If the target application is not running, the instrumentation will wait for the process to start.

   Example command:

   ```sh
   sudo OTEL_GO_AUTO_TARGET_EXE=/home/bin/service_executable OTEL_SERVICE_NAME=my_service OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./otel-go-instrumentation
   ```

## Configuration

For additional configuration options, refer to the [`InstrumentationOption`](https://pkg.go.dev/go.opentelemetry.io/auto#InstrumentationOption) factory functions in the OpenTelemetry Go Automatic Instrumentation documentation.
