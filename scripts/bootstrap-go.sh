#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 - <<'PY'
import hashlib, io, json, os, pathlib, platform, tarfile, urllib.request
root = pathlib.Path.cwd()
releases = json.load(urllib.request.urlopen('https://go.dev/dl/?mode=json'))
release = next(r for r in releases if r['stable'])
arch = {'x86_64':'amd64', 'aarch64':'arm64'}.get(platform.machine(), platform.machine())
system = platform.system().lower()
item = next(f for f in release['files'] if f['os'] == system and f['arch'] == arch and f['kind'] == 'archive')
dest = root / '.tools' / release['version']
if not (dest / 'bin/go').exists():
    print('Downloading', item['filename'], flush=True)
    data = urllib.request.urlopen('https://go.dev/dl/' + item['filename']).read()
    if hashlib.sha256(data).hexdigest() != item['sha256']: raise SystemExit('Go archive checksum mismatch')
    dest.mkdir(parents=True, exist_ok=True)
    with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
        for member in archive.getmembers():
            member.name = member.name.removeprefix('go/')
            if member.name: archive.extract(member, dest, filter='data')
link = root / '.tools/go'
if link.is_symlink(): link.unlink()
link.symlink_to(dest.name)
print(dest / 'bin/go')
PY
