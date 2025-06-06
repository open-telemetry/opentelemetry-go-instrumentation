FROM golang:1.24.4@sha256:db5d0afbfb4ab648af2393b92e87eaae9ad5e01132803d80caef91b5752d289c
WORKDIR /usr/src/user
COPY user/ ./
WORKDIR /usr/src/frontend
# Copy go.mod for better layer caching
COPY frontend/go.mod frontend/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY frontend/*.go ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/frontend ./...
ENTRYPOINT ["frontend"]
