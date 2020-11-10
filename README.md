# Declare

*Disclaimer: Still unstable, reach out in you are looking to use this project in production and we can get a stable version out there soon.*

Declare facilitates declarative resource management by making it easy to define abstractions as Kubernetes-native objects.

Supported languages:
* [Javascript](./docs/javascript)
* [Jsonnet](./docs/jsonnet)

## Install

```sh
kustomize build ./config/default | kubectl apply -f -
```

## Library

### WebService

A WebService allows app teams to deploy web based services. This abstraction helps enforce best practices while implementing advanced features like autoscaling and canary rollouts.

```yaml
apiVersion: apps.codeform.io/v1alpha1
kind: WebService
metadata:
  name: hello
spec:
  port: 80
  image: "my-app:v1.3.0"
  allowedClients:
  - app: "client-a"
  - app: "client-b"
```

See [WebAPI quickstart guide](./library/webapis/).

### Project

A Project can be used to manage multiple Namespaces and related resources for teams.

See [Project Controller](./library/projects/controller.yaml).

### Cluster

A Cluster provides a simple abstraction around Cluster API resources for managing Kubernetes clusters.

See [Cluster quickstart guide](./library/clusters/).

## Related Projects

- https://github.com/GoogleCloudPlatform/metacontroller
