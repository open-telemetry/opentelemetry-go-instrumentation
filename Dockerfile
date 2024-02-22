FROM golang:1.22.0-bullseye as base

WORKDIR /app

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download && go mod verify

FROM base as builder
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    make build

FROM gcr.io/distroless/base-debian12@sha256:2102ce121ff1448316b93c5f347118a8e604f5ba7ec9dd7a5c2d8b0eac2941fe
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
