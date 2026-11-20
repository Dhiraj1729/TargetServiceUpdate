#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
k get namespace "$NAMESPACE"
account="system:serviceaccount:$NAMESPACE:service-target-manager"
for verb in get list create patch delete; do
  [[ "$(k auth can-i "$verb" services -n "$NAMESPACE" --as="$account")" == yes ]] || fail "Missing services/$verb permission."
done
[[ "$(k auth can-i get pods -n "$NAMESPACE" --as="$account")" == yes ]] || fail 'Missing pod read permission.'
# can-i returns 1 for a denial; distinguish that from a CLI/API failure.
denied() {
  local status=0 answer
  answer="$(k auth can-i "$@" --as="$account")" || status=$?
  [[ "$status" == 1 && "$answer" == no ]] || fail "Expected denied permission: $* (status $status, answer $answer)"
}
denied delete pods -n "$NAMESPACE"
denied patch services -n default
k -n "$NAMESPACE" rollout status deployment/service-target-manager --timeout=180s
k -n "$NAMESPACE" get service service-target-manager
printf 'HSM deployment and namespace permissions verified.\n'
