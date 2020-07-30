# Declare

Kubernetes tools that facilitate declarative resource management.

## Quickstart

```sh
make install && make run
```

In another terminal, install a Controller from the library.

```sh
kubectl apply -f ./library/webapis/bundle.yaml
```

Install an instance of the new resource.

```sh
kubectl apply -f ./library/webapis/example/hello-api.yaml
```

List child resources.

```sh
kubectl get deployments
kubectl get services
kubectl get networkpolicies
```

## Features

### Controller

The Controller resource can be used to build Kubernetes operators using custom resources. This resource allows organizations to easily extend the Kubernetes API with abstractions for their teams with minimal code.

Supported languages:
* [Jsonnet](https://jsonnet.org/)

Example:

```yaml
apiVersion: ctrl.declare.dev/v1
kind: Controller
metadata:
  name: webapis
spec:
  crdName: webapis.apps.example.com
  children:
  - apiVersion: apps/v1
    kind: Deployment
  - apiVersion: v1
    kind: Service
  source:
    controller.jsonnet: |
      function(request) {

        local obj = request.object,

        children: [
          {
            apiVersion: 'apps/v1',
            kind: 'Deployment',
            metadata: {
              name: obj.metadata.name,
              labels: {
                app: obj.metadata.name,
              },
            },
...
```

## Development

Requirements:
- [kind](https://kind.sigs.k8s.io/)
- [kustomize](https://kustomize.io/)

Start a dev environment.

```sh
./hack/dev-up.sh
```

Install a example Controller.

```sh
kustomize build ./library/webapis | kubectl apply -f -

kubectl apply -f ./library/webapis/example/hello-api.yaml
```

Run declare.

```sh
make run
```

In another terminal, get the created child resources.

```sh
kubectl get deployments
kubectl get services
kubectl get networkpolicies
```

Cleanup.

```sh
./hack/dev-down.sh
```
