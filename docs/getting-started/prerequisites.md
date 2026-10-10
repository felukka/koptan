# Prerequisites

## Cluster

| Requirement | Notes |
|---|---|
| Kubernetes **1.20 or newer** | Any distribution works: kind, k3s, EKS, GKE, AKS, OpenShift and others. |
| `kubectl` | Configured for the cluster, with rights to install CRDs and ClusterRoles (usually cluster-admin, for the installation only). |
| Helm **3.8 or newer** | Needed for installing from the OCI registry. |
| Outbound network access | Build Pods clone repositories and pull base images. The operator talks to your git host every minute. |

Builds run inside the cluster with [buildah](https://buildah.io/) in rootless, `vfs` mode. They need no privileged containers and no Docker daemon.

## A container registry

Koptan pushes every image it builds to a registry, and the cluster pulls it from there. You can use Docker Hub, GitHub Container Registry, GitLab, Harbor, ECR, GCR, a registry inside the cluster or any other OCI registry.

You need:

- A registry host, such as `ghcr.io` or `docker.io`.
- Credentials that can push, stored in a Kubernetes Secret of type `kubernetes.io/dockerconfigjson`. Koptan uses the same Secret to pull the image when it deploys.

```bash
kubectl create secret docker-registry my-registry \
  --docker-server=ghcr.io \
  --docker-username=<user> \
  --docker-password=<token> \
  -n <your-namespace>
```

!!! note
    A registry that accepts anonymous pushes, such as a test registry inside the cluster, needs no Secret.

## Git access

- **Public repositories** need nothing.
- **Private repositories** need a token with read access, stored in a Secret in the same namespace as the Service:

  ```bash
  kubectl create secret generic my-repo-token --from-literal=token=<token> -n <your-namespace>
  ```

Koptan accepts `https`, `http`, `ssh` and `git` repository URLs for Services. SelfService repositories must use `https`.

## Optional components

| Component | Needed for |
|---|---|
| [cert-manager](https://cert-manager.io/docs/installation/) | Serving operator metrics over HTTPS with a managed certificate (`certManager.enabled`). |
| A SonarQube server | The `sonarqube` CIPlugin type. |
| A GitHub token with `security_events` scope | Uploading CodeQL results to GitHub code scanning. |
| Slack or Teams incoming webhooks | Alerts to those tools. |
| An AI provider (Anthropic API key, or an OpenAI-compatible server such as Ollama) | SelfServices. |
| PostgreSQL | Keeping Koptan UI settings across restarts (optional). |

## Resource needs

The operator itself is small: it requests 10m CPU and 64Mi of memory, with a limit of 500m CPU and 128Mi. Builds are the heavy part. Each build runs in its own Pod in the Service's namespace, so make sure those namespaces have room for at least one build at a time.

Next: [Installation](installation.md).
