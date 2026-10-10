# Helm chart: `koptan`

The `koptan` chart installs the operator: the six CRDs, the controller Deployment, its RBAC and, optionally, metrics over HTTPS and the RBAC for the Koptan UI.

| | |
|---|---|
| Chart | `koptan` |
| OCI | `oci://ghcr.io/felukka/charts/koptan` |
| HTTPS repository | `https://charts.felukka.org` (`felukka/koptan`) |
| Operator image | `ghcr.io/felukka/koptan` (linux/amd64, linux/arm64) |
| Requirements | Kubernetes 1.20+, Helm 3.8+ |

## Install

=== "OCI registry (recommended)"

    ```bash
    helm install koptan oci://ghcr.io/felukka/charts/koptan \
      -n koptan-system --create-namespace

    # A specific version
    helm install koptan oci://ghcr.io/felukka/charts/koptan --version <chart-version> \
      -n koptan-system --create-namespace
    ```

=== "HTTPS repository"

    ```bash
    helm repo add felukka https://charts.felukka.org
    helm repo update
    helm search repo felukka
    helm install koptan felukka/koptan -n koptan-system --create-namespace
    ```

To change settings, write a `values.yaml` and pass it with `-f`:

```bash
helm show values oci://ghcr.io/felukka/charts/koptan > values.yaml
# edit values.yaml
helm upgrade --install koptan oci://ghcr.io/felukka/charts/koptan -n koptan-system -f values.yaml
```

After installing, Helm prints the next steps: checking the controller, creating the agent key and creating a first Service.

## Values

### Operator

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Controller replicas. Leader election is always on, so only one is active at a time. |
| `controller.image.repository` | `ghcr.io/felukka/koptan` | Operator image. |
| `controller.image.tag` | `""` | Image tag. Empty means the chart's `appVersion`. |
| `controller.image.pullPolicy` | `IfNotPresent` | Pull policy. |
| `imagePullSecrets` | `[]` | Pull Secrets for the operator image (for a private mirror). |
| `nameOverride` | `""` | Overrides the chart name in resource names. |
| `fullnameOverride` | `""` | Overrides the full resource name. |
| `defaultRegistry` | `docker.io` | Registry used when a Service sets no `spec.image.registry`. Sets `KOPTAN_DEFAULT_REGISTRY`. |
| `extraEnv` | `[]` | Extra environment variables for the manager container. |

### SelfService agent

| Value | Default | Description |
|---|---|---|
| `agent.image` | `ghcr.io/felukka/koptan-agent:latest` | Image of the SelfService agent pods. Sets `KOPTAN_AGENT_IMAGE`. Pin a version in production. |
| `agent.keySecret.name` | `koptan-agent-key` | Secret holding the agent key. Optional: without it SelfServices are disabled, everything else works. |
| `agent.keySecret.key` | `key` | Key inside that Secret. |

The chart does not create the Secret, so the key never ends up in your Helm values:

```bash
kubectl -n koptan-system create secret generic koptan-agent-key \
  --from-literal=key=$(openssl rand -hex 32)
kubectl -n koptan-system rollout restart deployment koptan
```

### Service account

| Value | Default | Description |
|---|---|---|
| `serviceAccount.create` | `true` | Create the controller's service account. |
| `serviceAccount.automount` | `true` | Mount its token. The controller needs it. |
| `serviceAccount.annotations` | `{}` | Annotations, for example for cloud workload identity. |
| `serviceAccount.name` | `controller-manager` | Service account name. |

### Pod and container

| Value | Default | Description |
|---|---|---|
| `tmpVolume.sizeLimit` | `2Gi` | Size of the `emptyDir` at `/tmp`. Discovery clones repositories there; the root filesystem stays read-only. Raise it for very large repositories. |
| `resources` | requests `10m` / `64Mi`, limits `500m` / `128Mi` | Controller resources. Builds do not run in the controller, so these rarely need changing. |
| `podAnnotations` | `{}` | Pod annotations. |
| `podLabels` | `{}` | Pod labels. |
| `podSecurityContext` | `{}` | Merged into the pod security context. `runAsNonRoot` and the `RuntimeDefault` seccomp profile are always set. |
| `securityContext` | `{}` | Merged into the container security context. A read-only root, no privilege escalation and dropping all capabilities are always set. |
| `volumes` / `volumeMounts` | `[]` | Extra volumes, for example a custom CA bundle. |
| `nodeSelector` / `tolerations` / `affinity` | `{}` / `[]` / `{}` | Scheduling. |

### Metrics

| Value | Default | Description |
|---|---|---|
| `metrics.secure` | `false` | Serve Prometheus metrics over HTTPS on `metrics.port`. When `false`, metrics are off. |
| `metrics.port` | `8443` | Metrics port. |
| `certManager.enabled` | `false` | Use cert-manager for the metrics certificate. Needs the cert-manager CRDs in the cluster. With `metrics.secure`, it also creates a self-signed Issuer, a Certificate, the Service `<release>-metrics-service` and the metrics RBAC. |

