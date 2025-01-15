FROM  --platform=$BUILDPLATFORM golang:1.23.4-bookworm@sha256:6f085f2a025fcd189fefd7dc51c98ad34fa40c749f73b5522aa53b27278e4ec1 AS base

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

FROM gcr.io/distroless/base-debian12@sha256:74ddbf52d93fafbdd21b399271b0b4aac1babf8fa98cab59e5692e01169a1348
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
