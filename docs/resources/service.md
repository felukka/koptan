# Service

A **Service** is the only resource you need to deploy an application. It points at a git repository. Koptan builds that repository into an image and runs it, and does so again for every new commit.

- **API:** `koptan.felukka.org/v1`, kind `Service`
- **Short name:** `ksvc` (`kubectl get ksvc`)
- **Creates:** a Dockerfile ConfigMap, a [CI](ci.md) named `<name>-ci`, and, after the first successful build, a [CD](cd.md) named `<name>-cd`

!!! note "Not the same as a Kubernetes Service"
    A Koptan Service (`services.koptan.felukka.org`) is a different resource from a core Kubernetes `Service`. `kubectl get services` shows the core ones. Use `kubectl get ksvc` for Koptan.

## Example

```yaml
apiVersion: koptan.felukka.org/v1
kind: Service
metadata:
  name: example-service
  namespace: default
spec:
  source:
    repo: "https://github.com/example/repo.git"
    # Branch, tag or commit SHA; omit for the default branch.
    revision: "main"
  # Optional. Defaults: the operator's --default-registry and the Service name.
  image:
    registry: "ghcr.io"
    repo: "example/repo"
    # A kubernetes.io/dockerconfigjson Secret, used to push and to pull.
    credentialsSecret: "example-registry"
  replicas: 2
  port: 8080
  env:
    - name: LOG_LEVEL
      value: info
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef: {name: example-db, key: url}
  # CIPlugins that run after checkout and before build/push.
  plugins:
    - name: trivy-fs
  build:
    # Build context inside the repository, for monorepos.
    contextDir: "services/api"
    # An explicit Dockerfile, relative to the repository root.
    # dockerfilePath: "services/api/Dockerfile.prod"
    # Skip detection and generate the Dockerfile for this stack.
    # language: "python"
```

The smallest valid Service only has `spec.source.repo`:

```yaml
apiVersion: koptan.felukka.org/v1
kind: Service
metadata:
  name: hello
spec:
  source:
    repo: https://github.com/example/hello.git
```

It builds the default branch, pushes to `<default-registry>/hello`, and runs one replica on port 8080.

## What happens when you create a Service

