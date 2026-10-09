"""Read-only artifact checks, plus an isolated build of the saved Mermaid probe.

This checks file links/fence structure/parser input, not browser layout or judgments.
Run from any directory. It never invokes canopy or reads a user's wiki.
"""
import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile
from pathlib import Path
from urllib.parse import unquote

audit = Path(__file__).resolve().parent.parent
repo = audit.parents[2]
scratch = repo/'review_probes/root_qa'
if scratch.exists():
    raise SystemExit('Existing root_qa scratch; refusing to overwrite.')
fixture = Path(tempfile.mkdtemp(prefix='root-report-qa-', dir='/private/tmp/canopy-audit-20261009'))
created_scratch = False
env = dict(os.environ)
for key, leaf in [('HOME','home'), ('XDG_CONFIG_HOME','config'), ('XDG_CACHE_HOME','cache'),
                  ('XDG_DATA_HOME','data'), ('XDG_STATE_HOME','state'), ('TMPDIR','tmp'),
                  ('GOPATH','gopath')]:
    path = fixture/leaf
    path.mkdir()
    env[key] = str(path)
env.update(GOCACHE=str(fixture/'go-cache'), GOTELEMETRY='off',
           GOMODCACHE='/Users/nobocop/go/pkg/mod', GOPROXY='off', GOSUMDB='off',
           GOENV='off', GOTOOLCHAIN='local', CGO_ENABLED='0')

def run(argv):
    p = subprocess.run(argv, cwd=repo, env=env, capture_output=True, text=True)
    if p.returncode:
        raise RuntimeError(json.dumps(dict(argv=argv, exit=p.returncode,
                                           stdout=p.stdout, stderr=p.stderr)))
    return p

report = {'scope':'Audit Markdown links/fences and repository Mermaid parser only; no browser QA.',
          'fixture':str(fixture), 'files':[], 'errors':[]}
try:
    scratch.mkdir(parents=True)
    created_scratch = True
    shutil.copyfile(audit/'evidence/root-report-qa.go.txt', scratch/'main.go')
    build = run(['go','build','-mod=readonly','-o',str(fixture/'qa'),'./review_probes/root_qa'])
    report['build_stderr'] = build.stderr
    for path in sorted(audit.glob('*.md')):
        text = path.read_text()
        opened = None
        prose = []
        for number, line in enumerate(text.splitlines(), 1):
            m = re.match(r'^ {0,3}(`{3,}|~{3,})(.*)$', line)
            if m:
                marker, rest = m.groups()
                if opened is None:
                    opened = (marker[0],len(marker),number)
                elif marker[0] == opened[0] and len(marker) >= opened[1] and not rest.strip():
                    opened = None
            elif opened is None:
                prose.append(line)
        if opened:
            report['errors'].append(f'{path.name}: unclosed fence at {opened[2]}')
        missing = []
        for link in re.findall(r'\]\(([^)\s]+)\)', '\n'.join(prose)):
            target = unquote(link.strip('<>').split('#')[0])
            if not target or re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:',target) or target.startswith('/'):
                continue
            if not (path.parent/target).exists():
                missing.append(link)
        report['errors'] += [f'{path.name}: missing {x}' for x in missing]
        result = json.loads(run([str(fixture/'qa'),str(path)]).stdout)
        report['files'].append(dict(path=path.name, sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
                                    missing_links=missing, **result))
except Exception as error:
    report['errors'].append(str(error))
finally:
    for name, path, owned in [('scratch',scratch,created_scratch), ('fixture',fixture,True)]:
        if owned and path.exists():
            try:
                shutil.rmtree(path)
            except OSError as error:
                report['errors'].append(f'{name} cleanup: {error}')
    report['scratch_removed'] = not scratch.exists()
    report['fixture_removed'] = not fixture.exists()
report['ok'] = not report['errors'] and report['scratch_removed'] and report['fixture_removed']
(audit/'evidence/root-report-qa.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'ok':report['ok'], 'files':len(report['files']),
                  'mermaid_blocks':sum(x['report_mermaid_blocks'] for x in report['files']),
                  'errors':report['errors']}, ensure_ascii=False))
raise SystemExit(0 if report['ok'] else 1)
