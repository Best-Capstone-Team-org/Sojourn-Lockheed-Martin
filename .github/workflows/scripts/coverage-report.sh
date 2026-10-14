#!/usr/bin/env bash
# Summarizes a Go coverage profile into coverage.md for the PR comment and job summary.
# Usage: coverage-report.sh <coverage.out> <commit sha>
set -euo pipefail

profile=${1:-coverage.out}
sha=${2:-$GITHUB_SHA}

go tool cover -func="$profile" | tee coverage.txt
{
  echo '<!-- go-coverage-report -->'
  echo "### Go test coverage: $(tail -n 1 coverage.txt | awk '{print $NF}')"
  echo
  echo "<details><summary>Per-function coverage for ${sha::7}</summary>"
  echo
  echo '```'
  cat coverage.txt
  echo '```'
  echo '</details>'
} > coverage.md

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat coverage.md >> "$GITHUB_STEP_SUMMARY"
fi
