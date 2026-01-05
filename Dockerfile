FROM --platform=$BUILDPLATFORM golang:1.25.5-bookworm@sha256:2c7c65601b020ee79db4c1a32ebee0bf3d6b298969ec683e24fcbea29305f10e AS base

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
ENV GOARCH=$TARGETARCH

ARG CGO_ENABLED=0
ENV CGO_ENABLED=$CGO_ENABLED

ARG BPF2GO_CFLAGS="-I/usr/src/go.opentelemetry.io/auto/internal/include/libbpf -I/usr/src/go.opentelemetry.io/auto/internal/include"
ENV BPF2GO_CFLAGS=$BPF2GO_CFLAGS

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go generate ./... \
	&& go build -o otel-go-instrumentation ./cli/...

FROM gcr.io/distroless/base-debian12@sha256:f5a3067027c2b322cd71b844f3d84ad3deada45ceb8a30f301260a602455070e
COPY --from=builder /usr/src/go.opentelemetry.io/auto/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
