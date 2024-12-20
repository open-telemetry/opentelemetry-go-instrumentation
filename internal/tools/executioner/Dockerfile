FROM golang:1.23

WORKDIR /usr/src/go.opentelemetry.io/auto/internal/tools/executioner

COPY . /usr/src/go.opentelemetry.io/auto/
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o /usr/local/bin/executioner ./...

ENTRYPOINT ["/usr/local/bin/executioner"]
