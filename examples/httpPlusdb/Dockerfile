FROM golang:1.20
WORKDIR /app
COPY . .
RUN go get github.com/mattn/go-sqlite3
RUN go build -o main
ENTRYPOINT ["/app/main"]
