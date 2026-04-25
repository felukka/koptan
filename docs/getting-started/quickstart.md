
# Quickstart

This page shows a minimal example of using Koptan.

## Step 1: Install the Operator

Make sure Koptan and its CRDs are installed.

See the Installation page for details.

## Step 2: Create an App Resource

Koptan supports multiple application resource types, including `GoApp`, `DotnetApp`, and `JavaApp`.

A minimal example is shown below:

```yaml
apiVersion: koptan.felukka.sh/v1alpha1
kind: GoApp
metadata:
  name: example-goapp
spec:
  {}
