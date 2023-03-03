# How to deploy

1. Deploy demo: you can use the blog example in the [deployments/00-demo-app.yml](./deployments/00-demo-app.yml) file.
   As a requirement, it must be compiled with Go 1.19+ and make use of the standard library HTTP handlers.
   ```
   $ kubectl apply -f ./deployments/00-demo-app.yml
   $ kubectl port-forward service/goblog 8443:8443
   ```

2. Provide your Grafana credentials. Use the following [K8s Secret template](deployments/01-example-k8s-agentconfig.yml.template)
   to introduce the endpoints, usernames and API keys for Mimir and Tempo:
   ```
   $ cp deployments/01-example-k8s-agentconfig.yml.template deployments/01-example-k8s-agentconfig.yml
   $ # EDIT the fields
   $ vim deployments/01-example-k8s-agentconfig.yml.template
   $ kubectl apply -f deployments/01-example-k8s-agentconfig.yml 
   ```
2. Deploy the auto-instrumenter+agent:
   ```
   kubectl apply -f deployments/02-auto-instrument.yml
   ```

You should be able to query traces and metrics in your Grafana board.
