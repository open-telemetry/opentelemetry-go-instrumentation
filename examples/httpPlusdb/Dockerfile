FROM golang:1.23.5@sha256:e213430692e5c31aba27473cdc84cfff2896d0c097e984bef67b6a44c75a8181
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
