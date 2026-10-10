# Helm chart: `koptan-ui`

The `koptan-ui` chart installs the [Koptan UI](../ui/overview.md): a Backstage app with the Services, Self Service, Security and Alerting pages. It talks to the Kubernetes API with its own service account and needs the [`koptan` operator](koptan.md) installed.

| | |
|---|---|
| Chart | `koptan-ui` (chart 0.1.0, app 0.2.0) |
| OCI | `oci://ghcr.io/felukka/charts/koptan-ui` |
| HTTPS repository | `https://charts.felukka.org` (`felukka/koptan-ui`) |
| Image | `ghcr.io/felukka/koptan-ui` (linux/amd64, linux/arm64) |
| Port | 7007 |

!!! info "New chart"
    `koptan-ui` is newer than the operator chart. If Helm cannot find it yet, it has not been published to the repository you use.

!!! danger "No authentication yet"
    The UI uses Backstage guest sign-in and an allow-all permission policy. **Anyone who can reach it can deploy services and prompt SelfService agents.** The chart therefore ships with the Ingress **disabled**. Reach the UI with `kubectl port-forward`, or put an Ingress behind your own authentication (an OAuth2 proxy, a VPN, or similar). See [Security](../configuration/security.md).

## Install

Install into the operator namespace, so the UI can read the operator's `koptan-agent-key` Secret:

=== "OCI registry"

    ```bash
    helm install koptan-ui oci://ghcr.io/felukka/charts/koptan-ui -n koptan-system
    ```

=== "HTTPS repository"

    ```bash
    helm repo add felukka https://charts.felukka.org
    helm repo update
    helm install koptan-ui felukka/koptan-ui -n koptan-system
    ```

Open it:

```bash
kubectl -n koptan-system port-forward svc/koptan-ui 7007:7007
# open http://localhost:7007
```

## Values

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `1` | UI replicas. Use PostgreSQL if you run more than one. |
| `image.repository` | `ghcr.io/felukka/koptan-ui` | UI image. |
| `image.tag` | `""` | Empty means the chart's `appVersion`. |
| `image.pullPolicy` | `IfNotPresent` | Pull policy. |
| `imagePullSecrets` | `[]` | Pull Secrets. |
| `nameOverride` / `fullnameOverride` | `""` | Resource naming. |
| `baseUrl` | `http://localhost:7007` | The URL browsers use to reach the UI. Sets Backstage `app.baseUrl`, `backend.baseUrl` and the CORS origin. **Set it to the Ingress URL when you enable one.** |
| `koptan.namespace` | `""` | Limit the UI to one namespace. Empty means all namespaces. |
| `agentKey.secret.name` | `koptan-agent-key` | Secret with the operator's agent key. Without it, the Self Service page cannot reach agents. |
| `agentKey.secret.key` | `key` | Key inside that Secret. |
| `database.postgres.enabled` | `false` | Use PostgreSQL. Without it, the UI uses in-memory SQLite. Koptan data lives in the cluster, so only UI state (settings, notifications) is lost on restart. |
| `database.postgres.host` | `""` | PostgreSQL host. Required when enabled. |
| `database.postgres.port` | `5432` | PostgreSQL port. |
| `database.postgres.user` | `backstage` | PostgreSQL user. |
| `database.postgres.existingSecret` | `""` | Secret holding the password. |
| `database.postgres.passwordKey` | `password` | Key of the password in that Secret. |
| `service.type` | `ClusterIP` | Kubernetes Service type. |
| `service.port` | `7007` | Service port. |
| `ingress.enabled` | `false` | Create an Ingress. See the warning above. |
| `ingress.className` | `""` | Ingress class. |
| `ingress.annotations` | `{}` | Ingress annotations, for example for an auth proxy. |
| `ingress.hosts` | `koptan.example.com`, path `/` | Hosts and paths. |
| `ingress.tls` | `[]` | TLS configuration. |
| `rbac.create` | `true` | ClusterRole and binding for the UI's service account. |
| `serviceAccount.create` | `true` | Create the service account. |
| `serviceAccount.annotations` | `{}` | Service account annotations. |
| `serviceAccount.name` | `""` | Service account name. Empty means generated. |
| `auth.allowGuest` | `true` | Backstage refuses guest sign-in in a production build unless this is set; without it nobody can sign in. Set it to `false` once `appConfig` configures a real sign-in provider. |
| `appConfig` | `{}` | Extra Backstage configuration, loaded last. Use it for an auth provider or any other Backstage setting. |
| `extraEnv` | `[]` | Extra environment variables. |
| `resources` | requests `100m` / `512Mi`, limit `1Gi` memory | UI resources. |
| `podAnnotations` / `podLabels` | `{}` | Pod metadata. |
| `nodeSelector` / `tolerations` / `affinity` | `{}` / `[]` / `{}` | Scheduling. |

## Examples

### PostgreSQL

```bash
kubectl -n koptan-system create secret generic koptan-ui-postgres --from-literal=password=<password>
```

```yaml
database:
  postgres:
    enabled: true
    host: postgres.example.svc
    user: backstage
    existingSecret: koptan-ui-postgres   # key "password"
```

### Ingress

Only do this behind authentication.

```yaml
baseUrl: https://koptan.example.com
ingress:
  enabled: true
  className: nginx
  annotations:
    # For example, with oauth2-proxy in front:
    nginx.ingress.kubernetes.io/auth-url: "https://oauth2.example.com/oauth2/auth"
    nginx.ingress.kubernetes.io/auth-signin: "https://oauth2.example.com/oauth2/start?rd=$escaped_request_uri"
  hosts:
    - host: koptan.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: koptan-ui-tls
      hosts: [koptan.example.com]
```

### One namespace only

```yaml
koptan:
  namespace: team-payments
```

### A different agent key Secret

If you install the UI outside the operator namespace, copy the key to a Secret there and point the chart at it:

```yaml
agentKey:
  secret:
    name: koptan-agent-key-copy
    key: key
```

The value must be the same as the operator's key.

## What the chart installs

| Kind | Purpose |
|---|---|
| Deployment | The Backstage app, running as UID 1000. It loads the chart's app-config, not the image's production config. |
| Service | Port 7007. |
| ConfigMap `<release>-app-config` | Base URLs, database, an in-cluster `kubernetes` cluster using the service account, the Koptan settings, and your `appConfig`. |
| ServiceAccount, ClusterRole, ClusterRoleBinding | Lets the UI list Koptan resources, create Services, Alerts and SelfServices and their Secrets, patch Services, reach agents through `services/proxy` and read nodes. |
| Ingress | Only with `ingress.enabled`. |

## Uninstall

```bash
helm uninstall koptan-ui -n koptan-system
```

Koptan resources created from the UI stay in the cluster.
