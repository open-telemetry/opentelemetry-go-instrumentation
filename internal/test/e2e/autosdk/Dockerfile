FROM golang:1.23.4@sha256:70031844b8c225351d0bb63e2c383f80db85d92ba894e3da7e13bcf80efa9a37
# Make sure this dir exists.
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/autosdk

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk/

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY internal/test/e2e/autosdk/go.mod internal/test/e2e/autosdk/go.sum ./
RUN go mod download && go mod verify

COPY internal/test/e2e/autosdk/main.go .
RUN go build -v -o /sample-app/main

WORKDIR /sample-app

CMD ["/sample-app/main"]
