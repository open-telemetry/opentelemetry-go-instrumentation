FROM golang:1.24.2@sha256:f52b85c1b58271ba213ac59878c4fe2fdbb41ac842c5c69f9e7a9d16d53c4ad8
WORKDIR /usr/src/user
# Copy go.mod for better layer caching
COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
