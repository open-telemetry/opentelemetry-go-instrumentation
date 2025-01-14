FROM golang:1.23.4@sha256:08e1417a41687ce17a8cfbf65285446a794ac5023635b848faf1c2ed1d7e475f
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
