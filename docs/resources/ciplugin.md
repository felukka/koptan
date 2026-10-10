# CIPlugin

A **CIPlugin** adds a step to builds. It runs after the source is checked out and before the image is built and pushed. Use it for code scanning, linting, tests, license checks, or any other container you want to run on the source.

- **API:** `koptan.felukka.org/v1`, kind `CIPlugin`
- **Short name:** `cip` (`kubectl get cip`)
- **Types:** `sonarqube`, `codeql`, `custom`

## Attaching a plugin to Services

A plugin runs for a Service when **any** of these is true:

=== "The Service lists it"

    ```yaml
    kind: Service
    spec:
      plugins:
        - name: trivy-fs
    ```

=== "The plugin targets the Service by name"

    ```yaml
    kind: CIPlugin
    spec:
      targetRefs:
        - kind: Service
          name: example-service
    ```

=== "The plugin selects Services by label"

    ```yaml
    kind: CIPlugin
    spec:
      selector:
        matchLabels:
          koptan.felukka.org/quality: "sonar"
    ```

    Then label the Services:
    ```bash
    kubectl label ksvc example-service koptan.felukka.org/quality=sonar
    ```

Plugins and Services must be in the **same namespace**. Platform teams can use `selector` to apply a plugin to every matching Service without touching each Service.

!!! warning "A missing or invalid plugin stops the build"
    If a Service lists a plugin that does not exist, or a plugin is invalid, the Service goes to `Failed` instead of building without it. Koptan never skips a required check silently.

Changing a plugin rebuilds every Service it is attached to, because the CI pins each plugin's generation.

## Order and failures

- Steps run one after another, sorted by `order` (lowest first, default `100`). Equal orders are sorted by name.
- With `failurePolicy: Fail` (default), a failing step fails the build: nothing is pushed or deployed.
- With `failurePolicy: Ignore`, the step's failure is logged and the build goes on. Use this to report findings without blocking. `Ignore` needs an explicit `command` on custom steps, and the image must have `sh`.

## Type `custom`

Runs any container against the checked-out source.

```yaml
# Any container can be a step. It starts in the build context and gets
# KOPTAN_SERVICE, KOPTAN_REVISION, KOPTAN_SOURCE_DIR, KOPTAN_REPORTS_DIR and
# the other KOPTAN_* variables.
apiVersion: koptan.felukka.org/v1
kind: CIPlugin
metadata:
  name: trivy-fs
  namespace: default
spec:
  type: custom
  order: 10
  # Ignore: report findings without blocking the build.
  failurePolicy: Ignore
  custom:
    image: aquasec/trivy:0.56.2
    command: ["trivy"]
    args: ["fs", "--exit-code", "1", "--severity", "CRITICAL", "--no-progress", "."]
  resources:
    requests: {cpu: 100m, memory: 256Mi}
    limits: {cpu: "1", memory: 1Gi}
```

More ideas:

```yaml
# Run the test suite of a Node.js project before building.
spec:
  type: custom
  order: 5
  custom:
    image: node:22-bookworm-slim
    command: ["sh", "-c"]
    args: ["npm ci && npm test"]
```

```yaml
# Lint a Go project.
spec:
  type: custom
  custom:
    image: golangci/golangci-lint:v2.1.6
    command: ["golangci-lint", "run", "./..."]
```

| Field | Type | Required | Description |
|---|---|---|---|
| `custom.image` | string | **Yes** | Container image. |
| `custom.command` | list of string | No | Entrypoint override. Required with `failurePolicy: Ignore`. |
| `custom.args` | list of string | No | Arguments. |
| `custom.env` | list of EnvVar | No | Extra environment, including `valueFrom` Secrets. |
| `custom.envFrom` | list of EnvFromSource | No | Load a whole Secret or ConfigMap as environment. |

### Environment available to every step

The step's working directory is the build context (`KOPTAN_SOURCE_DIR`).

| Variable | Value |
|---|---|
| `KOPTAN_PLUGIN` | Plugin name. |
| `KOPTAN_SERVICE` | Service name. |
| `KOPTAN_NAMESPACE` | Namespace. |
| `KOPTAN_REPO` | Repository URL. |
| `KOPTAN_REVISION` | Full commit SHA being built. |
| `KOPTAN_LANGUAGE` | Language Koptan detected (empty when your own Dockerfile is used). |
| `WORKSPACE` | `/workspace`: the repository root. |
| `CONTEXT_DIR` | The Service's `build.contextDir`. |
| `KOPTAN_SOURCE_DIR` | `/workspace` or `/workspace/<contextDir>`. |
| `KOPTAN_REPORTS_DIR` | `/koptan/reports`: a shared folder for reports, kept for the steps that follow. |

Exit code `0` means success; anything else fails the step.

## Type `sonarqube`

Runs `sonar-scanner` against your SonarQube server and, by default, waits for the quality gate.

