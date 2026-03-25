# Apps

Koptan provides application resources that represent application definitions at a higher level than native Kubernetes resources.

## Supported App Resources

Koptan currently defines:

- `GoApp`
- `DotnetApp`
- `JavaApp`

These types are defined in:

- `api/v1alpha/goapp_types.go`
- `api/v1alpha/dotnetapp_types.go`
- `api/v1alpha/javaapp_types.go`

## Purpose

An App resource is intended to describe an application in a language-aware way.

Instead of directly creating lower-level Kubernetes objects, users define an application using an App custom resource and let Koptan reconcile the desired state.

## Why Separate App Types?

Different languages often require different build or packaging behavior.

For that reason, Koptan models them as separate resources rather than a single generic application type.

## Related Resources

An App resource is related to:

- **Slipway**, which represents build-oriented workflow
- **Voyage**, which represents deployment-oriented workflow
