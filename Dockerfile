FROM debian:12 as builder
ARG TARGETARCH
RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev -y
RUN curl -LO https://go.dev/dl/go1.20.linux-${TARGETARCH}.tar.gz && tar -C /usr/local -xzf go*.linux-${TARGETARCH}.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

FROM gcr.io/distroless/base-debian12@sha256:cc22d6da39ff5d08ef85edde7bd291b6ddaf24b9da5c98a1a1cc567751a96af3
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
