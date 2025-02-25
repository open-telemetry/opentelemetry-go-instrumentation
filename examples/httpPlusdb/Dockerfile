FROM golang:1.24.0@sha256:58cf31c1b10858d5772475e4f86853a03e0e220cc6cfd60965053951a2563b5e
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
