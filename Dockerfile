FROM golang:1.21.6-bullseye as builder

WORKDIR /app

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN make build

FROM gcr.io/distroless/base-debian12@sha256:0a93daa199e7c6e387cea8cf03fac676146735caf6965d276d86ebd3a441f27e
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
