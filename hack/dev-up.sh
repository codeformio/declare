#!/usr/bin/env bash

kind create cluster --name declare.dev --config ./config/kind/kind.yaml

# Install CNI.
sh ./config/kind/calico.sh

# Install metrics server to enable HPAs.
kustomize build ./hack/metrics-server/ | kubectl apply -f -

# Install ArgoCD Rollouts (used for some library/ instances).
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://raw.githubusercontent.com/argoproj/argo-rollouts/stable/manifests/install.yaml

# Install controller CRD, etc.
make install
