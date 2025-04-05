ARG GOLANG_VERSION=latest
FROM golang:$GOLANG_VERSION

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/nethttp
RUN go mod init go.opentelemetry.io/auto/internal/test/e2e/nethttp && go mod tidy

COPY *.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
