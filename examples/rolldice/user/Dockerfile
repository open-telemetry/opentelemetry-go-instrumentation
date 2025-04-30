FROM golang:1.24.2@sha256:30baaea08c5d1e858329c50f29fe381e9b7d7bced11a0f5f1f69a1504cdfbf5e
WORKDIR /usr/src/user
# Copy go.mod for better layer caching
COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
