FROM golang:1.25.5@sha256:6cc2338c038bc20f96ab32848da2b5c0641bb9bb5363f2c33e9b7c8838f9a208
WORKDIR /app
# Copy go.mod for better layer caching
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY ./*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -o main
ENTRYPOINT ["/app/main"]
