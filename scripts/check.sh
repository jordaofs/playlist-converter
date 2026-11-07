#!/usr/bin/env bash
# Runs every local gate in the same order as CI. Run from the repository root.
set -euo pipefail
go vet ./...
go test -race -coverprofile=coverage.out ./...
bash scripts/check-coverage.sh coverage.out 80
test -z "$(gofmt -l cmd internal)"
(cd web && pnpm typecheck && pnpm lint && pnpm format:check && pnpm exec vitest run --coverage && pnpm build && pnpm exec playwright test)
go build ./cmd/server
bash scripts/workflow.test.sh
bash scripts/release-check.test.sh
# Use the maintainer's untracked denylist when present; forks run without one.
# An explicitly set RELEASE_DENYLIST_FILE wins and fails if unreadable.
denylist=${RELEASE_DENYLIST_FILE:-}
if [ -z "$denylist" ] && [ -f .release-denylist ]; then denylist=.release-denylist; fi
RELEASE_DENYLIST_FILE="$denylist" bash scripts/release-check.sh
