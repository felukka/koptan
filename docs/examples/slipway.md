# Slipway Example

A Slipway connects an application to the build pipeline.

It references an existing App using `name` and `kind`.

## Examples

=== "GoApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Slipway
    metadata:
      name: feedme
    spec:
      appRef:
        name: feedme
        kind: GoApp
    ```

=== "JavaApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Slipway
    metadata:
      name: orders-service
    spec:
      appRef:
        name: orders-service
        kind: JavaApp
    ```

=== "DotnetApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: Slipway
    metadata:
      name: payments-service
    spec:
      appRef:
        name: payments-service
        kind: DotnetApp
    ```

## Apply a Slipway

```bash
kubectl apply -f slipway.yaml