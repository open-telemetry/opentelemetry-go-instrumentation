FROM golang:1.23.4@sha256:9820aca42262f58451f006de3213055974b36f24b31508c1baa73c967fcecb99
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
