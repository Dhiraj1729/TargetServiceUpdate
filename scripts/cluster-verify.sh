#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
k wait --for=condition=Ready nodes --all --timeout=180s
nodes="$(k get nodes -o json)"
jq -e '.items | length == 2 and all(.[]; .status.nodeInfo.kubeletVersion == "v1.37.0")' <<<"$nodes" >/dev/null || fail 'Expected two Kubernetes v1.37.0 nodes.'
k -n kube-system rollout status deployment/coredns --timeout=180s
k -n kube-system wait --for=condition=Ready pod/storage-provisioner --timeout=120s
{
  date -u
  go version
  colima version
  docker version
  minikube version
  kubectl version --client
  helm version --short
  k get nodes -o wide
  k -n kube-system get pods -o wide
} > "$EVIDENCE/environment.txt"
k get nodes -o wide
printf 'Cluster infrastructure verified. Evidence: %s/environment.txt\n' "$EVIDENCE"
