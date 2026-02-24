# FAQ

**Do I need a Dockerfile?**
No. Koptan generates one for Go, Rust, Java, .NET, Python, Ruby, PHP, Node.js and static sites. If your repository has a Dockerfile, Koptan uses it.

**Do I need a CI system such as GitHub Actions or Jenkins?**
Not for building and deploying. Koptan builds inside your cluster. You can keep your CI for tests, or run tests as a [CIPlugin](../resources/ciplugin.md).

**Does Koptan need webhooks from my git host?**
No. It polls every minute. To deploy faster, have your git host or a script set the [refresh annotation](../resources/service.md#trigger-a-check-right-away).

**Which git hosts work?**
Any git server reachable from the cluster: GitHub, GitLab, Bitbucket, Gitea, Azure Repos and others. Creating repositories for SelfServices supports GitHub (and GitHub Enterprise) and GitLab (including self-hosted).

**Which registries work?**
Any OCI registry that buildah can push to and your nodes can pull from.

**How are images tagged?**
`<registry>/<repo>:<first 12 characters of the commit SHA>`.

**Does it need privileged containers or Docker?**
No. Builds use rootless buildah.

**How do I roll back?**
Set `spec.source.revision` on the Service to an older tag or commit SHA. Set it back to the branch to follow new commits again.

**Can I deploy one app from a monorepo?**
Yes. Set `spec.build.contextDir`. Create one Service per app.

**How do I expose my app on the internet?**
Koptan creates a Kubernetes Service `<name>-cd` on port 80. Point your Ingress, Gateway route or load balancer at it. See [CD](../resources/cd.md#what-it-deploys).

**How do I set CPU and memory for my app?**
Edit `spec.resources` on the CD `<name>-cd`. Koptan keeps your values. See [CD → Resources](../resources/cd.md#resources).

**Can I use Koptan without the UI?**
Yes. Everything is a Kubernetes resource and works with `kubectl`, GitOps tools such as Argo CD and Flux, or Helm.

**Can I use my own Backstage?**
Yes. The Koptan plugins are Backstage plugins. Give your Backstage service account the role from [`backstage.rbac`](../helm/koptan.md#backstage-rbac).

**Which AI models work with SelfService?**
Claude through the Anthropic API, and any server with an OpenAI-compatible chat completions API: OpenAI, Azure OpenAI, OpenRouter, LiteLLM, Gemini's OpenAI endpoint, and local servers such as Ollama, vLLM and LM Studio. With a local model, prompts and code stay inside your cluster.

**What happens to my apps if I uninstall Koptan?**
`helm uninstall` removes the operator but keeps the CRDs and your resources, so the apps keep running (without new builds). Deleting the CRDs deletes every Koptan resource and the apps with them.

**Where is the API reference?**
Every resource page lists its fields. You can also ask the cluster:

```bash
kubectl explain services.koptan.felukka.org.spec --recursive
kubectl explain ciplugins.spec.codeql
```
