# Voyage

`Voyage` is a Koptan resource associated with deployment-oriented workflow.

## Purpose

Voyage represents the stage where an application moves from build output into a running state on Kubernetes.

Depending on how the operator evolves, this may include concerns such as:

- rollout
- runtime configuration
- deployment reconciliation
- lifecycle management

## Implementation Clues in the Repository

Relevant files include:

- `api/v1alpha/voyage_types.go`
- `internal/controller/voyage_controller.go`

## Relationship to Other Resources

Voyage works with:

- **App** resources, which define the application
- **Slipway**, which handles build-oriented workflow
