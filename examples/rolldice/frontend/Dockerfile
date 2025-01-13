FROM golang:1.23

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
