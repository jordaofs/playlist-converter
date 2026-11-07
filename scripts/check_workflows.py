"""Security rules for the public repository's GitHub workflows.

Usage: python3 scripts/check_workflows.py <workflow-directory>
Exits non-zero and lists every violation. See .github/workflows/release.yml.
"""
import pathlib
import re
import sys

import yaml

PINNED = re.compile(r'^[\w.-]+/[\w./-]+@[0-9a-f]{40}$')
DEPLOY_SECRET = re.compile(r'secrets\.DEPLOY_')
problems = []


def fail(message):
    problems.append(message)


def triggers(doc):
    # PyYAML (YAML 1.1) reads the bare key `on` as boolean True.
    value = doc.get('on', doc.get(True, {}))
    return value if isinstance(value, dict) else {name: {} for name in ([value] if isinstance(value, str) else value)}


def check_common(name, doc):
    if 'pull_request_target' in triggers(doc):
        fail(f'{name}: pull_request_target exposes secrets to fork code')
    for job_name, job in doc.get('jobs', {}).items():
        for step in job.get('steps', []):
            uses = step.get('uses')
            if uses and not PINNED.match(uses):
                fail(f'{name}: {job_name} uses an action not pinned to a commit SHA: {uses}')


def check_release(doc):
    on = triggers(doc)
    if set(on) != {'push'} or set(on['push'] or {}) != {'tags'} or on['push']['tags'] != ['v*']:
        fail('release.yml: must run only on pushed v* tags')
    if doc.get('permissions') != {}:
        fail('release.yml: top-level permissions must be empty')
    if not doc.get('concurrency'):
        fail('release.yml: concurrency is required to serialize deploys')
    jobs = doc.get('jobs', {})
    image, deploy = jobs.get('image', {}), jobs.get('deploy', {})
    if image.get('permissions') != {'contents': 'read', 'packages': 'write'}:
        fail('release.yml: image job permissions must be contents read, packages write')
    if DEPLOY_SECRET.search(yaml.safe_dump(image)):
        fail('release.yml: deploy secrets must only appear in the deploy job')
    environment = deploy.get('environment')
    if (environment.get('name') if isinstance(environment, dict) else environment) != 'production':
        fail('release.yml: deploy job must use the production environment')
    if deploy.get('needs') not in ('image', ['image']):
        fail('release.yml: deploy job must need the image job')
    if deploy.get('permissions') != {}:
        fail('release.yml: deploy job permissions must be empty')
    for step in deploy.get('steps', []):
        if 'uses' in step:
            fail('release.yml: deploy job must not run actions')
        if re.search(r'set\s+-[a-z]*x', step.get('run', '')):
            fail('release.yml: deploy job must not trace commands (set -x)')
    for job_name in set(jobs) - {'image', 'deploy'}:
        if DEPLOY_SECRET.search(yaml.safe_dump(jobs[job_name])):
            fail(f'release.yml: {job_name} must not use deploy secrets')


def main(directory):
    files = sorted(pathlib.Path(directory).glob('*.y*ml'))
    names = {f.name for f in files}
    if 'release.yml' not in names:
        fail('release.yml is missing')
    for path in files:
        doc = yaml.safe_load(path.read_text()) or {}
        check_common(path.name, doc)
        if path.name == 'release.yml':
            check_release(doc)
        elif DEPLOY_SECRET.search(path.read_text()):
            fail(f'{path.name}: deploy secrets are only allowed in release.yml')
    for problem in problems:
        print(f'REJECT {problem}')
    return 1 if problems else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else '.github/workflows'))
