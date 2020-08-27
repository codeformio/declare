# Declare

Declare facilitates declarative resource management by making it easy to define abstractions as Kubernetes-native objects.

Supported languages:
* [Jsonnet](https://jsonnet.org/)

## Install

```sh
kustomize build ./config/default | kubectl apply -f -
```

## Library

### WebAPI

The WebAPI kinds allow app teams to easily specify deploy their application.

```yaml
apiVersion: apps.example.com/v1
kind: WebAPI
metadata:
  name: hello
spec:
  port: 80
  image: "nginx:1.14.2"
  allowedClients:
  - "client-a"
  - "client-b"
```

See [WebAPI Controller](./library/webapis/controller.yaml)

### Project

A Project kind can be used to manage multiple Namespaces and related resources for teams.

### Cluster

The Cluster kind expands out into Cluster API resources to managed Kubernetes clusters.

See [Cluster quickstart guide](./library/clusters/).

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
