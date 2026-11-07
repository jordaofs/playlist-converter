"""Redacted release inventory check. Complements, not replaces, manual review."""
import os
import pathlib
import re
import subprocess
import sys

root = pathlib.Path(sys.argv[1]).resolve()
excluded = {'.release-denylist', '.git', '.claude', 'node_modules', 'dist', 'coverage', 'test-results', 'playwright-report', 'data', '__pycache__'}
patterns = [
    re.compile(r'(?:SPOTIFY_CLIENT_SECRET|YOUTUBE_CLIENT_SECRET|TOKEN_ENCRYPTION_KEY)[ \t]*[=:][ \t]*[\"\x27]?([^\s\"\x27]+)'),
    re.compile(r'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----)'),
]
files = []
failed = False
# Optional untracked list of private infrastructure strings (hosts, IPs, SSH
# aliases, server paths), one per line. Values are matched case-insensitively
# and never printed. A configured but unreadable list fails closed.
denylist = []
denylist_path = os.environ.get('RELEASE_DENYLIST_FILE')
if denylist_path:
    try:
        lines = pathlib.Path(denylist_path).read_text().splitlines()
    except OSError:
        print('REJECT denylist file configured but unreadable'); sys.exit(1)
    denylist = [line.strip().lower() for line in lines if line.strip() and not line.strip().startswith('#')]
for path in sorted(root.rglob('*')):
    relative = path.relative_to(root)
    if any(part in excluded for part in relative.parts):
        continue
    if path.is_symlink():
        print(f'REJECT symlink: {relative}'); failed = True; continue
    if not path.is_file() or path.name.startswith('.env') and path.name != '.env.example':
        continue
    if path.name in {'coverage.out', 'server'}:
        continue
    if path.suffix in {'.db', '.sqlite', '.sqlite3', '.pem', '.key'} or path.name.endswith(('-wal', '-shm')):
        print(f'REJECT private artifact: {relative}'); failed = True; continue
    files.append(str(relative))
    try:
        content = path.read_text()
    except UnicodeDecodeError:
        if path.suffix.lower() not in {'.png', '.jpg', '.webp', '.ico'}:
            print(f'REJECT unexpected binary: {relative}'); failed = True
        continue
    lowered = content.lower()
    if any(value in lowered for value in denylist):
        print(f'REJECT private infrastructure reference: {relative} (value redacted)'); failed = True
    for pattern in patterns:
        for match in pattern.finditer(content):
            value = match.group(1) if match.lastindex else match.group()
            if value in {'=', '"', "'"} or value.startswith(('${', '$', 'process.', 'get(')):
                continue
            # Literal synthetic assignments are only allowed inside checker tests.
            if relative.as_posix() == 'scripts/release-check.test.sh':
                continue
            print(f'REJECT potential secret: {relative} (value redacted)'); failed = True; break
if (root / '.git').exists():
    tracked = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).decode().split('\0')
    for name in filter(None, tracked):
        p = pathlib.Path(name)
        if any(part in excluded for part in p.parts) or p.name.startswith('.env') and p.name != '.env.example':
            print(f'REJECT tracked excluded file: {name}'); failed = True
print(f'Candidate inventory: {len(files)} files; private environment, local data and workflow memory excluded.')
if '--inventory' in sys.argv:
    print('\n'.join(files))
sys.exit(1 if failed else 0)
