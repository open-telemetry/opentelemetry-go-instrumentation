FROM golang:1.23.5@sha256:8c10f21bec412f08f73aa7b97ca5ac5f28a39d8a88030ad8a339fd0a781d72b4
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
