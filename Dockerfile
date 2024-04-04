FROM golang:1.22.1-bullseye as base

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

FROM gcr.io/distroless/base-debian12@sha256:28a7f1fe3058f3efef4b7e5fe99f9c11d00eb09d9693b80bcb9d1f59989ba44a
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
