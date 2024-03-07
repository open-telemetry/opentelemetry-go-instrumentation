FROM golang:1.22.0 AS builder
WORKDIR /sample-app
COPY . .
RUN go mod init go.opentelemetry.io/auto/internal/test/e2e/kafka-go && go mod tidy && CGO_ENABLED=0 go build -o main

FROM moeenz/docker-kafka-kraft:latest
ENV KRAFT_CONTAINER_HOST_NAME=kafka
ENV KRAFT_CREATE_TOPICS=topic1,topic2
ENV KRAFT_PARTITIONS_PER_TOPIC=1
WORKDIR /sample-app
COPY --from=builder /sample-app/main .
COPY start.sh .
RUN chmod +x start.sh
WORKDIR /opt/kafka
ENTRYPOINT ["/sample-app/start.sh"]