```yaml
apiVersion: koptan.felukka.org/v1
kind: CIPlugin
metadata:
  name: sonarqube
  namespace: default
spec:
  type: sonarqube
  order: 20
  failurePolicy: Fail
  sonarqube:
    hostURL: "https://sonarqube.example.com"
    # kubectl create secret generic sonarqube-token --from-literal=token=...
    tokenSecretRef:
      name: sonarqube-token
      key: token
    # Defaults to <namespace>_<service>.
    # projectKey: "team_api"
    waitForQualityGate: true
    extraArgs:
      - "-Dsonar.exclusions=**/*_test.go,**/vendor/**"
  selector:
    matchLabels:
      koptan.felukka.org/quality: "sonar"
```

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `sonarqube.hostURL` | string (`http(s)://…`) | **Yes** | — | SonarQube server URL. |
| `sonarqube.tokenSecretRef` | `{name, key}` | **Yes** | — | Secret key holding an analysis token. |
| `sonarqube.projectKey` | string | No | `<namespace>_<service>` | SonarQube project key. |
| `sonarqube.waitForQualityGate` | bool | No | `true` | Fail the step when the quality gate fails. |
| `sonarqube.extraArgs` | list of string | No | — | Extra `sonar-scanner` arguments, such as `-Dsonar.exclusions=...`. |
| `sonarqube.image` | string | No | `sonarsource/sonar-scanner-cli:11` | Scanner image. |

The scanner analyzes the build context and sends the commit SHA as the SCM revision.

## Type `codeql`

Analyzes the source with the [CodeQL](https://codeql.github.com/) CLI and fails when it finds results at or above a severity.

```yaml
# CodeQL code scanning. Without `languages` it analyzes the language Koptan
# detected (go, node, python, java, dotnet, ruby). The build fails when a
# result at or above failOnSeverity is found.
apiVersion: koptan.felukka.org/v1
kind: CIPlugin
metadata:
  name: codeql
  namespace: default
spec:
  type: codeql
  order: 30
  codeql:
    querySuite: security-extended
    failOnSeverity: error
    # languages: ["javascript-typescript"]
    # Upload the SARIF to GitHub code scanning:
    # github:
    #   repository: example/repo
    #   ref: refs/heads/main
    #   tokenSecretRef: {name: github-security-events, key: token}
  targetRefs:
    - kind: Service
      name: example-service
```

| Field | Type | Default | Description |
|---|---|---|---|
| `codeql.languages` | list of string | The detected language | CodeQL languages: `javascript-typescript` (or `javascript`, `typescript`), `python`, `java-kotlin` (or `java`, `kotlin`), `csharp`, `ruby`, `go`, `c-cpp` (or `cpp`), `swift`. |
| `codeql.querySuite` | enum | `code-scanning` | `code-scanning`, `security-extended` or `security-and-quality`. |
| `codeql.failOnSeverity` | enum | `error` | Lowest result level that fails the step: `error`, `warning`, `note`, or `none` (never fail). |
| `codeql.bundleURL` | `https://` URL | Latest GitHub CodeQL bundle | Use a pinned or mirrored bundle. |
| `codeql.image` | string | `debian:bookworm-slim` | Image to run in. Must be glibc-based. `go`, `cpp` and `swift` need their toolchain in the image. |
| `codeql.github.repository` | `owner/name` | — | Upload the SARIF results to this repository's code scanning. |
| `codeql.github.ref` | `refs/...` | — | Ref the results belong to, such as `refs/heads/main`. |
| `codeql.github.tokenSecretRef` | `{name, key}` | — | Token with `security_events` scope. |

Detected languages map to CodeQL like this: `go` → `go`, `node` → `javascript-typescript`, `python` → `python`, `java` → `java-kotlin`, `dotnet` → `csharp`, `ruby` → `ruby`. For other stacks, set `languages`.

## Common fields

| Field | Type | Default | Description |
|---|---|---|---|
| `type` | enum | — (**required**) | `sonarqube`, `codeql` or `custom`. The matching section (`sonarqube`, `codeql`, `custom`) is required. |
| `order` | integer 0–1000 | `100` | Sort order of the steps in a build. |
| `failurePolicy` | `Fail` / `Ignore` | `Fail` | What a failing step does to the build. |
| `targetRefs` | list of `{kind: Service, name}` | — | Services to attach to by name. Max 64. |
| `selector` | LabelSelector | — | Services to attach to by label. |
| `resources` | ResourceRequirements | — | CPU and memory for the step container. |

## Status

```bash
kubectl get cip
```

```
NAME       TYPE     ORDER   ACCEPTED   SERVICES              AGE
trivy-fs   custom   10      True       ["example-service"]   1h
```

| Field | Description |
|---|---|
| `attachedServices` | Services whose builds run this plugin. |
| `conditions[Accepted]` | `True` when the plugin is valid. When `False`, the message says why, for example `spec.sonarqube.hostURL is required`. |

Results of each run are on the CI, in `status.pluginResults`, and in the step's logs:

```bash
kubectl get ci example-service-ci -o jsonpath='{.status.pluginResults}'
kubectl logs <build-pod> -c plugin-trivy-fs
```

The **Security** page of the [Koptan UI](../ui/overview.md#security) shows every plugin and its latest results.
