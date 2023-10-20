FROM golang:1.21.3-bullseye as builder

WORKDIR /app

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN make build

FROM gcr.io/distroless/base-debian12@sha256:5be49dea7bf68f9f193066dc922918279e6006f4efdea6846fd03927387058ca
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
