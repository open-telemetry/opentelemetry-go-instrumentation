FROM golang:1.22.0
WORKDIR /sample-app
COPY . .
RUN go mod init go.opentelemetry.io/auto/internal/test/e2e/databasesql && go mod tidy && go build -o main
