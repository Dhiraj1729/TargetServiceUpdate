#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
mkdir -p "$ROOT/work/smoke"
evidence="$(mktemp -d "$ROOT/work/smoke/run.XXXXXX")"
service="hsm-smoke-$(date +%s)-$$"
pf_pid=''
cleanup() { if [[ -n "$pf_pid" ]]; then kill "$pf_pid" 2>/dev/null || true; wait "$pf_pid" 2>/dev/null || true; fi; }
trap cleanup EXIT
# Let kubectl choose a free loopback port; do not attach to an unrelated listener.
k -n "$NAMESPACE" port-forward --address=127.0.0.1 service/service-target-manager :8080 > "$evidence/port-forward.log" 2>&1 &
pf_pid=$!
for ((i=0;i<60;i++)); do
  kill -0 "$pf_pid" 2>/dev/null || fail "Port-forward failed: $evidence/port-forward.log"
  port="$(sed -n 's/^Forwarding from 127.0.0.1:\([0-9]*\) -> 8080/\1/p' "$evidence/port-forward.log" | head -1)"
  [[ -n "$port" ]] && break
  sleep 1
done
[[ -n "$port" ]] || fail 'Port-forward timed out.'
base="http://127.0.0.1:$port/api/v1/services"
request() {
  local method="$1" url="$2" expected="$3" body="${4:-}" status
  status="$(curl --silent --show-error --max-time 20 -X "$method" -H 'Content-Type: application/json' --data "$body" -o "$evidence/response.json" -w '%{http_code}' "$url")"
  [[ "$status" == "$expected" ]] || fail "$method returned $status instead of $expected; see $evidence/response.json (Service $service retained for debugging)"
}
wait_target() {
  local target="$1" body=''
  for ((i=0;i<60;i++)); do
    if k -n "$NAMESPACE" get endpointslices -l "kubernetes.io/service-name=$service" -o json | jq -e --arg pod "$target" '[.items[].endpoints[]? | select(.conditions.ready == true) | .targetRef.name] == [$pod]' >/dev/null; then
      body="$(k -n "$NAMESPACE" exec hsm-demo-0 -- wget -T 3 -qO- "http://$service:8080" 2>/dev/null || true)"
      if [[ "$body" == "$target" ]]; then printf 'Traffic through %s reached %s\n' "$service" "$target" | tee -a "$evidence/result.txt"; return; fi
    fi
    sleep 1
  done
  fail "Traffic did not converge to $target (last body: $body)"
}
request POST "$base" 201 "{\"serviceName\":\"$service\",\"podName\":\"hsm-demo-0\",\"ports\":[{\"name\":\"http\",\"port\":8080,\"targetPort\":8080}]}"
cluster_ip="$(jq -er '.clusterIP' "$evidence/response.json")"
wait_target hsm-demo-0
request PUT "$base/$service/target" 204 '{"podName":"hsm-demo-1"}'
wait_target hsm-demo-1
[[ "$(k -n "$NAMESPACE" get service "$service" -o jsonpath='{.spec.clusterIP}')" == "$cluster_ip" ]] || fail 'ClusterIP changed.'
request DELETE "$base/$service" 204
[[ -z "$(k -n "$NAMESPACE" get service "$service" --ignore-not-found -o name)" ]] || fail 'Service still exists.'
request DELETE "$base/$service" 204
request DELETE "$base/service-target-manager" 403
printf 'PASS create -> target update (same ClusterIP) -> delete; unrelated Service protected.\n' | tee -a "$evidence/result.txt"
printf 'Evidence: %s\n' "$evidence"
