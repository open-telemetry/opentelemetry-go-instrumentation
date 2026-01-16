FROM golang:1.25.6@sha256:fc24d3881a021e7b968a4610fc024fba749f98fe5c07d4f28e6cfa14dc65a84c
WORKDIR /usr/src/user
COPY user/ ./
WORKDIR /usr/src/frontend
# Copy go.mod for better layer caching
COPY frontend/go.mod frontend/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY frontend/*.go ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/frontend ./...
ENTRYPOINT ["frontend"]
