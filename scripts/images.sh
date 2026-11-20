#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster_identity
arch="$(docker info --format '{{.Architecture}}')"
case "$arch" in aarch64|arm64) arch=arm64;; x86_64|amd64) arch=amd64;; *) fail "Unsupported architecture: $arch";; esac
cd "$ROOT"
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o bin/manager-linux ./cmd/manager
docker build --platform="linux/$arch" -t service-target-manager:local .
minikube -p "$PROFILE" image load service-target-manager:local --overwrite=true
