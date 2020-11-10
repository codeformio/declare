# Web Services

A developer-focused abstraction for web-based services.

## Quickstart

(Assumes you have Declare running in your cluster.)

Install the WebService Controller.

```sh
kubectl apply -k ./library/webservices
```

Install an instance of a WebService.

```sh
kubectl apply -f ./library/webservices/example/hello-api.yaml
```

Query the status of the WebService. Note that the status of the underlying resources gets summarized here, maintaining a full abstraction.

```sh
kubectl get webservices
```

Information about the creation of child resources and any errors in the Controller logic will be exposed as events which can be inspected:

```sh
kubectl describe webservices
```

### Child Resources

#### Workload

##### A. Deployment

A Deployment is the default type of workload object that a WebService produces.

```sh
kubectl describe deployment hello
```

##### B. Rollout

A Rollout will be created in place of a Deployment if [Argo Rollouts](https://argoproj.github.io/argo-rollouts/) are installed on the cluster. This allows for canary rollout strategies.

```sh
kubectl describe rollout hello
```

#### Horizontal Pod Autoscaler

*NOTE: The k8s metrics server must be installed on the cluster for autoscaling to work.*

A Horizontal Pod Autoscaler should be created.

```sh
kubectl describe hpa hello
```

The HPA can take a long time to start adjusting pod counts.

#### Service

A Service should be created matching the name of the WebService.

```sh
kubectl describe service hello
```

#### Ingress

*NOTE: Currently only the official k8s NGINX ingress controller is supported.*

An Ingress should be created matching the name of the WebService (if `.spec.public = true`).

```sh
kubectl describe ingress hello
```

#### Network Policies

*NOTE: Requires a CNI with NetworkPolicy support.*

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

## Config

The WebService Controller can be configured using the following variables:

```yaml
  minRelicas: "1"
  maxRelicas: "10"
```

See [./config.yaml](./config.yaml) for example.

