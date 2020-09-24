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

### Child Resources

#### Deployment

A Deployment should be created matching the name of the WebAPI.

```sh
kubectl describe deployment hello
```

#### Service

A Service should be created matching the name of the WebAPI.

```sh
kubectl describe service hello
```

#### Ingress

An Ingress should be created matching the name of the WebAPI (if `.spec.public = true`).

```sh
kubectl describe ingress hello
```

NOTE: Currently only the official k8s NGINX ingress controller is supported.

#### Network Policies

A network policy will be created (it may take a second) that only allows traffic from Pods specified in the allowedClients list.

```sh
kubectl describe networkpolicies hello
```

The policy works by allowing traffic based on `app` labels.

```sh
# This request should be allowed.
kubectl run busybox -l 'app=some-allowed-client' -i --image=busybox --restart=Never --rm -- wget --timeout=2 'http://hello'

# This request should not be allowed (it should timeout).
kubectl run busybox -l 'app=some-unknown-client' -i --image=busybox --restart=Never --rm -- wget --timeout=2 'http://hello'
```
