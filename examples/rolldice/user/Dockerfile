FROM golang:1.24.3@sha256:02a22753ab3426d91ba5ba6f4dfb4ac2454f19b05afdb18d61ab02cbf1a2dffe
WORKDIR /usr/src/user
# Copy go.mod for better layer caching
COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
