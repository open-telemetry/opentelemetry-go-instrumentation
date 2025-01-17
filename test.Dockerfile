FROM golang:1.23.4-bookworm@sha256:e4906bc13f563c90ea22151e831cda45d58838f2bee18823f0bcc717464ccfe5

WORKDIR /usr/src/go.opentelemetry.io/auto/

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY . .

ARG CGO_ENABLED=0

# TARGETARCH is an automatic platform ARG enabled by Docker BuildKit.
ARG TARGETARCH

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	CGO_ENABLED=$CGO_ENABLED \
	go build -o /usr/local/bin/otel-go-instrumentation ./cli/...

ENTRYPOINT ["/usr/local/bin/otel-go-instrumentation"]
