FROM golang:1.22.1
WORKDIR /sample-app
COPY . .
RUN go mod init go.opentelemetry.io/auto/internal/test/e2e/gin && go mod tidy && go build -o main
