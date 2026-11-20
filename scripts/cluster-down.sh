#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
owned
state="$(colima list --json | jq -sr '[.[] | select(.name == "hsm")][0].status // "Absent"')"
if [[ "$state" == Stopped || "$state" == Absent ]]; then
  printf 'HSM VM is %s. No stop required.\n' "$state"
  exit 0
fi
[[ "$state" == Running ]] || fail "Unexpected Colima state: $state"
docker_identity
if [[ -f "$HOME/.minikube/profiles/hsm/config.json" ]]; then
  profile_identity
  minikube stop -p "$PROFILE"
fi
colima stop "$PROFILE"
if colima status "$PROFILE" >/dev/null 2>&1; then fail 'HSM VM is still running.'; fi
printf 'HSM stopped; VM disk, cluster data and kubeconfig preserved.\n'
