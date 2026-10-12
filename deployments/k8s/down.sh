#!/usr/bin/env bash
# Stops the cluster; data (Postgres, MinIO, ES, Keycloak PVCs) survives. ./up.sh brings it back with a new URL.
# Wipe everything instead:  kubectl delete namespace mytube
set -euo pipefail
minikube stop
