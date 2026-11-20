#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
context_namespace="${1:-default}"
case "$context_namespace" in default|hsm-system) ;; *) fail 'Unsupported context namespace.';; esac
owned
context_identity
# Use kubectl's configuration writer so other entries (and KUBECONFIG lists)
# are preserved. Minikube keeps these certificates at stable paths on the host.
config="$(k config view -o json)"
server="$(jq -er '.clusters[] | select(.name == "hsm") | .cluster.server' <<<"$config")"
ca="$(jq -er '.clusters[] | select(.name == "hsm") | .cluster["certificate-authority"]' <<<"$config")"
cert="$(jq -er '.users[] | select(.name == "hsm") | .user["client-certificate"]' <<<"$config")"
key="$(jq -er '.users[] | select(.name == "hsm") | .user["client-key"]' <<<"$config")"
[[ -f "$ca" && -f "$cert" && -f "$key" ]] || fail 'Minikube certificate files are missing.'
export KUBECONFIG="$HSM_USER_KUBECONFIG"
# Refuse to repurpose an unrelated entry sharing the hsm name.
existing="$(kubectl config view -o json)"
jq -e --arg ca "$ca" --arg cert "$cert" --arg key "$key" '
  all(.clusters[]?; .name != "hsm" or .cluster["certificate-authority"] == $ca) and
  all(.users[]?; .name != "hsm" or (.user["client-certificate"] == $cert and .user["client-key"] == $key)) and
  all(.contexts[]?; .name != "hsm" or (.context.cluster == "hsm" and .context.user == "hsm"))
' <<<"$existing" >/dev/null || fail 'An unrelated hsm kubeconfig entry exists; refusing to overwrite it.'
kubectl config set-cluster hsm --server="$server" --certificate-authority="$ca"
kubectl config set-credentials hsm --client-certificate="$cert" --client-key="$key"
kubectl config set-context hsm --cluster=hsm --user=hsm --namespace="$context_namespace"
kubectl config use-context hsm
printf 'kubectl is ready: context=hsm, namespace=%s. Try kubectl get nodes or kubectl get pods -A.\n' "$context_namespace"
