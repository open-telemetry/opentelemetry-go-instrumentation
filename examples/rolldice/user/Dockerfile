FROM golang:1.23@sha256:e213430692e5c31aba27473cdc84cfff2896d0c097e984bef67b6a44c75a8181

WORKDIR /usr/src/user

COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...

CMD ["user"]
