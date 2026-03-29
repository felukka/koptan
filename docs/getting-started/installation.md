# Installation

Koptan is installed as a Kubernetes operator.

## Prerequisites

Before installing Koptan, make sure you have:

- a Kubernetes cluster
- `kubectl` configured for that cluster
- permissions to install CRDs and controller resources

## Install the CRDs

You can install the CRDs using the manifests under `config/crd`.

Example:

```bash
kubectl apply -k config/crd
