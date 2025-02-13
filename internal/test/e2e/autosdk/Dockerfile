FROM golang:1.23.6@sha256:77a21b3e354c03e9f66b13bc39f4f0db8085c70f8414406af66b29c6d6c4dd85
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
