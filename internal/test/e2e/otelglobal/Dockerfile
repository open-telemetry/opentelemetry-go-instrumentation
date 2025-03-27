FROM golang:1.24.1@sha256:52ff1b35ff8de185bf9fd26c70077190cd0bed1e9f16a2d498ce907e5c421268

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/otelglobal

# Copy auto/sdk so `go mod` finds the replaced module.
COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk/

COPY internal/test/e2e/otelglobal/go.mod internal/test/e2e/otelglobal/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/e2e/otelglobal/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	go build -v -o /usr/local/bin/app .

ENTRYPOINT ["/usr/local/bin/app"]
