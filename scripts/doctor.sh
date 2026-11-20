#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
missing=0
for tool in go colima docker minikube kubectl helm jq; do
  if command -v "$tool" >/dev/null; then
    printf 'OK %s: %s\n' "$tool" "$(command -v "$tool")"
  else
    printf 'MISSING %s\n' "$tool"; missing=1
  fi
done
[[ "$missing" == 0 ]] || fail 'Run make tools.'
go version
colima version
docker --version
DOCKER_CONTEXT=default minikube version
kubectl version --client
helm version --short
printf 'Host CPUs: %s; RAM bytes: %s\n' "$(sysctl -n hw.ncpu)" "$(sysctl -n hw.memsize)"
df -h "$ROOT"
printf 'Client checks complete. Cluster health is checked by make cluster-verify.\n'
