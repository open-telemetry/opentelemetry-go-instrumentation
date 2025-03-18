FROM golang:1.24.1@sha256:4dda7a0bc7ffff3e16e7096dd1300f075dea8e2f06c191932ed3af4814a67026
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
