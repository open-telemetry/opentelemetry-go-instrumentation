FROM golang:1.24.2@sha256:991aa6a6e4431f2f01e869a812934bd60fbc87fb939e4a1ea54b8494ab9d2fc6

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/grpc

# Copy auto/sdk so `go mod` finds the replaced module.
COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk/

COPY internal/test/e2e/grpc/go.mod internal/test/e2e/grpc/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/e2e/grpc/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
