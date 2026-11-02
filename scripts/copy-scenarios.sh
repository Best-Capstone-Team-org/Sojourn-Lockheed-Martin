#!/usr/bin/env bash
# Copy scenario packages for the player image, keeping only allowlisted files
# usage: copy-scenarios.sh <src-dir> <dest-dir>
set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "usage: $0 <src-dir> <dest-dir>" >&2
    exit 1
fi

src=$1
dest=$2

allowed=(
    manifest.json
    briefing.md
    objectives.json
    setup.json
    firmware/probe_rom.elf
    firmware/symbols.json
    firmware/memmap.json
)

mkdir -p "$dest"

for dir in "$src"/*/; do
    name=$(basename "$dir")
    for f in "${allowed[@]}"; do
        if [[ -f "$dir$f" ]]; then
            mkdir -p "$dest/$name/$(dirname "$f")"
            cp "$dir$f" "$dest/$name/$f"
        fi
    done
done
