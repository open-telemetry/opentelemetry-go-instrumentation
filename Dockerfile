FROM debian:11 as builder
ARG TARGETARCH
RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev -y
RUN curl -LO https://go.dev/dl/go1.20.linux-${TARGETARCH}.tar.gz && tar -C /usr/local -xzf go*.linux-${TARGETARCH}.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

FROM gcr.io/distroless/base-debian11@sha256:73deaaf6a207c1a33850257ba74e0f196bc418636cada9943a03d7abea980d6d
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
