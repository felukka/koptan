#!/usr/bin/env bash

set -e

check_minikube() {
  if minikube status >/dev/null 2>&1; then
    echo "Minikube is already running."
  else
    echo "Minikube is not running."
  fi
}

if ! command -v go >/dev/null 2>&1; then
  echo "Go is missing or not in PATH"
elif ! command -v make >/dev/null 2>&1; then
  echo "Make is missing or not in PATH"
elif ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is missing or not in PATH"
else
  #minikube start
  read -p "Do you want to start Minikube? (y/n): " start_minikube

  if [[ "$start_minikube" == "y" || "$start_minikube" == "Y" ]]; then
    check_minikube
    if minikube status >/dev/null 2>&1; then
      echo "Skipping minikube start as it's already running."
    else
      minikube start
    fi
  fi
  make generate
  make manifests
  make install
  make run
fi
