# CD

A **CD** runs the image that a [CI](ci.md) built.

- **API:** `koptan.felukka.org/v1`, kind `CD`
- **Plural:** `cds` (`kubectl get cds`)
- **Created by:** Koptan, after the first successful build. Its name is `<service>-cd`.
- **Creates:** a Deployment and a Kubernetes Service, both named `<service>-cd`

!!! info "You do not create CDs"
    The CI creates the CD, and the Service keeps `replicas`, `env` and `port` in sync. Change those on the [Service](service.md). Resource requests and limits are the one thing you set on the CD itself (see [Resources](#resources)).

## What it deploys

**Deployment `<service>-cd`**

- One container named `app`, running the CI's latest image.
- Environment: your `spec.env`, plus `PORT` (the Service's port, default `8080`) and `CONTAINER_IMAGE` (the full image reference).
- A container port named `http` on `PORT`.
- A **TCP readiness probe** on `PORT`. Pods only receive traffic once something listens there.
- `imagePullSecrets` set to the Service's `image.credentialsSecret`, so private images can be pulled.
- `replicas` from the Service (default `1`).

**Kubernetes Service `<service>-cd`**

- Type `ClusterIP`, port **80** → your application's port.
- In-cluster address: `http://<service>-cd.<namespace>.svc` (port 80).

To expose the application outside the cluster, create an Ingress, Gateway API route or LoadBalancer that points at `<service>-cd` port 80:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hello
spec:
  rules:
    - host: hello.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: hello-cd
                port: {number: 80}
```

## Rollouts

Every successful build updates the Deployment to the new image, and Kubernetes rolls it out. The CD reports:

| Phase | Meaning |
|---|---|
| `Waiting` | The CI has no image yet. |
| `Deploying` | The rollout is in progress (`1/3 replicas available, rolling out <image>`). |
| `Running` | Every replica is updated and available. |
| `Failed` | The Deployment is not progressing (for example, the image cannot be pulled, or the container crashes). |

```bash
kubectl get cds
```

```
NAME                 PHASE     IMAGE                               REVISION     CI                   AGE
example-service-cd   Running   ghcr.io/example/repo:3f9c2a1b7d4e   3f9c2a1b…    example-service-ci   2d
```

When a CD is `Failed`, look at the application Pods:

```bash
kubectl get pods -l koptan.felukka.org/cd=<service>-cd
kubectl describe deployment <service>-cd
kubectl logs deployment/<service>-cd
```

## Resources

When Koptan first creates a CD, it sets these container resources:

| | Request | Limit |
|---|---|---|
| CPU | `100m` | `500m` |
| Memory | `128Mi` | `256Mi` |

Koptan only sets them when the CD has none, so you can change them on the CD and your values are kept:

```bash
kubectl patch cd hello-cd --type merge -p \
  '{"spec":{"resources":{"cpuRequest":"250m","cpuLimit":"1","memoryRequest":"256Mi","memoryLimit":"1Gi"}}}'
```

## Spec reference

| Field | Type | Default | Set by | Description |
|---|---|---|---|---|
| `ci.name` | string | — | Koptan | The CI whose image is deployed. |
| `replicas` | integer | `1` | Service | Number of Pods. |
| `env` | list of EnvVar | — | Service | Application environment. |
| `port` | integer, 1–65535 | `8080` | Service | Container port and `PORT`. |
| `imagePullSecret` | string | — | CI | Pull Secret for the image. |
| `resources.cpuRequest` / `cpuLimit` / `memoryRequest` / `memoryLimit` | quantity | see above | You | Container resources. |

## Status reference

| Field | Description |
|---|---|
| `phase` | `Waiting`, `Deploying`, `Running` or `Failed`. |
| `latestRevision` | Commit SHA of the deployed image. |
| `latestImage` | Deployed image. |
| `availableReplicas` | Ready replicas. |
| `message` | Progress or error. |
| `conditions` | Standard conditions. |

## Deleting

The CD is owned by the CI, which is owned by the Service. Deleting the Service deletes the CD, the Deployment and the Kubernetes Service.
