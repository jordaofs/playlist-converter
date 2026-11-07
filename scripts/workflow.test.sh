#!/usr/bin/env bash
# Verifies the security rules of the GitHub workflows (plan T11c): the real
# workflows must pass and each deliberately broken fixture must be rejected.
set -euo pipefail
python3 scripts/check_workflows.py .github/workflows
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mutate() { # name, python expression applied to the parsed release workflow
  mkdir -p "$tmp/$1"
  python3 - "$tmp/$1/release.yml" "$2" <<'PY'
import sys, yaml
doc = yaml.safe_load(open('.github/workflows/release.yml'))
exec(sys.argv[2])
yaml.safe_dump(doc, open(sys.argv[1], 'w'))
PY
  if python3 scripts/check_workflows.py "$tmp/$1" >/dev/null 2>&1; then echo "Accepted unsafe workflow: $1"; exit 1; fi
}
mutate branch-trigger "doc[True] = {'push': {'branches': ['main']}}"
mutate pr-target "doc[True]['pull_request_target'] = {}"
mutate unpinned "doc['jobs']['image']['steps'][0]['uses'] = 'actions/checkout@v4'"
mutate deploy-action "doc['jobs']['deploy']['steps'].insert(0, {'uses': 'actions/checkout@' + '0'*40})"
mutate deploy-permissions "doc['jobs']['deploy']['permissions'] = {'contents': 'read'}"
mutate no-environment "doc['jobs']['deploy'].pop('environment')"
mutate secret-in-image 'doc["jobs"]["image"]["env"] = {"K": "${{ secrets.DEPLOY_SSH_KEY }}"}'
mutate tracing "doc['jobs']['deploy']['steps'][-1]['run'] = 'set -x\\n' + doc['jobs']['deploy']['steps'][-1]['run']"
mutate no-concurrency "doc.pop('concurrency')"
printf 'Workflow checks passed\n'
