FROM golang:1.21.6-bullseye as builder

WORKDIR /app

RUN apt-get update && apt-get install -y curl clang gcc llvm make libbpf-dev

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN make build

FROM gcr.io/distroless/base-debian12@sha256:f47fa3dbb9c1b1a5d968106c98380c40f28c721f0f8e598e8d760169ae2db836
COPY --from=builder /app/otel-go-instrumentation /
CMD ["/otel-go-instrumentation"]
