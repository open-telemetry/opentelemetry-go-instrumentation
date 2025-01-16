FROM golang:1.23@sha256:9820aca42262f58451f006de3213055974b36f24b31508c1baa73c967fcecb99

WORKDIR /usr/src/user

COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...

CMD ["user"]
