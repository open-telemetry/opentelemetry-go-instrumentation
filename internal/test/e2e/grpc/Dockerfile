ARG BASE_IMAGE=golang:1.23.4@sha256:574185e5c6b9d09873f455a7c205ea0514bfd99738c5dc7750196403a44ed4b7
FROM ${BASE_IMAGE}

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test
COPY internal/test/go.mod internal/test/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/trigger/ ./trigger

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/grpc
COPY internal/test/e2e/grpc/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/grpc .

RUN ln -s /usr/local/bin/grpc /usr/local/bin/app

CMD ["/usr/local/bin/app"]
