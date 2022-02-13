FROM fedora:35 as builder
RUN dnf install clang llvm make libbpf-devel -y
RUN curl -LO https://go.dev/dl/go1.17.6.linux-amd64.tar.gz && tar -C /usr/local -xzf go*.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN make build

# gcr.io/distroless/base-debian11
FROM fedora:35
COPY --from=builder /app/kv-go-instrumentation /
CMD ["/kv-go-instrumentation"]