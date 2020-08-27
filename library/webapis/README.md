# WebAPIs

A developer-focused abstraction for web APIs.

## Quickstart

(Assumes you have Declare running in your cluster.)

Install the WebAPI Controller.

```sh
kubectl apply -k ./library/webapis
```

Install an instance of a WebAPI.

```sh
kubectl apply -f ./library/webapis/example/hello-api.yaml
```

List child resources.

```sh
kubectl get deployments
kubectl get services
kubectl get networkpolicies
kubectl get ingress
```
