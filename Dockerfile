FROM fedora:35 as builder
RUN dnf install clang llvm make libbpf-devel -y
RUN curl -LO https://go.dev/dl/go1.18.linux-amd64.tar.gz && tar -C /usr/local -xzf go*.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
