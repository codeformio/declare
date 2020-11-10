#!/usr/bin/env bash

set -e

export INSTANCE=full

cd ./e2e-test/cluster-instances/$INSTANCE
kind create cluster --name declare-$INSTANCE --config kind.yaml || echo "cluster already exists"
kustomize build ./01/ | kubectl apply -f -
kustomize build ./02/ | kubectl apply -f -

# Install controller CRD, etc.
cd -
make install

