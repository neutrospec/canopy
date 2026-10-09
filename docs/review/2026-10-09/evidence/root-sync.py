"""Reproduce sync autostash conflicts using only a temporary local git remote."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True, help='Absolute path to a built canopy binary')
args = parser.parse_args()
binary = str(Path(args.binary).resolve())
base = Path(tempfile.mkdtemp(prefix='canopy-root-sync-', dir='/private/tmp'))
env = os.environ.copy()
env.update(HOME=str(base / 'home'), XDG_CONFIG_HOME=str(base / 'config'),
           XDG_DATA_HOME=str(base / 'data'), XDG_CACHE_HOME=str(base / 'cache'),
           XDG_STATE_HOME=str(base / 'state'), GIT_CONFIG_GLOBAL='/dev/null',
           GIT_CONFIG_NOSYSTEM='1')
records = []

def run(argv, cwd=base, check=True):
    r = subprocess.run(argv, cwd=cwd, env=env, text=True,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    records.append({'argv': argv, 'cwd': str(cwd), 'exit': r.returncode,
                    'stdout': r.stdout, 'stderr': r.stderr})
    if check and r.returncode:
        raise RuntimeError(json.dumps(records[-1]))
    return r

remote = base / 'origin.git'
a = base / 'a'
b = base / 'b'
(base / 'home').mkdir()
run(['git', 'init', '--bare', '--initial-branch=main', str(remote)])
run(['git', 'clone', str(remote), str(a)])
run(['git', '-C', str(a), 'config', 'user.name', 'Audit'])
run(['git', '-C', str(a), 'config', 'user.email', 'audit@example.invalid'])
run([binary, 'init', '--wiki', str(a), '--json'])
(a / 'concepts').mkdir(exist_ok=True)
page = a / 'concepts' / 'conflict.md'
content = '---\ntitle: Conflict\ncreated: 2026-01-01\nupdated: 2026-01-01\ntype: concept\ntags: []\n---\n\n# Conflict\n\nvalue = base\n'
page.write_text(content)
run(['git', '-C', str(a), 'add', '-A'])
run(['git', '-C', str(a), 'commit', '-m', 'base'])
run(['git', '-C', str(a), 'push', '-u', 'origin', 'main'])
run(['git', 'clone', str(remote), str(b)])
run(['git', '-C', str(b), 'config', 'user.name', 'Audit'])
run(['git', '-C', str(b), 'config', 'user.email', 'audit@example.invalid'])
(b / 'concepts' / 'conflict.md').write_text(content.replace('value = base', 'value = remote'))
run(['git', '-C', str(b), 'add', '-A'])
run(['git', '-C', str(b), 'commit', '-m', 'remote edit'])
run(['git', '-C', str(b), 'push'])
page.write_text(content.replace('value = base', 'value = local'))
sync = run([binary, 'sync', '--wiki', str(a), '--json', '-m', 'audit sync'], check=False)
remote_body = run(['git', '--git-dir', str(remote), 'show', 'main:concepts/conflict.md']).stdout
result = {'fixture': str(base), 'sync_exit': sync.returncode,
          'sync_stdout': sync.stdout, 'sync_stderr': sync.stderr,
          'working_body': page.read_text(), 'remote_body': remote_body,
          'remote_has_conflict_markers': '<<<<<<<' in remote_body,
          'records': records}
print(json.dumps(result, ensure_ascii=False, indent=2))
