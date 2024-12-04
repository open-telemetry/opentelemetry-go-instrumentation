FROM golang:1.23.4@sha256:a5ec4a1403fb63b1afc4643d707a4ee11ab4b4637fb73afa3f05ee67b9282c92
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
