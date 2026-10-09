"""Bounded CLI check of DEF-A-01, using a new disposable wiki only.

The one-day counterexample depends on UTC time and the selected TZ. A seven-day
positive control distinguishes it from the literal seven-day E4 check.
"""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True)
parser.add_argument('--output', required=True)
parser.add_argument('--tz', default='America/Los_Angeles')
args = parser.parse_args()
binary = str(Path(args.binary).resolve())
base = Path(tempfile.mkdtemp(prefix='root-snooze-', dir='/private/tmp/canopy-audit-20261009'))
wiki = base/'wiki'
wiki.mkdir()
env = dict(os.environ)
for key, leaf in [('HOME','home'), ('XDG_CONFIG_HOME','config'), ('XDG_CACHE_HOME','cache'),
                  ('XDG_DATA_HOME','data'), ('XDG_STATE_HOME','state'), ('TMPDIR','tmp')]:
    path = base/leaf
    path.mkdir()
    env[key] = str(path)
env.update(TZ=args.tz, GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1',
           GOTELEMETRY='off')
records = []

def cli(*words):
    argv = [binary, *words, '--wiki', str(wiki), '--json']
    p = subprocess.run(argv, cwd=base, env=env, capture_output=True, text=True, timeout=30)
    records.append(dict(argv=argv, exit=p.returncode, stdout=p.stdout, stderr=p.stderr))
    if p.returncode:
        raise RuntimeError('CLI failed; see records')
    return json.loads(p.stdout)

result = dict(fixture=str(base), utc_start=datetime.now(timezone.utc).isoformat(),
              tz=args.tz, binary_sha256=hashlib.sha256(Path(binary).read_bytes()).hexdigest(),
              limit='Wall-clock/TZ conditional CLI example, not a seven-day E4 failure.', records=records)
try:
    cli('init')
    (wiki/'concepts').mkdir(exist_ok=True)
    (wiki/'concepts/alpha.md').write_text('---\ntitle: Alpha\ncreated: 2026-01-01\nupdated: 2026-01-01\ntype: concept\ntags: []\n---\n\nOld isolated fixture page.\n')
    result['before'] = cli('resurface', '--strategy', 'random', '--peek')
    result['one_day_response'] = cli('resurface', 'feedback', 'alpha', '--snooze', '1')
    result['one_day_state'] = json.loads((wiki/'_meta/resurface/state.json').read_text())
    result['one_day_picks'] = cli('resurface', '--strategy', 'random', '--peek')
    result['seven_day_response'] = cli('resurface', 'feedback', 'alpha', '--snooze', '7')
    result['seven_day_state'] = json.loads((wiki/'_meta/resurface/state.json').read_text())
    result['seven_day_picks'] = cli('resurface', '--strategy', 'random', '--peek')
    result['one_day_counterexample'] = (result['one_day_response']['recorded'] and
                                       any(p['slug']=='alpha' for p in result['one_day_picks']['picks']))
    result['seven_day_suppressed'] = not result['seven_day_picks']['picks']
except Exception as error:
    result['error'] = str(error)
finally:
    shutil.rmtree(base)
    result['fixture_removed'] = not base.exists()
    Path(args.output).write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({k:result.get(k) for k in ['one_day_counterexample','seven_day_suppressed','fixture_removed','error']}))
raise SystemExit(1 if 'error' in result else 0)
