FROM golang:1.23.4@sha256:585103a29aa6d4c98bbb45d2446e1fdf41441698bbdf707d1801f5708e479f04
WORKDIR /app
COPY ./*.go .
RUN go mod init main
RUN go mod tidy
RUN go build -o main
ENTRYPOINT ["/app/main"]
