# Migrating to OpenTelemetry eBPF Instrumentation (OBI)

OpenTelemetry Go Automatic Instrumentation is being merged with [OpenTelemetry eBPF Instrumentation (OBI)][obi] and this repository is no longer actively maintained.
This guide compares the two projects to help you move an existing deployment to OBI.

This is a comparison of what exists in both projects, not a promise of a zero-change migration.
Some features of this project have no direct equivalent in OBI.
Those are listed in [Known gaps](#known-gaps).

> [!NOTE]
> This guide was written for OpenTelemetry Go Automatic Instrumentation v0.24.0 and OBI v1.0.0.
> It is not updated for later releases.
> For newer versions of OBI, the [OBI documentation][obi] is the source of truth.

## Before you start

OBI has different host requirements than this project.
Check them first.

|                   | Go Automatic Instrumentation | OBI                                                            |
| ----------------- | ---------------------------- | -------------------------------------------------------------- |
| Linux kernel      | 4.19+                        | 5.8+, or RHEL-family 4.18+ with eBPF backports                 |
| Kernel features   |                              | BTF                                                            |
| CPU architecture  | amd64, arm64                 | amd64, arm64                                                   |
| Privileges        | Root and `privileged`        | Root, or the [Linux capabilities][obi-security] for the features you enable |
| Container image   | `otel/autoinstrumentation-go` | `otel/ebpf-instrument`                                        |

There are also differences in what OBI does by default:

- OBI produces metrics as well as traces.
- OBI instruments applications in any language, not only Go.
  A broad process selector instruments more than your Go services.
- OBI can run once per node as a DaemonSet instead of once per pod as a sidecar.

## Configuration

### Selecting the process to instrument

| Go Automatic Instrumentation | OBI                         | Notes                                                                 |
| ---------------------------- | --------------------------- | --------------------------------------------------------------------- |
| `OTEL_GO_AUTO_TARGET_EXE`    | `OTEL_EBPF_AUTO_TARGET_EXE` | OBI matches a glob against the full executable path, for example `*/server`. |
| `OTEL_GO_AUTO_TARGET_PID`    | `OTEL_EBPF_TARGET_PID`      |                                                                       |

OBI can also select processes by open port (`OTEL_EBPF_OPEN_PORT`), by language (`OTEL_EBPF_AUTO_TARGET_LANGUAGE`), and by Kubernetes metadata.
See [service discovery][obi-discovery].

### Service name and resource attributes

This project reads `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` from the environment of the instrumentation process.
OBI reads them from the environment of the *instrumented* process, then falls back to Kubernetes annotations, labels, and owner names, and finally to the executable name.
Set these variables on your application container instead of on the OBI container.
See [service discovery][obi-discovery] for the full precedence.

`OTEL_GO_AUTO_RESOURCE_DETECTORS` has no direct equivalent.
OBI adds Kubernetes and host metadata itself.

### Exporting and sampling

OBI supports the standard OpenTelemetry exporter environment variables, so these keep working unchanged:

- `OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
- `OTEL_EXPORTER_OTLP_PROTOCOL` and `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL`
- `OTEL_EXPORTER_OTLP_HEADERS`
- `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG`

See [export data][obi-export] and [sample traces][obi-sampling].

| Go Automatic Instrumentation      | OBI                          | Notes                                                        |
| --------------------------------- | ---------------------------- | ------------------------------------------------------------ |
| `OTEL_TRACES_EXPORTER=console`    | `OTEL_EBPF_TRACE_PRINTER`    | Prints traces to stdout, for example with the value `text`.  |
| `OTEL_TRACES_EXPORTER=zipkin`     | None                         | Export OTLP to an OpenTelemetry Collector and convert there. |
| `OTEL_LOG_LEVEL`                  | `OTEL_EBPF_LOG_LEVEL`        |                                                              |
| `OTEL_GO_AUTO_SHOW_VERIFIER_LOG`  | None                         |                                                              |

### Instrumentation options

| Go Automatic Instrumentation        | OBI  | Notes                                                                 |
| ----------------------------------- | ---- | --------------------------------------------------------------------- |
| `OTEL_GO_AUTO_GLOBAL`               | None | OBI detects use of the OpenTelemetry Go Trace API automatically. See [Manual spans](#manual-spans). |
| `OTEL_GO_AUTO_INCLUDE_DB_STATEMENT` | None | See [Known gaps](#known-gaps).                                        |
| `OTEL_GO_AUTO_PARSE_DB_STATEMENT`   | None | See [Known gaps](#known-gaps).                                        |

To limit which protocols OBI traces, use `OTEL_EBPF_TRACES_INSTRUMENTATIONS`.
To choose which metrics OBI produces, use `OTEL_EBPF_METRICS_FEATURES`.
Both are described in [export data][obi-export].

## Kubernetes sidecar

Both projects can run as a sidecar in a pod with `shareProcessNamespace: true`.

Before:

```yaml
- name: autoinstrumentation-go
  image: otel/autoinstrumentation-go
  env:
    - name: OTEL_GO_AUTO_TARGET_EXE
      value: /app/server
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: "http://otel-collector:4318"
    - name: OTEL_SERVICE_NAME
      value: "my-service"
  securityContext:
    runAsUser: 0
    privileged: true
```

After:

```yaml
- name: obi
  image: otel/ebpf-instrument:<version>
  env:
    - name: OTEL_EBPF_AUTO_TARGET_EXE
      value: "*/server"
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: "http://otel-collector:4318"
  securityContext:
    runAsUser: 0
    privileged: true
```

Move `OTEL_SERVICE_NAME` to the application container.

In a sidecar, OBI only sees the processes in its own pod, so `OTEL_EBPF_AUTO_TARGET_EXE` can be `*` to instrument everything in the pod.

See [Deploy OBI in Kubernetes][obi-k8s] for DaemonSet, Helm, and unprivileged deployments, and the OBI setup guides for [Docker][obi-docker] and [standalone][obi-standalone] use.

## Supported Go libraries

OBI instruments every library this project supports.

| Library                         | Go Automatic Instrumentation | OBI |
| ------------------------------- | ---------------------------- | --- |
| `net/http`                      | Yes                          | Yes |
| `google.golang.org/grpc`        | Yes                          | Yes |
| `database/sql`                  | Yes                          | Yes |
| `github.com/segmentio/kafka-go` | Yes                          | Yes |

OBI also instruments Go libraries this project does not, including `github.com/gin-gonic/gin`, `github.com/gorilla/mux`, `github.com/IBM/sarama`, `github.com/redis/go-redis/v9`, `go.mongodb.org/mongo-driver`, and `github.com/jackc/pgx/v5`.
See the Go library compatibility table in [export data][obi-export] for the current list and minimum versions.

## Manual spans

With `OTEL_GO_AUTO_GLOBAL`, this project records spans your application creates through the OpenTelemetry Go Trace API.
OBI does this without a setting, but with conditions.
The main ones are:

- Your application must not register its own global `TracerProvider`.
  This includes calling `otel.SetTracerProvider(auto.TracerProvider())`.
- Your application must use a version of `go.opentelemetry.io/otel` that OBI supports, without a `replace` directive for it.
- OBI needs permission to write to process memory, which requires `CAP_SYS_ADMIN` or a privileged container.

When these conditions are not met, OBI can still export partial spans, but they lack the instrumentation scope, events, and span kind.

See the Go Trace API section of [distributed traces][obi-traces] for the full conditions and the list of known limitations.

## Context propagation

This project propagates trace context for the libraries it instruments.
In OBI:

- Incoming trace context is read automatically.
- Go library-level propagation, which works for HTTP, HTTPS, HTTP/2, and gRPC, requires `CAP_SYS_ADMIN` or a privileged container.
  It is not available when the kernel is in integrity lockdown mode.
- Network-level propagation is disabled by default.
  Enable it with `OTEL_EBPF_BPF_CONTEXT_PROPAGATION`.

See [distributed traces][obi-traces].

## Telemetry differences

Span names, attributes, and instrumentation scope names produced by OBI are not identical to the ones produced by this project.
Dashboards, alerts, and sampling rules that depend on them need to be reviewed.

Run OBI next to your existing setup in a test environment and compare the output before switching.
`OTEL_EBPF_TRACE_PRINTER=text` prints the spans OBI produces to stdout.

## Known gaps

The following have no documented equivalent in OBI v1.0.0:

- **SQL statement options.**
  There is no single setting matching `OTEL_GO_AUTO_INCLUDE_DB_STATEMENT` and `OTEL_GO_AUTO_PARSE_DB_STATEMENT`.
- **Resource detectors.**
  There is no equivalent of `OTEL_GO_AUTO_RESOURCE_DETECTORS`.
- **Zipkin export.**
  OBI exports traces with OTLP.
- **Embedding as a Go library.**
  This project can be embedded in your own binary with [`auto.NewInstrumentation`][auto-pkg] and options such as `WithPID`, `WithSampler`, and `WithHandler`.
  OBI does not document an equivalent use.
- **Span limits.**
  There are no equivalents for `OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT` and the related span limit variables.

If one of these, or anything else, blocks your migration, [open an issue][obi-issue] in the OBI repository describing your scenario, what you are doing, and the outcome you expect.
Contributions to OBI that close these gaps are welcome.

[obi]: https://opentelemetry.io/docs/zero-code/obi/
[obi-security]: https://opentelemetry.io/docs/zero-code/obi/security/
[obi-discovery]: https://opentelemetry.io/docs/zero-code/obi/configure/service-discovery/
[obi-export]: https://opentelemetry.io/docs/zero-code/obi/configure/export-data/
[obi-sampling]: https://opentelemetry.io/docs/zero-code/obi/configure/sample-traces/
[obi-traces]: https://opentelemetry.io/docs/zero-code/obi/distributed-traces/
[obi-k8s]: https://opentelemetry.io/docs/zero-code/obi/setup/kubernetes/
[obi-docker]: https://opentelemetry.io/docs/zero-code/obi/setup/docker/
[obi-standalone]: https://opentelemetry.io/docs/zero-code/obi/setup/standalone/
[obi-issue]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/issues/new/choose
[auto-pkg]: https://pkg.go.dev/go.opentelemetry.io/auto#NewInstrumentation
