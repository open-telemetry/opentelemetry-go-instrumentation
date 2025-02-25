FROM golang:1.24.0@sha256:a14c5a656f220b7067734beebef6362b0858a61d766ccaf320e0553329e1f34e
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
