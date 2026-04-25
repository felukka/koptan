# Slipway

`Slipway` is a Koptan resource associated with build-oriented workflow.

## Purpose

Slipway represents the part of the operator responsible for preparing or building application artifacts.

In practical terms, this may include operations such as:

- fetching source code
- preparing build context
- running language-specific build logic
- producing deployable output

## Implementation Clues in the Repository

Relevant files include:

- `api/v1alpha/slipway_types.go`
- `internal/controller/slipway_controller.go`
- `internal/controller/slipway_build.go`

## Relationship to Other Resources

Slipway works alongside:

- **App** resources, which describe the application
- **Voyage**, which represents the runtime or deployment stage
