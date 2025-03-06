ARG BASE_IMAGE=golang:1.24.1@sha256:c5adecdb7b3f8c5ca3c88648a861882849cc8b02fed68ece31e25de88ad13418
FROM ${BASE_IMAGE}

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test
COPY internal/test/go.mod internal/test/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/trigger/ ./trigger

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/otelglobal
COPY internal/test/e2e/otelglobal/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
