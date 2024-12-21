ARG BASE_IMAGE=golang:1.23.4@sha256:70031844b8c225351d0bb63e2c383f80db85d92ba894e3da7e13bcf80efa9a37
FROM ${BASE_IMAGE}

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test
COPY internal/test/go.mod internal/test/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/trigger/ ./trigger

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/nethttp
COPY internal/test/e2e/nethttp/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
