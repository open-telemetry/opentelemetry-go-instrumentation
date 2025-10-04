FROM golang:1.25.1@sha256:d7098379b7da665ab25b99795465ec320b1ca9d4addb9f77409c4827dc904211
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
