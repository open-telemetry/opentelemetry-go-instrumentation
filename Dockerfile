FROM fedora:38 as builder
ARG TARGETARCH
RUN dnf install clang llvm make libbpf-devel -y
RUN curl -LO https://go.dev/dl/go1.20.linux-${TARGETARCH}.tar.gz && tar -C /usr/local -xzf go*.linux-${TARGETARCH}.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

FROM registry.fedoraproject.org/fedora-minimal:38
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
