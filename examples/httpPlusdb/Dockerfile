FROM golang:1.23.4@sha256:95983c2237525489841f6c9dc8e0df3d5cfbcf7788d74a6720ef5163ab7fc0f7
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
