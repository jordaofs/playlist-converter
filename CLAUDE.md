# Playlist converter

## Flow
### Scope
- Read `.claude/flow/architecture.md` for the proposed product, structure, and decisions.
- Owned Spotify ↔ YouTube playlists; React/TypeScript/Tailwind UI, Go backend, SQLite.
- Implementation is available; setup and operating instructions are in `docs/development.md` and `docs/deployment.md`.
### Commands
Commands below are implemented. See reports for verification results.
- install: `cd web && pnpm install --frozen-lockfile`; `go mod download`
- typecheck: `cd web && pnpm typecheck`
- lint: `cd web && pnpm lint`; `go vet ./...`
- format check: `cd web && pnpm format:check`; `test -z "$(gofmt -l cmd internal)"`
- test (all): `go test -race -coverprofile=coverage.out ./...`; `cd web && pnpm exec vitest run --coverage`
- test (single frontend file): `cd web && pnpm exec vitest run src/path/example.test.tsx`
- test (single Go test): `go test ./internal/matching -run '^TestName$' -count=1`
- build: `cd web && pnpm build`; `go build ./cmd/server`
- e2e: `cd web && pnpm exec playwright test`
- release checks: `bash scripts/workflow.test.sh`; `bash scripts/release-check.test.sh`; `RELEASE_DENYLIST_FILE=.release-denylist bash scripts/release-check.sh`
- toolchain (local): Go in `~/.local/go/bin`, Node 24.22.2 in `~/.nvm/versions/node/v24.22.2/bin`; neither is on the default PATH.
- matching evaluation: `go run ./cmd/match-eval -dataset internal/testfixtures/matching/holdout.json -min-precision 0.99 -min-coverage 0.50 -output .claude/flow/reports/matching.json` (synthetic data deliberately fails release gate).
### Conventions
- Provider-specific API behavior stays in adapters; matching remains independently testable.
- Persist source occurrences, decisions, and publication progress; preserve order and duplicates.
- Never silently select uncertain recordings or retry unknown remote mutations.
- Keep OAuth tokens encrypted on the server; validate inputs and enforce session ownership.
- Use TDD for implementation and at least 80% application test coverage.
- Keep helpers small and feature-focused; avoid in-place domain-state mutation.
- This repository is public. Never add hostnames, IPs, SSH details, server paths, production compose files or secrets; operations live in a separate private repository. Run `RELEASE_DENYLIST_FILE=.release-denylist bash scripts/release-check.sh` before publishing.
- Logs must not contain playlist names, songs, links, account identities or tokens; conversion events use the closed field set in `internal/conversions/events.go`; failed OAuth callbacks log only `auth.failed` with provider, reason and cause codes (`internal/httpapi/auth.go`).
- The maintainer handles all Git operations (init, commit, push, tags).
### Gates (all must pass before implementation is done)
- Typecheck, lint, format check, unit/integration tests, coverage, build, critical E2E tests.
- Matching evaluation reports precision and coverage separately in each direction.
- Live provider validation and independently labeled matching evaluation remain release gates.
### Memory
Read `.claude/flow/learnings.md` before planning.
