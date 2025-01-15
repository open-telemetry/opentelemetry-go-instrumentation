FROM golang:1.23.4@sha256:5305905078d6c33f427b4c7dc483e1bb4bef2991ca95f4ac2e830cb1f0a717fe

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/gin
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY *.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
