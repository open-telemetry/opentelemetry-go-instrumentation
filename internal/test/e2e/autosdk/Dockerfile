FROM golang:1.23.3@sha256:2b01164887cecfa796f6bff6787c8c597d0e0e09e0694428fdde4a343303eb60
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
