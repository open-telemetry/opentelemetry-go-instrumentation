FROM golang:1.23.6@sha256:9bd95af8f36381d1c035993c1655c5483593854795f20a4fa378eef14c79220c
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
