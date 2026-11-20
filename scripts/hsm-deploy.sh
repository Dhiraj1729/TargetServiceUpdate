#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
# Application-owned resources belong here, never in cluster-up.
k apply -f "$ROOT/deploy/local/bootstrap.yaml"
k apply -f "$ROOT/deploy/local/manager.yaml"
# The local tag is deliberately reused; restart to pick up the loaded image.
k -n "$NAMESPACE" rollout restart deployment/service-target-manager
bash "$ROOT/scripts/hsm-verify.sh"
bash "$ROOT/scripts/activate-context.sh" "$NAMESPACE"
