#!/usr/bin/env bash
set -euo pipefail
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'safe\n' > "$root/README.md"
bash scripts/release-check.sh "$root" >/dev/null
printf 'SPOTIFY_CLIENT_SECRET=fixture-secret-value\n' > "$root/leak.txt"
if bash scripts/release-check.sh "$root" > "$root/output" 2>&1; then echo 'Secret accepted'; exit 1; fi
if grep -q 'fixture-secret-value' "$root/output"; then echo 'Secret leaked in output'; exit 1; fi
rm "$root/leak.txt" "$root/output"
printf 'TOKEN_ENCRYPTION_KEY=private-value\n' > "$root/.env"
bash scripts/release-check.sh "$root" >/dev/null
printf 'artifact' > "$root/unsafe.db"
if bash scripts/release-check.sh "$root" >/dev/null 2>&1; then echo 'Database accepted'; exit 1; fi
rm "$root/unsafe.db"
# Private infrastructure references come from an untracked denylist, so the
# public script never contains the values it protects.
printf 'host notes: vps-fixture-alias at 203.0.113.99\n' > "$root/notes.md"
bash scripts/release-check.sh "$root" >/dev/null
deny=$(mktemp)
printf '# comment\n\nvps-fixture-alias\n203.0.113.99\n' > "$deny"
if RELEASE_DENYLIST_FILE="$deny" bash scripts/release-check.sh "$root" > "$root/output" 2>&1; then echo 'Infrastructure reference accepted'; rm -f "$deny"; exit 1; fi
if grep -qE 'vps-fixture-alias|203\.0\.113\.99' "$root/output"; then echo 'Denylisted value leaked in output'; rm -f "$deny"; exit 1; fi
grep -q 'notes.md' "$root/output" || { echo 'Offending file not named'; rm -f "$deny"; exit 1; }
if RELEASE_DENYLIST_FILE="$root/missing-denylist" bash scripts/release-check.sh "$root" >/dev/null 2>&1; then echo 'Missing denylist accepted'; rm -f "$deny"; exit 1; fi
rm -f "$deny" "$root/notes.md" "$root/output"
printf 'Release checker tests passed\n'
