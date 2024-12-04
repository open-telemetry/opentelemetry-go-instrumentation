FROM golang:1.23.4@sha256:a5ec4a1403fb63b1afc4643d707a4ee11ab4b4637fb73afa3f05ee67b9282c92
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
