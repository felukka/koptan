
---

# 📄 `docs/examples/voyage.md`

```md
# Voyage Example

A Voyage deploys an application using a Slipway.

It defines how the application runs inside the cluster.

## Examples

=== "Minimal"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Voyage
    metadata:
      name: feedme
    spec:
      slipwayRef:
        name: feedme
      port: 8080
    ```

=== "With Replicas & Env"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Voyage
    metadata:
      name: feedme
    spec:
      slipwayRef:
        name: feedme
      port: 8080
      replicas: 2
      env:
        - name: ENV
          value: production
        - name: LOG_LEVEL
          value: info
    ```

=== "With Health Check"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Voyage
    metadata:
      name: feedme
    spec:
      slipwayRef:
        name: feedme
      port: 8080
      replicas: 2
      healthCheck:
        path: /health
        port: 8080
        initialDelaySeconds: 10
        periodSeconds: 5
    ```

## Apply a Voyage

```bash
kubectl apply -f voyage.yaml