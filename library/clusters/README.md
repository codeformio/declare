# Clusters

An abstraction around [Cluster API](https://github.com/kubernetes-sigs/cluster-api).

## Quickstart

Follow the [Cluster API quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) up until the point where you "Create your first workload cluster" (we will use our controller for this).
Setup your AWS credentials (this example will incur charges).

Install Cluster controller.

```sh
kubectl apply -f ./library/clusters/bundle.yaml
```

Install example instance of a cluster.

```sh
kubectl apply -f ./library/clusters/example/
```

### Cleanup

Ensure you delete your cluster to prevent incurring long-term charges.

```sh
kubectl delete -f ./library/clusters/example/
```
