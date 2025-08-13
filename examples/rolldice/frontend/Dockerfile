FROM golang:1.24.6@sha256:746a0e918c6fe61aaab1bcc9fd42068ee808e63c052d34ac3c9c36e061abddf6
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
