FROM golang:1.23@sha256:8c10f21bec412f08f73aa7b97ca5ac5f28a39d8a88030ad8a339fd0a781d72b4

WORKDIR /usr/src/user

COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...

CMD ["user"]
