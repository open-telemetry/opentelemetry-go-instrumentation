FROM golang:1.27.1@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea
WORKDIR /app
# Copy go.mod for better layer caching
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY ./*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -o main
ENTRYPOINT ["/app/main"]
