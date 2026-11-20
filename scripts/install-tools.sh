#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
[[ "$(uname -s)" == Darwin ]] || fail 'This setup targets macOS with Colima.'
xcode-select -p >/dev/null 2>&1 || fail 'Install Apple Command Line Tools: xcode-select --install'
command -v brew >/dev/null || fail 'Install Homebrew from https://brew.sh first.'
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_UPGRADE=1 HOMEBREW_NO_INSTALL_CLEANUP=1
for pair in go:go colima:colima docker:docker minikube:minikube kubectl:kubernetes-cli helm:helm jq:jq; do
  tool="${pair%%:*}"; formula="${pair#*:}"
  if command -v "$tool" >/dev/null; then
    printf 'KEEP %s: %s\n' "$tool" "$(command -v "$tool")"
  else
    brew install "$formula"
  fi
done
bash "$ROOT/scripts/doctor.sh"
