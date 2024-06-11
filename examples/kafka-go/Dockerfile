FROM golang:1.22.4
WORKDIR /app
COPY . .
RUN go build -o main
ENTRYPOINT ["/app/main"]
