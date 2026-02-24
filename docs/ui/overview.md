# Koptan UI

The Koptan UI is a web app, built on [Backstage](https://backstage.io), for everything Koptan does. You can create and follow Services, run AI self-service sessions, see CI plugin results and manage alerts without writing YAML.

Everything the UI shows comes from the Koptan resources in the cluster, and everything it creates is an ordinary Koptan resource. You can mix the UI and `kubectl` freely.

## Opening the UI

Install it with the [`koptan-ui` chart](../helm/koptan-ui.md), then:

```bash
kubectl -n koptan-system port-forward svc/koptan-ui 7007:7007
```

Open <http://localhost:7007> and sign in as **guest**.

!!! danger
    Guest sign-in means anyone who can open the UI can deploy and prompt agents. Keep it behind port-forward or your own authentication. See [Security](../configuration/security.md).

The sidebar has **Search**, the **Menu** with the Koptan pages, **Notifications**, **Sign out** and **Settings** (theme and profile).

## Services

**Menu → Services** (`/service`) lists every Koptan Service as a pipeline: **Service → CI build → Deployment**.

For each pipeline you see:

- **Service:** source repository, revision, detected language, and whether the Dockerfile is *Generated* or *From repository*.
- **CI build:** phase, latest commit, image and the plugin steps of the last build.
- **Deployment:** phase, active revision, last image and available replicas.

The header shows how many Services are running (**Active Service**).

### New Deployment

Choose **New Deployment** (`/service/new`) to create a Service.

| Field | Maps to | Notes |
|---|---|---|
| Service name | `metadata.name` | Also the default image repository. |
| Namespace | `metadata.namespace` | Default `default`. |
| Source repository | `spec.source.repo` | |
| Revision | `spec.source.revision` | Empty means the default branch. |
| Access token (private repos) | Secret `<name>-git` → `spec.source.secretRef` | Leave empty for public repositories. |
| Build context (monorepos) | `spec.build.contextDir` | |
| Environment variables | `spec.env` | Plain values. |
| Registry | `spec.image.registry` | Empty means the operator default. |
| Image repository | `spec.image.repo` | Empty means the Service name. |
| Registry username / password | dockerconfigjson Secret `<name>-registry` → `spec.image.credentialsSecret` | |
| Replicas | `spec.replicas` | 0–50 in the form. |
| Container port | `spec.port` | Default 8080. |
| CI plugins | `spec.plugins` | Pick from the CIPlugins in the namespace. |

The UI validates the repository and revision the same way the operator does before creating anything. The Secrets it creates are owned by the Service and deleted with it.

## Self Service

**Menu → Self Service** (`/self-service`) lists your [SelfService](../resources/selfservice.md) sessions.

### New Session

Choose **New Session** (`/self-service/new`):

| Field | Notes |
|---|---|
| Name, Namespace | Name of the SelfService (and of the Service it deploys). |
| Repository | **Create** a new one (Git host GitHub or GitLab, Owner, optional API URL for GitHub Enterprise or self-hosted GitLab) or use an **existing** one (Clone URL, https only). |
| Token (create and push) | Stored in Secret `<name>-git`. |
| Provider | *Anthropic (Claude)* or *OpenAI-compatible*. |
| Model | For example `claude-opus-5-5` or `qwen2.5-coder:14b`. |
| Base URL | OpenAI-compatible servers only. |
| API key (not needed for local servers) | Stored in Secret `<name>-ai`. |
| Container port | Port of the application the agent will write. |
| Agent commands | Allow the agent to run shell commands (tests, formatters). |

### Session page

Open a session (`/self-service/<namespace>/<name>`) to see its repository, model and pipeline, and to talk to the agent:

1. Type what you want in **Ask the agent**, for example *"Create a FastAPI service with a /orders endpoint backed by an in-memory list"*.
2. The agent's log streams live while it works.
3. When it pushes, Koptan starts a build right away. Follow it in the pipeline on the same page.

**Runs** lists earlier prompts and their results. The run history lives in the agent pod and is lost when the pod restarts; the commits in git are the durable record.

The Self Service page needs the UI and the operator to share the same [agent key](../getting-started/installation.md#2-create-the-agent-key-for-selfservices).

## Security

**Menu → Security** (`/security`) shows the [CIPlugins](../resources/ciplugin.md):

- **Plugins:** each plugin's type, order, which Services it attaches to and whether it is valid.
- **Runs / Latest results:** the outcome of each plugin step in the latest builds.

Create plugins with `kubectl`; this page is read-only.

## Alerting

**Menu → Alerting** (`/alert`) shows your [Alerts](../resources/alert.md), with **Recently delivered** and **Recently failed** notifications and a feed of **Recent signals**.

### New Alert

| Field | Notes |
|---|---|
| Alert name, Namespace | |
| Service | One Service by name… |
| Or Services with labels | …or a label selector, such as `team=payments`. |
| Events (none ticked = all) | Push, CI and CD events. |
| Channels | Name, type (Slack, Teams, Webhook), webhook URL and, for webhooks, an optional signing key. |

The URLs and signing keys go into a Secret named `<alert>-channels`, owned by the Alert.

## Home

The home page holds widgets, such as **Cluster Health**. Some widgets and the *Infrastructure* and *Monitoring* pages are placeholders for upcoming features.

## Permissions

The UI acts in the cluster with its own service account, not with the signed-in user's rights. What it can do is set by the ClusterRole the chart creates: list all Koptan resources, create Services, Alerts and SelfServices with their Secrets, patch Services and reach agents. Use `koptan.namespace` in the chart to limit it to one namespace.
