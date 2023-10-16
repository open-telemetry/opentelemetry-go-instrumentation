# Instrument a Go application or services automatically

You can instrument a Go executable using OpenTelemetry without having
to write additional code. All you need to do is configure a few environment
variables and run the instrumentation with elevated privileges.

The following example shows how to instrument a simple Go application
automatically on a Linux host, through Docker, and using Kubernetes.

## Prerequisites

To instrument an application automatically, you need the following:

- Binary file or Docker image of OpenTelemetry Go instrumentation
- Go 1.17 and higher in your system
- Linux with kernel version 4.4 and higher
- x64 or ARM processor

## Instrument an application on the same host

To instrument an application on the same host, follow these steps:

1. Set the following environment variables:

  - `OTEL_GO_AUTO_TARGET_EXE`: Full path of the executable you want to
  instrument
  - `OTEL_EXPORTER_OTLP_ENDPOINT`: Your observability backend. For example,
  `localhost:4317`
  - `OTEL_SERVICE_NAME`: Name of your service or application

2. Run the target application.

3. Run the OpenTelemetry Go instrumentation with root privileges.

## Instrument an application in Docker Compose

To instrument a containerized application, follow these steps:

1. Create or edit the docker-compose.yaml file. Make sure to add a Docker
network , a shared volume, and a service for the application  

2. Edit the docker-compose file to add a new service for the instrumentation:

  ```yaml
    go-auto:
      depends_on:
        - <name_of_your_application_service>
      image: otel/autoinstrumentation-go
      privileged: true
      cap_add:
        - SYS_PTRACE
      pid: "host"
      environment:
        - OTEL_EXPORTER_OTLP_ENDPOINT=http://<address_in_docker_network>:4317
        - OTEL_GO_AUTO_TARGET_EXE=<location_of_target_application_binary>
        - OTEL_SERVICE_NAME=<name_of_your_application>
        - OTEL_PROPAGATORS=tracecontext,baggage
      volumes:
        - <shared_volume_of_application>
        - /proc:/host/proc
  ```

3. Run `docker compose up`.

## Instrument an application in Kubernetes

To instrument an application running in Kubernetes, follow these steps:

1. Add the container image of the OpenTelemetry Go instrumentation to your manifest. Make sure that `runAsUser` is set to `0`, `privileged` is set to `true`, and that the `SYS_PTRACE` capability is present:

   ```yaml
   - name: <your_application_name>
     image: otel-go-instrumentation
     imagePullPolicy: IfNotPresent
     # env variables go here
     securityContext:
       runAsUser: 0
       capabilities:
         add:
           - SYS_PTRACE
       privileged: true
   ```

2. Add the OTel Go environment variables. For example:

   ```yaml
   - name: <your_application_name>
     image: otel/autoinstrumentation-go
     imagePullPolicy: IfNotPresent
     env:
       - name: OTEL_GO_AUTO_TARGET_EXE
         value: <location_of_target_application_binary>
       - name: OTEL_EXPORTER_OTLP_ENDPOINT
         value: "http://<address_in_network>:4317"
       - name: OTEL_SERVICE_NAME
         value: "<name_of_service>"
     securityContext:
       runAsUser: 0
       capabilities:
         add:
           - SYS_PTRACE
       privileged: true
   ```

The following is a full example of the instrumented emojivoto demo application running in Kubernetes:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: emojivoto
  labels:
    app.kubernetes.io/name: web
    app.kubernetes.io/part-of: emojivoto
    app.kubernetes.io/version: v11
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-svc
      version: v11
  template:
    metadata:
      labels:
        app: web-svc
        version: v11
    spec:
      serviceAccountName: web
      shareProcessNamespace: true
      terminationGracePeriodSeconds: 0
      containers:
        - env:
            - name: WEB_PORT
              value: "8080"
            - name: EMOJISVC_HOST
              value: emoji-svc.emojivoto:8080
            - name: VOTINGSVC_HOST
              value: voting-svc.emojivoto:8080
            - name: INDEX_BUNDLE
              value: dist/index_bundle.js
          image: docker.l5d.io/buoyantio/emojivoto-web:v11
          name: web-svc
          ports:
            - containerPort: 8080
              name: http
          resources:
            requests:
              cpu: 100m
        - name: <your_application_name>
          image: otel-go-instrumentation
          imagePullPolicy: IfNotPresent
          env:
            - name: OTEL_GO_AUTO_TARGET_EXE
              value: <location_of_target_application_binary>
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "http://<address_in_network>:4317"
            - name: OTEL_SERVICE_NAME
              value: "<name_of_service>"
          securityContext:
            runAsUser: 0
            capabilities:
              add:
                - SYS_PTRACE
            privileged: true
```

