#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
[[ "$(uname -s)" == Darwin ]] || fail 'This setup requires macOS.'
[[ "$(sysctl -n hw.ncpu)" -ge 4 && "$(sysctl -n hw.memsize)" -ge 12884901888 ]] || fail 'At least 4 host CPUs and 12 GiB RAM are required.'
mkdir -p "$EVIDENCE"
chmod 700 "$EVIDENCE"
if [[ ! -f "$MARKER" ]]; then
  [[ ! -d "$HOME/.colima/hsm" && ! -d "$HOME/.minikube/profiles/hsm" && ! -e "$KUBECONFIG" ]] || fail 'Existing HSM environment has no ownership record; inspect it before reuse.'
  ! docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1 || fail 'Existing HSM Docker context has no ownership record.'
  printf '%s\n' "$ROOT" > "$MARKER"
fi
owned
if [[ -f "$HOME/.minikube/profiles/hsm/config.json" ]]; then profile_identity; fi
if [[ -s "$KUBECONFIG" ]]; then context_identity allow-missing; fi
if docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1; then docker_identity; fi
colima start "$PROFILE" --activate=false --runtime docker --vm-type vz --cpus 4 --memory 6 --disk 30
docker_identity
docker info >/dev/null
node_args=(--nodes=2)
if [[ -f "$HOME/.minikube/profiles/hsm/config.json" ]] && jq -e '.Nodes | length == 2' "$HOME/.minikube/profiles/hsm/config.json" >/dev/null; then
  # Existing node topology is retained by Minikube; avoid its resize warning.
  node_args=()
fi
minikube start -p "$PROFILE" --driver=docker --container-runtime=containerd ${node_args[@]+"${node_args[@]}"} --cpus=2 --memory=2048 --kubernetes-version="$KUBERNETES_VERSION" --keep-context --interactive=false
chmod 600 "$KUBECONFIG"
cluster_identity
k wait --for=condition=Ready nodes --all --timeout=240s
bash "$ROOT/scripts/cluster-verify.sh"
bash "$ROOT/scripts/activate-context.sh"
