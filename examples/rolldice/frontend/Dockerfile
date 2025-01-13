FROM golang:1.23@sha256:7ea4c9dcb2b97ff8ee80a67db3d44f98c8ffa0d191399197007d8459c1453041

WORKDIR /usr/src/user

COPY user/ ./

WORKDIR /usr/src/frontend

COPY frontend/go.mod frontend/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY frontend/ .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/frontend ./...

ENTRYPOINT ["frontend"]
