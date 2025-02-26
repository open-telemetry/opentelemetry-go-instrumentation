FROM  --platform=$BUILDPLATFORM golang:1.24.0-bookworm@sha256:b970e6d47c09fdd34179acef5c4fecaf6410f0b597a759733b3cbea04b4e604a AS base

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
