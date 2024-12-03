FROM golang:1.23.3@sha256:2b01164887cecfa796f6bff6787c8c597d0e0e09e0694428fdde4a343303eb60
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
