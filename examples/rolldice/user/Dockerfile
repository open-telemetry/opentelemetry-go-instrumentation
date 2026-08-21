FROM golang:1.27.0@sha256:65b6f280bf050ec5af12716857e8ea8439d694dbba8f31ceeb7630670071f2bb
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
