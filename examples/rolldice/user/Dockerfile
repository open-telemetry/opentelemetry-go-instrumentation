FROM golang:1.25.3@sha256:6ea52a02734dd15e943286b048278da1e04eca196a564578d718c7720433dbbe
WORKDIR /usr/src/go.opentelemetry.io/auto/examples/rolldice/user
# Use the local SDK copy.
COPY ./sdk /usr/src/go.opentelemetry.io/auto/sdk
# Copy go.mod for better layer caching
COPY ./examples/rolldice/user/go.mod ./examples/rolldice/user/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY ./examples/rolldice/user/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
