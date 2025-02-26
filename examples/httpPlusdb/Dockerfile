FROM golang:1.24.0@sha256:cd0c949a4709ef70a8dad14274f09bd07b25542de5a1c4812f217087737efd17
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
