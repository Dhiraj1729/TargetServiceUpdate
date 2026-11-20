#!/usr/bin/env bash
set -euo pipefail
# Backward-compatible entrypoint; application deployment is owned by this script.
exec bash "$(dirname "$0")/hsm-deploy.sh"
