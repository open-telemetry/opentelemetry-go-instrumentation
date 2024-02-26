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

FROM gcr.io/distroless/base-debian12@sha256:5eae9ef0b97acf7de819f936e12b24976b2d54333a2cf329615366e16ba598cd
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
