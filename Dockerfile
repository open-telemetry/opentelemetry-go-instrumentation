FROM fedora:37 as builder
RUN dnf install clang llvm make libbpf-devel -y
RUN curl -LO https://go.dev/dl/go1.20.linux-amd64.tar.gz && tar -C /usr/local -xzf go*.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

FROM registry.fedoraproject.org/fedora-minimal:37
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
