#!/usr/bin/env bash
set -euo pipefail
export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:$PATH"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
PROFILE=hsm
NAMESPACE=hsm-system
KUBERNETES_VERSION=v1.37.0
EVIDENCE="$ROOT/work/setup"
MARKER="$EVIDENCE/managed-environment"
# Keep callers' daemon and kubeconfig overrides out of project operations.
unset DOCKER_HOST DOCKER_TLS_VERIFY DOCKER_CERT_PATH
export DOCKER_CONTEXT=colima-hsm
# Remember the terminal's kubeconfig before using our isolated operational copy.
export HSM_USER_KUBECONFIG="${HSM_USER_KUBECONFIG:-${KUBECONFIG:-$HOME/.kube/config}}"
export KUBECONFIG="$EVIDENCE/kubeconfig"
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
k() { kubectl --context="$PROFILE" --request-timeout=30s "$@"; }
owned() {
  [[ -f "$MARKER" && "$(< "$MARKER")" == "$ROOT" ]] || fail 'HSM profile ownership record is missing or belongs to another checkout.'
}
profile_identity() {
  local config="$HOME/.minikube/profiles/$PROFILE/config.json"
  [[ -f "$config" ]] || fail 'HSM Minikube profile is absent; run make cluster-up.'
  jq -e '.Name == "hsm" and .Driver == "docker" and .KubernetesConfig.ContainerRuntime == "containerd" and .KubernetesConfig.KubernetesVersion == "v1.37.0"' "$config" >/dev/null || fail 'Unexpected HSM Minikube configuration.'
}
docker_identity() {
  [[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/hsm/docker.sock" ]] || fail 'Unexpected HSM Docker socket.'
}
context_identity() {
  if [[ "${1:-required}" == allow-missing ]] && k config view -o json | jq -e '((.contexts // []) | length) == 0 and ((.clusters // []) | length) == 0 and ((.users // []) | length) == 0' >/dev/null; then
    return
  fi
  k config view -o json | jq -e '.contexts | length == 1 and .[0].name == "hsm" and .[0].context.cluster == "hsm" and .[0].context.user == "hsm"' >/dev/null || fail 'Unexpected project kubeconfig identity.'
}
cluster_identity() { owned; profile_identity; docker_identity; context_identity; }
