#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
k get namespace "$NAMESPACE" >/dev/null || fail 'Run make deploy before deploying the demo.'
k apply -f "$ROOT/deploy/local/demo.yaml"
k -n "$NAMESPACE" rollout status statefulset/hsm-demo --timeout=240s
k -n "$NAMESPACE" get pods -l app=hsm-demo -o wide
