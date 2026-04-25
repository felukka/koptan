# App Example

An App resource defines the source code repository for an application.

Koptan currently supports multiple app kinds, including:

- `GoApp`
- `JavaApp`
- `DotnetApp`

The only required field is the source repository. The examples below show minimal definitions for each app type.

=== "GoApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: GoApp
    metadata:
      name: feedme
    spec:
      source:
        repo: https://github.com/example/feedme
    ```

=== "JavaApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: JavaApp
    metadata:
      name: orders-service
    spec:
      source:
        repo: https://github.com/example/orders-service
    ```

=== "DotnetApp"

    ```yaml
    apiVersion: koptan.felukka.org/v1alpha
    kind: DotnetApp
    metadata:
      name: payments-service
    spec:
      source:
        repo: https://github.com/example/payments-service
    ```