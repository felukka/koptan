# Troubleshooting

## Where to look

Start at the Service and follow the chain:

```bash
kubectl get ksvc,cis,cds -n <ns>
kubectl describe ksvc <name> -n <ns>         # status.message, conditions, events
kubectl describe ci <name>-ci -n <ns>
kubectl describe cd <name>-cd -n <ns>
```

Build logs:

```bash
POD=$(kubectl -n <ns> get ci <name>-ci -o jsonpath='{.status.buildPod}')
kubectl -n <ns> logs "$POD" --all-containers --prefix
```

Operator logs:

```bash
kubectl -n koptan-system logs deployment/koptan
```

## Service is `Failed`

The Service retries by itself every minute. After fixing the cause, add the refresh annotation to retry now:

```bash
kubectl -n <ns> annotate ksvc <name> koptan.felukka.org/refresh="$(date +%s)" --overwrite
```

| Message | Cause | Fix |
|---|---|---|
| `no Dockerfile and no supported stack found in <dir> (supported: ...)` | No Dockerfile, and none of the [supported stacks](../resources/service.md#supported-stacks) was detected. | Add a Dockerfile, set `spec.build.language`, or check `spec.build.contextDir` points at the app. |
| `spec.build.dockerfilePath "<path>": ...` | The Dockerfile you named does not exist. | The path is relative to the **repository root**, not the context. |
| `spec.build.contextDir "<path>": ... is not a directory` | Wrong monorepo path. | Fix `contextDir`. |
| `branch "<x>" not found in <repo>` / `tag ... not found` / `revision ... not found` | The revision does not exist. | Check the spelling, or push the branch. |
| `<repo> has no default branch (HEAD)` | Empty repository with no revision. | Push a first commit, or set `revision`. |
| `repository URL "<url>" must use https` / `scheme ... is not allowed` / `needs a host and a path` | Invalid URL. | Use a normal clone URL. |
| `read git token secret "<name>": ...` / `secret "<name>" has no key "<key>"` | The Secret in `source.secretRef` is missing or has another key. | Create it in the Service's namespace with the right key. |
| `CIPlugin <name> not found in namespace <ns>` | `spec.plugins` names a plugin that does not exist. | Create the plugin, or remove it from the list. |
| `CIPlugin <name> is invalid: ...` | The plugin is not `Accepted`. | `kubectl describe cip <name>` and fix it. |
| `no FROM instruction in the Dockerfile` / `empty Dockerfile` / `the Dockerfile is larger than ...` | Your Dockerfile is not usable. | Fix the Dockerfile. |
| `registry credentials secret "<name>": ...` / `secret "<name>" has no .dockerconfigjson key` | `image.credentialsSecret` is missing or not of type `kubernetes.io/dockerconfigjson`. | Create it with `kubectl create secret docker-registry`. |

## CI is `Failed`

`status.message` holds the end of the failing container's log.

| Symptom | Likely cause | Fix |
|---|---|---|
| Failing in `clone` with authentication errors | Private repository without a token, or a token without read access. | Set `source.secretRef` with a valid token. Tokens are used for `https` URLs. |
| Failing in `plugin-<name>` | The plugin found a problem (that is its job), or it is misconfigured. | Read `kubectl logs <pod> -c plugin-<name>`. Use `failurePolicy: Ignore` to report without blocking. |
| Failing in `build-push` during `RUN` steps | The application does not build (missing dependency, failing compile). | Build locally with the same Dockerfile: `kubectl get cm <svc>-dockerfile-... -o jsonpath='{.data.Dockerfile}' > Dockerfile`. |
| `unauthorized` / `denied` on push | Wrong or missing registry credentials, or the repository does not allow pushes. | Check `image.credentialsSecret`, the registry host and `image.repo`. On GHCR the token needs `write:packages`. |
| Build Pod `Pending` | Not enough CPU or memory in the namespace, or a ResourceQuota. | `kubectl describe pod <pod>`. |
| Build Pod `OOMKilled` | Large builds (Java, Rust, Node) need memory. | Give the namespace more room; for plugins, raise `resources`. |

## CD is `Failed` or stuck in `Deploying`

```bash
kubectl -n <ns> get pods -l koptan.felukka.org/cd=<name>-cd
kubectl -n <ns> describe deployment <name>-cd
kubectl -n <ns> logs deployment/<name>-cd
```

| Symptom | Cause | Fix |
|---|---|---|
| `ImagePullBackOff` | The cluster cannot pull the image. | Set `image.credentialsSecret` (it is also the pull Secret), or make the image public. |
| Pods `Running` but never `Ready` | Nothing listens on `PORT`. | Make the app listen on `$PORT`, or set `spec.port` to the port it uses. |
| `CrashLoopBackOff` | The application exits. | Check the logs; often a missing environment variable — add it to `spec.env`. |

## No new builds after a push

- The Service must be `Ready` for polling. A `Failed` Service retries every minute.
- `revision` set to a tag or SHA pins the Service. Use a branch to follow new commits.
- Polling runs every minute. Use the refresh annotation to check at once.
- The CI builds a commit only once. To rebuild the same commit, change something in the CI's inputs (for example, edit an attached plugin) or push a new commit.

## Alert sends nothing

- An Alert only reports what happens **after** it is created.
- Check `kubectl get alert <name> -o yaml`: `status.deliveries` lists every attempt with its error.
- `the slack URL must use https`: Slack and Teams need https URLs.
- `destination address is not allowed`: the URL resolves to a loopback or link-local address.
- `spec.suspend: true` stops sending.
- `serviceRef` and `selector` only match Services in the Alert's namespace.

## SelfService problems

| Message | Fix |
|---|---|
| `the operator has no agent key (KOPTAN_AGENT_KEY); SelfServices are disabled` | Create the `koptan-agent-key` Secret and restart the operator. See [Installation](../getting-started/installation.md#2-create-the-agent-key-for-selfservices). |
| `create repository on github: ...` | The token cannot create repositories under `owner`, or the name is taken. |
| `a Service named <x> already exists and does not belong to this SelfService` | Rename the SelfService, or delete the existing Service. |
| The UI cannot send prompts (401 / unauthorized) | The UI and the operator have different agent keys. Use the same Secret for both. |
| The agent answers but changes nothing | The model has no tool calling and plan mode failed, or the model is too small. Try a larger code model. |

## Getting help

Open an issue at [github.com/felukka/koptan](https://github.com/felukka/koptan/issues) with the output of `kubectl describe` for the failing resource and the operator logs. Remove tokens and URLs first.