```yaml
# Metrics over HTTPS with a cert-manager certificate
certManager:
  enabled: true
metrics:
  secure: true
```

Grant your Prometheus service account the `<release>-metrics-reader` ClusterRole to scrape the endpoint.

### Koptan UI RBAC

| Value | Default | Description |
|---|---|---|
| `backstage.rbac.create` | `false` | Create the ClusterRole the Koptan UI backend needs. |
| `backstage.rbac.serviceAccount.name` | `""` | Bind the role to this service account, when set. |
| `backstage.rbac.serviceAccount.namespace` | `""` | Namespace of that service account. |

## Backstage RBAC

The [`koptan-ui` chart](koptan-ui.md) creates its own RBAC, so you do not need this when you use it. Use these values when you run the Koptan plugins in **your own Backstage**. The role lets the backend list every Koptan resource, create Services, Alerts and SelfServices, store their credentials as Secrets, patch Services (to trigger builds), reach SelfService agents through the `services/proxy` subresource and read node status.

```bash
helm upgrade --install koptan oci://ghcr.io/felukka/charts/koptan -n koptan-system \
  --set backstage.rbac.create=true \
  --set backstage.rbac.serviceAccount.name=backstage \
  --set backstage.rbac.serviceAccount.namespace=backstage
```

## Example values

```yaml
defaultRegistry: ghcr.io

agent:
  image: ghcr.io/felukka/koptan-agent:0.2.0

tmpVolume:
  sizeLimit: 5Gi

extraEnv:
  - name: HTTPS_PROXY
    value: http://proxy.corp:3128
  - name: NO_PROXY
    value: .svc,.cluster.local,10.0.0.0/8

nodeSelector:
  kubernetes.io/os: linux

certManager:
  enabled: true
metrics:
  secure: true
```

## What the chart installs

| Kind | Name | Notes |
|---|---|---|
| CustomResourceDefinition × 6 | `*.koptan.felukka.org` | From `crds/`. Installed on first install only (see [Upgrading](#upgrading)). |
| Deployment | `<release>` (`koptan`) | The controller. Read-only root, `emptyDir` at `/tmp`. |
| ServiceAccount | `controller-manager` | |
| ClusterRole + binding | `<release>-manager-role` | Koptan resources, Deployments, Services, Pods, ConfigMaps, Secrets and events. |
| Role + binding | leader election | In the release namespace. |
| ClusterRole | `<release>-metrics-reader`, metrics auth | For metrics. |
| Issuer, Certificate, Service | metrics | Only with `certManager.enabled` and `metrics.secure`. |
| ClusterRole + binding | `<release>-backstage-role` | Only with `backstage.rbac.create`. |

## Upgrading

```bash
helm upgrade koptan oci://ghcr.io/felukka/charts/koptan -n koptan-system --reuse-values
```

!!! warning "Helm does not upgrade CRDs"
    Helm installs the CRDs in `crds/` on the first install only. It never upgrades or deletes them. When a new version changes the CRDs, apply them yourself **before** upgrading the release:

    ```bash
    helm pull oci://ghcr.io/felukka/charts/koptan --version <new-version> --untar
    kubectl apply --server-side -f koptan/crds/
    helm upgrade koptan oci://ghcr.io/felukka/charts/koptan --version <new-version> -n koptan-system --reuse-values
    ```

### From 0.1.x to 0.2.0

Version 0.2.0 replaces the `koptan.felukka.sh/v1alpha` API (GoApp, JavaApp, DotnetApp, Slipway, Voyage) with `koptan.felukka.org/v1` (Service, CI, CD, CIPlugin, Alert, SelfService). Old resources are not migrated, and the admission webhooks are gone.

```bash
helm pull oci://ghcr.io/felukka/charts/koptan --version 0.2.0 --untar
kubectl apply --server-side -f koptan/crds/
helm upgrade koptan oci://ghcr.io/felukka/charts/koptan --version 0.2.0 -n koptan-system

# After you have exported anything you need from the old resources:
kubectl delete crd goapps.koptan.felukka.sh javaapps.koptan.felukka.sh \
  dotnetapps.koptan.felukka.sh slipways.koptan.felukka.sh voyages.koptan.felukka.sh
```

Then recreate each application as a [Service](../resources/service.md).

!!! note
    Chart 0.1.0 was published at `oci://ghcr.io/felukka/koptan`. That path is now the operator image. Charts live under `oci://ghcr.io/felukka/charts/`.

## Uninstall

```bash
helm uninstall koptan -n koptan-system
```

The CRDs, and therefore all Koptan resources and the applications they deployed, stay in the cluster. See [Installation → Uninstall](../getting-started/installation.md#uninstall) to remove them too.
