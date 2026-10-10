# Quickstart

This guide deploys an application from a git repository. It assumes the [operator is installed](installation.md).

You need:

- A git repository with a web application in one of the [supported stacks](../resources/service.md#supported-stacks), or with its own Dockerfile. The application must listen on the port in the `PORT` environment variable (Koptan sets it), or on the port you give in `spec.port`.
- A registry you can push to.

## 1. Create a namespace and the registry Secret

```bash
kubectl create namespace demo

kubectl -n demo create secret docker-registry demo-registry \
  --docker-server=ghcr.io \
  --docker-username=<github-user> \
  --docker-password=<token-with-write:packages>
```

## 2. Create the Service

Save this as `service.yaml`, and change the repository and image:

```yaml
apiVersion: koptan.felukka.org/v1
kind: Service
metadata:
  name: hello
  namespace: demo
spec:
  source:
    repo: https://github.com/<you>/<repo>.git
    revision: main          # branch, tag or commit SHA; omit for the default branch
  image:
    registry: ghcr.io
    repo: <you>/hello
    credentialsSecret: demo-registry
  port: 8080
  replicas: 1
```

```bash
kubectl apply -f service.yaml
```

!!! tip "Private repository?"
    Create a token Secret and add it under `source`:
    ```bash
    kubectl -n demo create secret generic hello-git --from-literal=token=<token>
    ```
    ```yaml
      source:
        repo: https://github.com/<you>/<repo>.git
        secretRef: {name: hello-git, key: token}
    ```

## 3. Watch it build and deploy

```bash
kubectl -n demo get ksvc,cis,cds -w
```

The Service moves through these phases:

| Phase | Meaning |
|---|---|
| `Pending` | Accepted, not processed yet. |
| `Discovering` | Cloning the repository and finding or generating the Dockerfile. |
| `Building` | The CI `hello-ci` is building the image. |
| `Ready` | The build succeeded. The CD `hello-cd` deploys it. |
| `Failed` | See `kubectl -n demo describe ksvc hello` and [Troubleshooting](../reference/troubleshooting.md). |

To follow the build itself:

```bash
POD=$(kubectl -n demo get ci hello-ci -o jsonpath='{.status.buildPod}')
kubectl -n demo logs -f "$POD" --all-containers
```

When the CD reports `Running`, the application is live.

## 4. Open the application

Koptan creates a Deployment and a Kubernetes Service, both named after the CD: `hello-cd`. The Kubernetes Service listens on port 80 and forwards to your application's port.

```bash
kubectl -n demo port-forward svc/hello-cd 8080:80
curl http://localhost:8080
```

To expose it outside the cluster, point an Ingress or Gateway route at the Kubernetes Service `hello-cd`, port 80.

## 5. Ship a change

Push a commit to the branch. Within a minute Koptan notices it, builds a new image tagged with the first 12 characters of the commit SHA and rolls it out.

To check right away instead of waiting:

```bash
kubectl -n demo annotate ksvc hello koptan.felukka.org/refresh="$(date +%s)" --overwrite
```

## 6. Clean up

```bash
kubectl -n demo delete ksvc hello
```

Deleting the Service deletes its CI, CD, build Pods, Deployment and Kubernetes Service. The image stays in your registry.

## What next

- See [what Koptan detected](../resources/service.md#status) and tune the build with [`spec.build`](../resources/service.md#build).
- Add a security scan before every build with a [CIPlugin](../resources/ciplugin.md).
- Get a Slack message on every deployment with an [Alert](../resources/alert.md).
- Let an AI agent write the code with a [SelfService](../resources/selfservice.md).
