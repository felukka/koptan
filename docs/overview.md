# Overview

Koptan is a Kubernetes operator for managing application delivery using custom resources.

It provides a higher-level workflow for teams that want to define applications declaratively and let the operator handle reconciliation, build orchestration, and deployment-related flows.

## Purpose

Koptan is designed to reduce the amount of manual Kubernetes configuration needed to build and run applications.

Instead of managing multiple low-level resources directly, users interact with a small set of Koptan resources:

- App resources for describing applications
- Slipway resources for build-related workflow
- Voyage resources for deployment-related workflow

## Problem Koptan Solves

Application teams often need to combine:

- source configuration
- build logic
- deployment configuration
- Kubernetes resource management

Koptan introduces operator-managed abstractions so this flow can be handled in a more consistent and reusable way.

## Main Resources

Koptan currently introduces the following main custom resources:

- **GoApp**
- **DotnetApp**
- **JavaApp**
- **Slipway**
- **Voyage**

See the Concepts section for more details.
