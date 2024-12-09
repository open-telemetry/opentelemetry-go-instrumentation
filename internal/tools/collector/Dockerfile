FROM golang:1.23 AS builder

RUN apt-get update && apt-get install -y curl

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/tools/collector

COPY . .
RUN go mod init go.opentelemetry.io/auto/internal/tools/collector && go mod tidy
RUN go build -o /usr/local/bin/collector ./...

EXPOSE 4318
EXPOSE 8080
EXPOSE 13133
ENTRYPOINT ["/usr/local/bin/collector"]
