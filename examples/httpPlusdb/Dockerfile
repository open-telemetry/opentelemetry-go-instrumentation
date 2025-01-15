FROM golang:1.23.4@sha256:5305905078d6c33f427b4c7dc483e1bb4bef2991ca95f4ac2e830cb1f0a717fe
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
