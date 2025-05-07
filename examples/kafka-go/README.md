# Example of Auto instrumentation of HTTP server + Kafka producer + Kafka consumer + Manual span

This example shows a trace being generated which is composed of:

- `kafkaproducer` HTTP server handler which produces a batch of 2 message to different kafka topics.
- `kafkaconsumer`  consuming one of these messages, and generates a manual span for each message it handles, this span is visible as the son of the consumer span.

To run the example, bring up the services using the command.

```
docker compose up 
```

After the services are up, you can hit the server using the below command

```
curl localhost:8080/produce
```

Which will produce a batch of 2 messages.
Every hit to the server should generate a trace that we can observe in [Jaeger UI](http://localhost:16686/)
