# Example of Auto instrumentation of HTTP server

Rolldice server exposes an endpoint `/rolldice.` When we hit the endpoint, it returns a random number between 1-6. 

For testing auto instrumentation, we can use the docker compose. 

To run the example, bring up the services using the command.

```
docker compose up 
```

Now, you can hit roll dice server using the below command
```
curl localhost:8080/rolldice
```
Every hit to the server should generate a trace that we can observe in [Jaeger UI](http://localhost:16686/)

Example trace ![Image](jaeger.jpg)
