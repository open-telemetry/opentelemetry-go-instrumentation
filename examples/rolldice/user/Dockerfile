FROM golang:1.26.2@sha256:b54cbf583d390341599d7bcbc062425c081105cc5ef6d170ced98ef9d047c716
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
