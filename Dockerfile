FROM golang:1.23.1-bullseye as base

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

FROM gcr.io/distroless/base-debian12@sha256:88e0a2ac7c9b54f1ef941e7978c21fd45b46cc6e768f4bc28f3618a51438dc5d
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
