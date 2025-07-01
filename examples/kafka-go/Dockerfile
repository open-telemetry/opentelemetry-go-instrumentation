FROM golang:1.24.4@sha256:1aa97ddeb238eba47a930016f676e46f471e4e26e82d1a353ff2bf42304a48e2
WORKDIR /app
ARG BINARY_NAME
ENV BINARY_NAME=$BINARY_NAME
# Copy go.mod for better layer caching
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY ./*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -o $BINARY_NAME
ENTRYPOINT ["sh", "-c", "/app/$BINARY_NAME"]
