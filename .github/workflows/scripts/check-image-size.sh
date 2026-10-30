#!/usr/bin/env bash
# Fails if a docker image exceeds the size limit (4 GB).
# Usage: check-image-size.sh <image>
set -euo pipefail

image=$1
max=4000000000

size=$(docker image inspect "$image" --format '{{.Size}}')
echo "Image size: $size bytes (max $max)"
if [ "$size" -gt "$max" ]; then
  echo "::error::Image is $size bytes, over the 4 GB limit (R17.2)"
  exit 1
fi
