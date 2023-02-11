FROM fedora:35 as builder
ARG TARGETARCH
RUN dnf install clang llvm make libbpf-devel -y
RUN curl -LO https://go.dev/dl/go1.18.linux-$TARGETARCH.tar.gz && tar -C /usr/local -xzf go*.linux-$TARGETARCH.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . .
RUN TARGET=$TARGETARCH make build

FROM registry.fedoraproject.org/fedora-minimal:35
COPY --from=builder /app/kv-go-instrumentation /
CMD ["/kv-go-instrumentation"]