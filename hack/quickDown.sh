#!/usr/bin/env bash

set -e

extract_name_from_yaml() {
  # This will match 'name:' that is indented under 'metadata'
  awk '/metadata:/,/- name:/ { if ($1 == "name:") print $2 }' "$1" | head -n 1 | tr -d '[:space:]'
}

YAML_FOLDER="examples"

GO_APP_NAME=$(extract_name_from_yaml "$YAML_FOLDER/goapp.yml")
SLIPWAY_NAME=$(extract_name_from_yaml "$YAML_FOLDER/goapp.yml")
VOYAGE_NAME=$(extract_name_from_yaml "$YAML_FOLDER/goapp.yml")

DOTNET_APP_NAME=$(extract_name_from_yaml "$YAML_FOLDER/dotnet.yml")
DOTNET_SLIPWAY_NAME=$(extract_name_from_yaml "$YAML_FOLDER/dotnet.yml")
DOTNET_VOYAGE_NAME=$(extract_name_from_yaml "$YAML_FOLDER/dotnet.yml")

JAVA_APP_NAME=$(extract_name_from_yaml "$YAML_FOLDER/java.yml")
JAVA_SLIPWAY_NAME=$(extract_name_from_yaml "$YAML_FOLDER/java.yml")
JAVA_VOYAGE_NAME=$(extract_name_from_yaml "$YAML_FOLDER/java.yml")

# Delete resources for GoApp
kubectl delete slipway "$SLIPWAY_NAME" || true
kubectl delete voyage "$VOYAGE_NAME" || true
kubectl delete goapp "$GO_APP_NAME" || true

# Delete resources for DotNetApp
kubectl delete slipway "$DOTNET_SLIPWAY_NAME" || true
kubectl delete voyage "$DOTNET_VOYAGE_NAME" || true
kubectl delete dotnetapp "$DOTNET_APP_NAME" || true

# Delete resources for JavaApp
kubectl delete slipway "$JAVA_SLIPWAY_NAME" || true
kubectl delete voyage "$JAVA_VOYAGE_NAME" || true
kubectl delete javaapp "$JAVA_APP_NAME" || true

make uninstall || true
sleep 2
check_minikube() {
  if minikube status >/dev/null 2>&1; then
     minikube stop
  fi
}

check_minikube
