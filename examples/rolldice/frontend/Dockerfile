FROM golang:1.23@sha256:e40ac815076dfc9e2bd26c35f1837383f44b6e0a34ca365313cea6c97b04c188

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
