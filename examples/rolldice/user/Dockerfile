FROM golang:1.24.3@sha256:e84f2e80dc8ab2cd38c6e028f6e569f6ae54c08ad4caee492a596275086a2fa5
WORKDIR /usr/src/user
# Copy go.mod for better layer caching
COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
