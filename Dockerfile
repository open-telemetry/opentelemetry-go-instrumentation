FROM  --platform=$BUILDPLATFORM golang:1.23.3-bookworm@sha256:c59a9d759ae4cef5584870f0e88377eefa79d2c5cc7e634d4069701f1adbc907 AS base

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

WORKDIR /app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download && go mod verify

FROM --platform=$BUILDPLATFORM base AS builder
COPY . .

ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOARCH=$TARGETARCH make build

FROM gcr.io/distroless/base-debian12@sha256:e9d0321de8927f69ce20e39bfc061343cce395996dfc1f0db6540e5145bc63a5
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
