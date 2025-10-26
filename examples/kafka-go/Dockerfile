FROM golang:1.25.3@sha256:dd08f769578a5f51a22bf6a81109288e23cfe2211f051a5c29bd1c05ad3db52a
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
