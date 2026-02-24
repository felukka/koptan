# Installation

Koptan has two parts:

| Part | Chart | Required |
|---|---|---|
| **Operator**: the controllers and the six CRDs | [`koptan`](../helm/koptan.md) | Yes |
| **UI**: the Backstage web app | [`koptan-ui`](../helm/koptan-ui.md) | No. You can do everything with `kubectl`. |

Both charts are published to the Felukka chart repository, both as OCI artifacts on GitHub Container Registry and as a classic HTTPS Helm repository.

## 1. Install the operator

=== "OCI registry (recommended)"

    ```bash
    helm install koptan oci://ghcr.io/felukka/charts/koptan \
      -n koptan-system --create-namespace
    ```

    To install a specific version, add `--version <chart-version>`.

=== "HTTPS repository"

    ```bash
    helm repo add felukka https://charts.felukka.org
    helm repo update
    helm install koptan felukka/koptan -n koptan-system --create-namespace
    ```

Check that the controller is running:

```bash
kubectl -n koptan-system get pods
kubectl get crds | grep koptan.felukka.org
```

You should see one `koptan-...` Pod in `Running` state and six CRDs: `services`, `cis`, `cds`, `ciplugins`, `alerts` and `selfservices`.

### Set your default registry

Services that set no `spec.image.registry` push to the operator's default registry, which is `docker.io` unless you change it:

```bash
helm upgrade koptan oci://ghcr.io/felukka/charts/koptan -n koptan-system \
  --reuse-values --set defaultRegistry=ghcr.io
```

## 2. Create the agent key (for SelfServices)

SelfServices need a shared key. The operator uses it to give each agent its own API token, and the UI uses the same key to talk to the agents. Without the key, everything else works, and SelfServices report `the operator has no agent key (KOPTAN_AGENT_KEY); SelfServices are disabled`.

```bash
kubectl -n koptan-system create secret generic koptan-agent-key \
  --from-literal=key=$(openssl rand -hex 32)

# The operator reads the key at start-up:
kubectl -n koptan-system rollout restart deployment koptan
```

The Secret name and key are set by `agent.keySecret` in the chart values.

## 3. Install the UI (optional)

!!! warning "The UI has no authentication yet"
    The UI uses Backstage guest sign-in and allows every action. Anyone who can reach it can deploy services and prompt agents. Reach it with `kubectl port-forward`, or put it behind your own authentication (an OAuth2 proxy, a VPN, or similar). See [Security](../configuration/security.md).

Install it into the operator namespace, so it can read the `koptan-agent-key` Secret:

=== "OCI registry"

    ```bash
    helm install koptan-ui oci://ghcr.io/felukka/charts/koptan-ui -n koptan-system
    ```

=== "HTTPS repository"

    ```bash
    helm install koptan-ui felukka/koptan-ui -n koptan-system
    ```

Open it:

```bash
kubectl -n koptan-system port-forward svc/koptan-ui 7007:7007
# then open http://localhost:7007
```

See [koptan-ui chart](../helm/koptan-ui.md) for an Ingress, PostgreSQL and other settings.

!!! info "The UI chart is new"
    If `helm install` cannot find `koptan-ui`, the chart has not been published to your channel yet. Run the UI with the [Backstage RBAC from the operator chart](../helm/koptan.md#backstage-rbac) in the meantime, or use `kubectl`.

## 4. Deploy something

Continue with the [Quickstart](quickstart.md).

## Other ways to install

### From source with kustomize

From a clone of [felukka/koptan](https://github.com/felukka/koptan), with Go 1.24 or newer:

```bash
make install                              # CRDs only
make deploy IMG=ghcr.io/felukka/koptan:<version>
```

To build your own operator image:

```bash
make docker-build docker-push IMG=<registry>/koptan:<tag>
make deploy IMG=<registry>/koptan:<tag>
```

### A single YAML bundle

```bash
make build-installer IMG=ghcr.io/felukka/koptan:<version>
kubectl apply -f dist/install.yaml
```

### Running the operator locally

For development, run the controllers on your machine against the current kubeconfig:

```bash
make install   # CRDs
make run
```

## Uninstall

```bash
helm uninstall koptan-ui -n koptan-system   # if installed
helm uninstall koptan -n koptan-system
```

Helm does **not** delete CRDs. Deleting them deletes every Koptan resource in the cluster, and with them the Deployments Koptan created. Only do this if that is what you want:

```bash
kubectl delete crd services.koptan.felukka.org cis.koptan.felukka.org cds.koptan.felukka.org \
  ciplugins.koptan.felukka.org alerts.koptan.felukka.org selfservices.koptan.felukka.org
```
