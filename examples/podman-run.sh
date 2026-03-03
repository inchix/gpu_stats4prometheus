#!/bin/bash
# Quick-start: run gpu_stats4prometheus with Podman CDI GPU passthrough
set -euo pipefail

IMAGE="${IMAGE:-localhost/gpu_stats4prometheus:latest}"
PORT="${PORT:-9835}"

podman run --rm -it \
  --name gpu-exporter \
  --device nvidia.com/gpu=all \
  --read-only \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  -p "${PORT}:9835" \
  "${IMAGE}"
