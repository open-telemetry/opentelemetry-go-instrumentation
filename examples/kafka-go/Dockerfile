FROM golang:1.25.5@sha256:581c059c1d53b96a6db51a2f7a0eb943491ba1da50f1e43700db0cc325618096
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
