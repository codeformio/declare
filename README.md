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

A WebAPI allows app teams to easily deploy their application without much Kubernetes knowledge.

```yaml
apiVersion: apps.example.com/v1
kind: WebAPI
metadata:
  name: hello
spec:
  port: 80
  image: "my-app:v1.3.0"
  allowedClients:
  - "client-a"
  - "client-b"
```

See [WebAPI quickstart guide](./library/webapis/).

### Project

A Project can be used to manage multiple Namespaces and related resources for teams.

See [Project Controller](./library/projects/controller.yaml).

### Cluster

A Cluster provides a simple abstraction around Cluster API resources for managing Kubernetes clusters.

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
