# Declare

Kubernetes tools that facilitate declarative resource management.

## Components

### Controller

The Controller resource can be used to build Kubernetes operators using custom resources. This resource allows organizations to easily extend the Kubernetes API with abstractions for their teams with minimal code.

[See Example](./library/webapis/controller.yaml)

Supported languages:
* [Jsonnet](https://jsonnet.org/)

#### Quickstart

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

## Library Development

Tools:
- [kind](https://kind.sigs.k8s.io/)
- [kustomize](https://kustomize.io/)
- [skaffold](https://skaffold.dev/)

Start a dev environment.

```sh
./hack/dev-up.sh
skaffold dev
make run
```

... Hack on ./library ...

Cleanup.

```sh
./hack/dev-down.sh
```

## Related Projects

- https://github.com/GoogleCloudPlatform/metacontroller
