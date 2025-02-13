FROM golang:1.24.0@sha256:2b1cbf278ce05a2a310a3d695ebb176420117a8cfcfcc4e5e68a1bef5f6354da
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
