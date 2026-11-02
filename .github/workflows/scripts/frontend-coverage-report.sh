#!/usr/bin/env bash
# Summarizes the Playwright coverage report into coverage.md for the PR comment and job summary.
# Usage: frontend-coverage-report.sh <coverage dir> <commit sha>
set -euo pipefail

dir=${1:-coverage}
sha=${2:-$GITHUB_SHA}

statements=$(node -p "require('./$dir/coverage-report.json').summary.statements.pct")
{
  echo '<!-- frontend-coverage-report -->'
  echo "### Frontend test coverage: ${statements}% of statements"
  echo
  echo "<details><summary>Per-file coverage for ${sha::7}</summary>"
  echo
  grep '^|' "$dir/coverage-details.md"
  echo
  echo '</details>'
} > "$dir/coverage.md"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat "$dir/coverage.md" >> "$GITHUB_STEP_SUMMARY"
fi
