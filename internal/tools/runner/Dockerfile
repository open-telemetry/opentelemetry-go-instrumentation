FROM golang:1.23 AS builder

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/tools/runner

COPY . /usr/src/go.opentelemetry.io/auto/
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o /usr/local/bin/runner ./...

ENTRYPOINT ["/usr/local/bin/runner"]
