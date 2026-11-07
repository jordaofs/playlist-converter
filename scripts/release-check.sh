#!/usr/bin/env bash
set -euo pipefail
python3 scripts/release_check.py "${1:-.}" "${@:2}"
