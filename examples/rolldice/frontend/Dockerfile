FROM golang:1.25.3@sha256:dd08f769578a5f51a22bf6a81109288e23cfe2211f051a5c29bd1c05ad3db52a
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
