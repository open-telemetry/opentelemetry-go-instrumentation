ARG BASE_IMAGE=golang:1.23.4@sha256:574185e5c6b9d09873f455a7c205ea0514bfd99738c5dc7750196403a44ed4b7
FROM ${BASE_IMAGE} AS builder

COPY sdk/ /usr/src/go.opentelemetry.io/auto/sdk

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test
COPY internal/test/go.mod internal/test/go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY internal/test/trigger/ ./trigger

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/test/e2e/kafka-go
COPY internal/test/e2e/kafka-go/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
	CGO_ENABLED=0 go build -v -o /usr/local/bin/kafka-go .

FROM bitnami/kafka:latest@sha256:978583d0859444d3bc6b6592b38b5af5a834d473db51441071a20e67e0afb945

USER root
ENV KAFKA_CFG_PROCESS_ROLES=controller,broker
ENV KAFKA_CFG_NODE_ID=0
ENV KAFKA_CFG_LISTENERS=PLAINTEXT://127.0.0.1:9092,CONTROLLER://:9093
ENV KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
ENV KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=0@127.0.0.1:9093
ENV KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER
COPY --from=builder /usr/local/bin/kafka-go /usr/local/bin/kafka-go
COPY --from=builder /usr/local/bin/runne[r] /usr/local/bin/runner

WORKDIR /opt/kafka/
COPY internal/test/e2e/kafka-go/start.sh .
RUN chmod +x /opt/kafka/start.sh

RUN ln -s /usr/local/bin/kafka-go /usr/local/bin/app

ENTRYPOINT ["/usr/local/bin/app"]
CMD ["-setup=/opt/kafka/start.sh"]
