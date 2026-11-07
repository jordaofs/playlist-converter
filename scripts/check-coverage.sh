#!/usr/bin/env bash
set -euo pipefail
profile="${1:-coverage.out}"
minimum="${2:-80}"
filtered=$(mktemp)
trap 'rm -f "$filtered"' EXIT
awk 'NR==1 || $1 !~ /^playlistconverter\/internal\/testfixtures\// && $1 !~ /^playlistconverter\/cmd\/testserver\//' "$profile" > "$filtered"
percentage=$(go tool cover -func="$filtered" | awk '/^total:/ {gsub(/%/,"",$3); print $3}')
awk -v actual="$percentage" -v required="$minimum" 'BEGIN {printf "Backend coverage: %.1f%% (required %.1f%%)\n",actual,required; exit(actual < required)}'