1. **Validate.** Koptan checks the repository URL, the revision and the build paths.
2. **Resolve.** Koptan resolves the revision (branch, tag or default branch) to a commit SHA.
3. **Discover.** Koptan clones the repository at that SHA and looks for a Dockerfile (see [Dockerfile discovery](#dockerfile-discovery)). If there is none, it detects the stack and generates one.
4. **Store.** Koptan saves the Dockerfile, and a default `.dockerignore`, in a ConfigMap named `<name>-dockerfile-<hash>`.
5. **Build.** Koptan creates or updates the CI `<name>-ci` with the commit and the attached [CIPlugins](ciplugin.md). The CI builds and pushes the image.
6. **Deploy.** When the build succeeds, the CI creates the CD `<name>-cd`, which deploys the image.
7. **Watch.** Once the Service is `Ready`, Koptan checks the repository **every minute**. A new commit starts again at step 3.

Changing the Service's spec also starts the cycle again. Changes to `replicas` and `env` go straight to the CD and roll out without a rebuild. A change to `port` with a generated Dockerfile also rebuilds the image, because the port is written into the Dockerfile.

### Trigger a check right away

Any update to the Service makes Koptan check the repository at once. The usual way is the refresh annotation:

```bash
kubectl annotate ksvc <name> koptan.felukka.org/refresh="$(date +%s)" --overwrite
```

You can call this from your git host's webhook, or from a script after `git push`, so that deployments start without waiting for the next poll.

## Spec reference

### `spec`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `source` | [Source](#source) | **Yes** | — | The git repository to build. |
| `image` | [ImageSpec](#image) | No | see below | Where the built image is pushed. |
| `replicas` | integer, ≥ 0 | No | `1` | Number of Pods to run. `0` keeps the Service but stops the application. |
| `port` | integer, 1–65535 | No | `8080` | Port your application listens on. Koptan also exports it as the `PORT` environment variable. |
| `env` | list of [EnvVar](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/) | No | — | Environment variables for the application. Supports `value` and `valueFrom` (Secrets, ConfigMaps, field refs). |
| `build` | [BuildSpec](#build) | No | — | Tunes how the image is built. |
| `plugins` | list of `{name}` | No | — | [CIPlugins](ciplugin.md) in the same namespace that run before every build. Max 32. Plugins can also attach themselves to the Service. |

### `source`

| Field | Type | Required | Description |
|---|---|---|---|
| `repo` | string | **Yes** | Clone URL. Schemes `https`, `http`, `ssh` and `git` are accepted. It needs a host and a path. |
| `revision` | string | No | Branch, tag or full commit SHA. Empty means the repository's default branch. Max 250 characters, letters, digits, `.`, `_`, `/` and `-`. |
| `secretRef` | `{name, key}` | No | Secret key holding a token for private repositories. The Secret must be in the same namespace. |

When `revision` is a branch, Koptan follows it and builds every new commit. When it is a tag or a SHA, the Service stays on that commit.

### `image`

| Field | Type | Default | Description |
|---|---|---|---|
| `registry` | string | The operator's default registry (`docker.io` unless changed with the [chart value `defaultRegistry`](../helm/koptan.md#values)) | Registry host, such as `ghcr.io`. |
| `repo` | string | The Service name | Repository path inside the registry, such as `my-org/my-app`. |
| `credentialsSecret` | string | — | Name of a `kubernetes.io/dockerconfigjson` Secret in the same namespace. It is used to push the image and, on the Deployment, to pull it. |

Images are tagged with the first 12 characters of the commit SHA, for example `ghcr.io/my-org/my-app:3f9c2a1b7d4e`.

### `build`

| Field | Type | Description |
|---|---|---|
| `contextDir` | string | Build context, relative to the repository root. Use it for a service inside a monorepo, for example `services/api`. Empty means the root. Detection and the Dockerfile search both run in this directory. |
| `dockerfilePath` | string | Your Dockerfile, relative to the **repository root**, for example `services/api/Dockerfile.prod`. The file must exist. |
| `language` | enum | Skip detection and generate the Dockerfile for this stack: `go`, `rust`, `java`, `dotnet`, `python`, `ruby`, `php`, `node` or `static`. |

Paths must be relative, use only letters, digits, `.`, `_` and `-` in each segment, and must not contain `.` or `..` segments.

## Dockerfile discovery

Koptan picks the Dockerfile in this order:

1. `spec.build.dockerfilePath`, if set.
2. The first of these files in the build context: `Dockerfile`, `Containerfile`, `dockerfile`, `docker/Dockerfile`, `build/Dockerfile`, `deploy/Dockerfile`.
3. A Dockerfile generated for `spec.build.language`, if set.
4. A Dockerfile generated for the detected stack.

If nothing matches, the Service fails with `no Dockerfile and no supported stack found in <dir> (supported: ...)`. Add a Dockerfile, or set `spec.build.language`.

`status.dockerfileSource` tells you which one was used: `repo` for your own file, `template` for a generated one.

### Supported stacks

Detection tries the stacks in this order and takes the first that matches. More specific stacks come first, because Rails, Django and Laravel apps often also have a `package.json` for their assets.

| Stack | Detected by | Default version | Version read from | Notes |
|---|---|---|---|---|
| **Go** | `go.mod` | 1.24 | `go` directive in `go.mod` | Finds the main package. Static binary on `distroless/static:nonroot`. |
| **Rust** | `Cargo.toml` | 1 (latest stable) | `rust-toolchain.toml` / `rust-toolchain` | Runs the package or `[[bin]]` binary on `debian:bookworm-slim`. |
| **Java** | `pom.xml` (Maven) or `build.gradle` / `build.gradle.kts` (Gradle) | 21 | `pom.xml` / Gradle Java version | Uses `mvnw` / `gradlew` when present. Runs `app.jar` on `eclipse-temurin` JRE. Tests are skipped during the build. |
| **.NET** | `*.csproj` (or a `*.sln`) | 8.0 | `<TargetFramework>` | `dotnet publish`, runs on `mcr.microsoft.com/dotnet/aspnet`. |
| **Python** | `uv.lock`, `poetry.lock` / `[tool.poetry]`, `Pipfile`, `requirements.txt`, `pyproject.toml` or `setup.py` | 3.12 | `.python-version`, `requires-python` | Package manager: uv, poetry, pipenv or pip. Django and Flask run under gunicorn, FastAPI under uvicorn. Otherwise `python main.py` (or `app.py`, `server.py`, `src/main.py`). |
| **Ruby** | `Gemfile` | 3.3 | `.ruby-version`, `ruby` line in `Gemfile` | Rails and Rack apps via `config.ru`. Otherwise `bundle exec ruby app.rb` (or `main.rb`, `server.rb`). |
| **PHP** | `composer.json`, `index.php` or `public/index.php` | 8.3 | `composer.json` | Apache image. Laravel detected through `artisan`. |
| **Node.js** | `package.json` | 22 | `.nvmrc`, `.node-version`, `engines.node` | npm, pnpm, yarn or bun, chosen by lockfile. Runs `build` if the script exists. Starts with the `start` script, or `main`, or `server.js` / `index.js` / `app.js` / `main.js` / `dist/index.js`. |
| **Static site** | `index.html` in `.`, `public`, `site`, `www` or `html` | nginx 1.27 | — | Served by `nginx-unprivileged`. |

Every generated Dockerfile:

- uses a multi-stage build where the stack allows it, so build tools are not in the final image;
- runs as a non-root user;
- listens on `$PORT`.

When the build context has no `.dockerignore` and the Dockerfile is generated, Koptan adds a default one. It keeps `.git`, `.env` files, dependency folders (`node_modules`, `.venv`, ...), build output (`target`, `bin`, `obj`) and editor folders out of the image.

To see the Dockerfile Koptan used:

```bash
kubectl get configmap $(kubectl get ksvc <name> -o jsonpath='{.status.dockerfileConfigMap}') \
  -o jsonpath='{.data.Dockerfile}'
```

!!! tip "Your application must listen on `$PORT`"
    Koptan sets `PORT` in the container, and the readiness probe checks that port over TCP. If your application listens on a fixed port, set `spec.port` to that port.

## Status

```bash
kubectl get ksvc
```

```
NAME              PHASE   LANGUAGE   REVISION                                   CI                   CD                   AGE
example-service   Ready   python     3f9c2a1b7d4e8a0c9b6e5f4d3c2b1a0987654321   example-service-ci   example-service-cd   5m
```

Add `-o wide` to also see `LASTPUSH`.

| Field | Description |
|---|---|
| `phase` | `Pending`, `Discovering`, `Building`, `Ready` or `Failed`. |
| `serviceType` | The detected language, or `dockerfile` when your own Dockerfile was used. |
| `latestRevision` | SHA of the last processed commit. |
| `lastPushDetected` | When Koptan last saw a new commit. |
| `ciRef` / `cdRef` | Names of the CI and CD. |
| `dockerfileConfigMap` | ConfigMap holding the Dockerfile the CI builds with. |
| `dockerfileSource` | `repo` or `template`. |
| `detected` | What discovery found: `language`, `version`, `packageManager`, `framework`, `entrypoint`. |
| `message` / `error` | Human-readable progress or the reason for a failure. |
| `conditions` | Standard conditions, including `Ready`. |

A `Failed` Service retries by itself after one minute. You do not need to recreate it. Fix the cause (for example, push a fix or create the missing Secret) and it recovers on the next attempt, or right away if you add the [refresh annotation](#trigger-a-check-right-away).

## Common tasks

**Scale:**

```bash
kubectl patch ksvc hello --type merge -p '{"spec":{"replicas":3}}'
```

**Pin a version (roll back):** set `revision` to a tag or SHA.

```bash
kubectl patch ksvc hello --type merge -p '{"spec":{"source":{"revision":"3f9c2a1b7d4e8a0c9b6e5f4d3c2b1a0987654321"}}}'
```

**Change an environment variable:** edit `spec.env`. The Deployment rolls out with the new value without a rebuild.

**Build one service in a monorepo:**

```yaml
spec:
  source:
    repo: https://github.com/acme/platform.git
  build:
    contextDir: services/orders
```

**Delete:** `kubectl delete ksvc hello`. This deletes the CI, the CD, the build Pods, the Dockerfile ConfigMap, the Deployment and the Kubernetes Service. Secrets you created yourself are kept. Images stay in the registry.

## Creating a Service from the UI

On the **Services** page, choose **New Deployment**. The form creates the same Service. The access token you enter is stored in a Secret named `<name>-git`, and the registry username and password in a dockerconfigjson Secret named `<name>-registry`. The Service owns both, so they are deleted with it. See [Koptan UI](../ui/overview.md#services).
