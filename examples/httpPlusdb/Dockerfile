FROM golang:1.24.1@sha256:52ff1b35ff8de185bf9fd26c70077190cd0bed1e9f16a2d498ce907e5c421268
WORKDIR /app
# Copy go.mod for better layer caching
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY ./*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -o main
ENTRYPOINT ["/app/main"]
