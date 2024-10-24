FROM golang:1.23
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/runner
COPY . /usr/src/go.opentelemetry.io/auto/
RUN go build -v -o /usr/local/bin/runner ./...
ENTRYPOINT ["runner"]

