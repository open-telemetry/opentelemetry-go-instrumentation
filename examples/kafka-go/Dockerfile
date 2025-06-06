FROM golang:1.24.4@sha256:db5d0afbfb4ab648af2393b92e87eaae9ad5e01132803d80caef91b5752d289c
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
