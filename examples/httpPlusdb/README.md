# Example of Auto instrumentation of HTTP server + SQL database

This example shows a trace being generated which is composed of a http request and sql db handling -
both visible in the trace.

For testing auto instrumentation, we can use the docker compose.

To run the example, bring up the services using the command.

```
docker compose up 
```

Now, you can hit the server using the below command

```
curl localhost:8080/query_db
```

Which will query the dummy sqlite database named test.db
Every hit to the server should generate a trace that we can observe in [Jaeger UI](http://localhost:16686/)
