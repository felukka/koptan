# Operator configuration

The Helm chart sets everything on this page for you. Use this page when you need a setting the chart does not cover (add it with `extraEnv`), or when you run the operator another way.

## Environment variables

| Variable | Flag | Default | Chart value | Description |
|---|---|---|---|---|
| `KOPTAN_DEFAULT_REGISTRY` | `--default-registry` | `docker.io` | `defaultRegistry` | Registry for Services that set no `spec.image.registry`. |
| `KOPTAN_AGENT_IMAGE` | `--agent-image` | `ghcr.io/felukka/koptan-agent:latest` | `agent.image` | Image of SelfService agent pods. A SelfService can override it with `spec.agentImage`. |
| `KOPTAN_AGENT_KEY` | — | — | `agent.keySecret` | Key that derives each agent's API token. Read from the optional Secret `koptan-agent-key`. Without it, SelfServices are disabled. The Koptan UI must use the same value. |

A flag wins over its environment variable.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--default-registry` | `$KOPTAN_DEFAULT_REGISTRY` or `docker.io` | See above. |
| `--agent-image` | `$KOPTAN_AGENT_IMAGE` or `ghcr.io/felukka/koptan-agent:latest` | See above. |
| `--leader-elect` | `false` (the chart sets it) | Leader election, so only one replica works at a time. |
| `--health-probe-bind-address` | `:8081` | `/healthz` and `/readyz`. |
| `--metrics-bind-address` | `0` (off) | Metrics address, for example `:8443`. The chart sets it when `metrics.secure` is on. |
| `--metrics-secure` | `true` | Serve metrics over HTTPS with authentication and authorization. |
| `--metrics-cert-path`, `--metrics-cert-name`, `--metrics-cert-key` | `""`, `tls.crt`, `tls.key` | Certificate for the metrics server. Without it a self-signed certificate is generated. |
| `--enable-http2` | `false` | HTTP/2 for the metrics and webhook servers. Off by default for security reasons. |
| `--webhook-cert-path`, `--webhook-cert-name`, `--webhook-cert-key` | `""`, `tls.crt`, `tls.key` | Reserved; Koptan has no admission webhooks at the moment. |

Standard controller-runtime logging flags (`--zap-log-level`, `--zap-devel`, `--zap-encoder`, ...) are also accepted. The chart does not expose them as values; add them to the Deployment's `args` if you need debug logs (`--zap-log-level=debug`).

## Fixed behaviour

These are built in and not configurable:

| Setting | Value |
|---|---|
| Git poll interval for `Ready` Services | 1 minute |
| Retry interval for `Failed` Services | 1 minute |
| Retry interval for failed alert deliveries | 30 seconds |
| Default application port | 8080 |
| Default replicas | 1 |
| Default application resources (on new CDs) | 100m / 128Mi requested, 500m / 256Mi limit |
| Clone image | `alpine/git:2.47.2` |
| Build image | `quay.io/buildah/stable:v1.43.0` |
| SonarQube scanner image (default) | `sonarsource/sonar-scanner-cli:11` |
| CodeQL image (default) | `debian:bookworm-slim` |

If your cluster cannot reach Docker Hub or Quay, mirror these images and use a registry mirror on your nodes.

## Proxies and custom CAs

The operator, build Pods and agents reach your git host and registries over the network. Behind a proxy, set it for the operator with `extraEnv`:

```yaml
extraEnv:
  - name: HTTPS_PROXY
    value: http://proxy.corp:3128
  - name: NO_PROXY
    value: .svc,.cluster.local,10.0.0.0/8
```

For a private CA, mount the bundle with `volumes` / `volumeMounts` and point `SSL_CERT_FILE` at it.

## RBAC

The operator's ClusterRole lets it:

- manage all Koptan resources and their status;
- create and update Deployments, Services and ConfigMaps;
- create and delete build Pods;
- read Secrets (git tokens, registry credentials, webhook URLs, API keys) and create the Secrets it manages (for example `<selfservice>-agent`);
- record events.

It runs cluster-wide and acts in every namespace that has Koptan resources.

## Health and metrics

- Liveness: `GET :8081/healthz`. Readiness: `GET :8081/readyz`.
- Metrics: standard controller-runtime metrics (reconcile counts and durations, work queue depth) on `metrics.port` when enabled. See [the chart](../helm/koptan.md#metrics).
