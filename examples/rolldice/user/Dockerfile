FROM golang:1.26.1@sha256:cd78d88e00afadbedd272f977d375a6247455f3a4b1178f8ae8bbcb201743a8a
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
