# Security

This page explains how Koptan handles credentials and untrusted input, and what you should do to run it safely.

## Do this first

1. **Keep the UI private.** It has no real authentication yet (guest sign-in, every action allowed). Use `kubectl port-forward`, or put it behind an authenticating proxy or a VPN. Do not expose it with a plain Ingress.
2. **Use namespaces as the security boundary.** Anyone who can create a Koptan Service in a namespace can run code (the build and the application) in that namespace. Give teams their own namespaces and control who can create Koptan resources with normal Kubernetes RBAC.
3. **Use least-privilege tokens.** Read-only git tokens for Services, push-only registry credentials, a SonarQube analysis token, a GitHub token with only `security_events` for CodeQL uploads.
4. **Pin images** in production (`agent.image`, `controller.image.tag`) instead of `latest`.

## Credentials

Koptan never takes credentials inline in a resource. Every credential is a reference to a Secret in the same namespace:

| Credential | Referenced from |
|---|---|
| Git token (private repos) | `Service.spec.source.secretRef` |
| Registry login | `Service.spec.image.credentialsSecret` (dockerconfigjson) |
| SonarQube token | `CIPlugin.spec.sonarqube.tokenSecretRef` |
| GitHub upload token | `CIPlugin.spec.codeql.github.tokenSecretRef` |
| Slack / Teams / webhook URLs, signing keys | `Alert.spec.channels[].urlSecretRef`, `signingSecretRef` |
| Git token, AI API key | `SelfService.spec.repo.*`, `spec.ai.apiKeySecretRef` |
| Agent key | Secret `koptan-agent-key` in the operator namespace |

The UI follows the same rule: tokens you type in a form are written to Secrets owned by the created resource and never returned to the browser.

In build Pods, the git token is only given to the clone step, and registry credentials only to the build step.

## Builds

- Builds run in ordinary Pods in the Service's namespace, with rootless buildah. No privileged containers, no Docker socket.
- Repository content, commit SHAs, image names and paths reach build scripts only as environment variables, never pasted into shell commands.
- Discovery reads the repository through a sandbox: paths that leave the repository (for example through symlinks or `..`) are refused, and file sizes are capped.
- Repository URLs, revisions and paths are validated before anything is cloned.

## Generated images

Generated Dockerfiles run the application as a non-root user, with multi-stage builds so compilers and package caches are not shipped. A default `.dockerignore` keeps `.git` and `.env` files out of the image.

## Alerts

- Webhook URLs come only from Secrets.
- Slack and Teams require `https`.
- Koptan refuses to send to loopback, link-local (cloud metadata) and unspecified addresses, checked after DNS resolution and on redirects. This stops an Alert from being used to reach the node or the cloud metadata service.
- Use `signingSecretRef` on webhooks and verify `X-Koptan-Signature` in your receiver. See [Alert](../resources/alert.md#webhook).

## SelfService agents

- Each agent runs as its own Deployment: non-root, read-only root filesystem, all capabilities dropped, **no Kubernetes service account token**.
- Each agent has its own API token, derived from the agent key and the SelfService's namespace and name. One agent's token does not work for another.
- The agent API is only reachable inside the cluster (through the Kubernetes API server's service proxy, which the UI uses).
- File tools are confined to the repository. Shell commands are off unless you set `allowCommands: true`.
- The agent can change your repository, and every change is deployed. Treat access to the Self Service page like push access to the repository.

Rotate the agent key by replacing the Secret and restarting the operator and the UI. The operator then writes new tokens to the agents' Secrets; restart the agent Deployments (`kubectl -n <namespace> rollout restart deployment -l app.kubernetes.io/name=koptan-agent`) so they pick them up.

## The operator

- Runs as non-root with a read-only root filesystem, `RuntimeDefault` seccomp and all capabilities dropped. Repositories are cloned to an `emptyDir` at `/tmp`.
- Needs cluster-wide read access to Secrets, because the credentials it uses live in your application namespaces.
- Metrics are off by default. When on, they are served over HTTPS with Kubernetes authentication and authorization.

## Known limitations

- The UI has guest sign-in and an allow-all permission policy.
- SelfService repositories must be https.
- Agent run history is kept in memory only.
