FROM golang:1.24.0@sha256:3f7444391c51a11a039bf0359ee81cc64e663c17d787ad0e637a4de1a3f62a71 AS builder

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/kafka-go
COPY *.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	CGO_ENABLED=0 go build -v -o /usr/local/bin/app .

FROM bitnami/kafka:latest@sha256:7c91967e7a76eacbc823f951f4c3826ddfd8188ed3cfc99fc9b2fe0d5e89e75b

USER root
ENV KAFKA_CFG_PROCESS_ROLES=controller,broker
ENV KAFKA_CFG_NODE_ID=0
ENV KAFKA_CFG_LISTENERS=PLAINTEXT://127.0.0.1:9092,CONTROLLER://:9093
ENV KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
ENV KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=0@127.0.0.1:9093
ENV KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER
COPY --from=builder /usr/local/bin/app /usr/local/bin/app

# Add script target of /usr/local/bin/app
WORKDIR /opt/kafka/
COPY start.sh .
RUN chmod +x /opt/kafka/start.sh

ENTRYPOINT ["/usr/local/bin/app"]
CMD ["-setup=/opt/kafka/start.sh"]
