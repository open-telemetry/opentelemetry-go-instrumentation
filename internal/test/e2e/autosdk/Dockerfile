FROM golang:1.23.5@sha256:8c10f21bec412f08f73aa7b97ca5ac5f28a39d8a88030ad8a339fd0a781d72b4
# Make sure this dir exists.
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/autosdk

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk/

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY internal/test/e2e/autosdk/go.mod internal/test/e2e/autosdk/go.sum ./
RUN go mod download && go mod verify

COPY internal/test/e2e/autosdk/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
