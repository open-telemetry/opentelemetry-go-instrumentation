FROM --platform=$BUILDPLATFORM golang:1.24.1-bookworm@sha256:fa1a01d362a7b9df68b021d59a124d28cae6d99ebd1a876e3557c4dd092f1b1d AS base

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

FROM --platform=$BUILDPLATFORM base AS builder

WORKDIR /usr/src/go.opentelemetry.io/auto/

# Copy auto/sdk so `go mod` finds the replaced module.
COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk/

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download && go mod verify

COPY . .

ARG TARGETARCH
ARG CGO_ENABLED=0
ARG BPF2GO_CFLAGS="-I/usr/src/go.opentelemetry.io/auto/internal/include/libbpf -I/usr/src/go.opentelemetry.io/auto/internal/include"
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOARCH=$TARGETARCH \
	CGO_ENABLED=$CGO_ENABLED \
	BPF2GO_CFLAGS=$BPF2GO_CFLAGS \
	go generate ./... \
	&& go build -o otel-go-instrumentation ./cli/...

FROM gcr.io/distroless/base-debian12@sha256:27769871031f67460f1545a52dfacead6d18a9f197db77110cfc649ca2a91f44
COPY --from=builder /usr/src/go.opentelemetry.io/auto/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
