FROM --platform=$BUILDPLATFORM golang:1.24.5-bookworm@sha256:ef8c5c733079ac219c77edab604c425d748c740d8699530ea6aced9de79aea40 AS base

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

FROM gcr.io/distroless/base-debian12@sha256:007fbc0e0df2f12b739e9032a45ade4c58be0c9981767c440da6c404418f3144
COPY --from=builder /usr/src/go.opentelemetry.io/auto/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
