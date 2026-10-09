"""Verify this audit's fixed inputs and delivery artifacts, never a user's wiki.

Run --prepare once after editing coordinator documents, then root-verify.py, then
run without --prepare. Worker settlement is a recorded Orca observation, not a
live runtime query. ID checks establish presence, not substantive correctness.
"""
import argparse
import hashlib
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--prepare', action='store_true')
args = parser.parse_args()
audit = Path(__file__).resolve().parent.parent
repo = audit.parents[2]
evidence = audit/'evidence'
env = dict(os.environ, GIT_OPTIONAL_LOCKS='0')

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def git(*words):
    return subprocess.check_output(['git',*words], cwd=repo, env=env).decode()

baseline = json.loads((evidence/'root-pre-final-manifest.json').read_text())
tracked = [p for p in git('ls-files','-z').split('\0') if p]
now_hashes = {p:sha(repo/p) for p in tracked}
changed = sorted(p for p,h in baseline['tracked_sha256'].items() if now_hashes.get(p)!=h)
index = Path(git('rev-parse','--git-path','index').strip())
if not index.is_absolute():
    index = repo/index
coordinator = {'README.md','plan.md','decisions.md','adjudication.json','evidence/check-review.py'}
audit_changes = sorted(p for p,h in baseline['audit_sha256'].items() if sha(audit/p)!=h)
immutable_changes = sorted(set(audit_changes)-coordinator)
final_check = json.loads((evidence/'final-final-check.json').read_text())
workers = json.loads((evidence/'root-workers.json').read_text())
ledger = json.loads((audit/'adjudication.json').read_text())
coverage_inputs = ['plan.md','adjudication.json','13-final-challenge.md',
                   '14-final-response.md','evidence/check-review.py']

if args.prepare:
    command = [sys.executable,str(evidence/'check-review.py')]
    run = subprocess.run(command,cwd=repo,capture_output=True,text=True)
    coverage = json.loads(run.stdout)
    coverage.update(command=command, exit=run.returncode, stderr=run.stderr,
                    checked_sha256={p:sha(audit/p) for p in coverage_inputs})
    (evidence/'root-coverage.json').write_text(json.dumps(coverage,ensure_ascii=False,indent=2)+'\n')
else:
    coverage = json.loads((evidence/'root-coverage.json').read_text())

checks = {
    'head_unchanged':git('rev-parse','HEAD').strip()==baseline['head'],
    'index_unchanged':sha(index)==baseline['index_sha256'],
    'tracked_contents_unchanged':not changed and set(now_hashes)==set(baseline['tracked_sha256']),
    'tracked_status_clean':not git('status','--porcelain','--untracked-files=no').strip(),
    'immutable_audit_inputs_unchanged':not immutable_changes,
    'final_challenge_unchanged':sha(audit/'13-final-challenge.md')==final_check['report_sha256'],
    'all_13_tasks_settled':len(workers['workers'])==13 and all(w['outcome']=='succeeded' for w in workers['workers']),
    'no_reclaimable_owned_workers':all(w['ownership']=='released' or
        (w['ownership']=='user_owned' and w['retained_reason']=='user_takeover') for w in workers['workers']),
    'complete_ledger':ledger['stage']=='complete' and len(ledger['findings'])==95 and len(ledger['final_objections'])==12,
    'coverage_pass':coverage.get('exit')==0 and coverage.get('ok') is True,
    'coverage_inputs_current':all(coverage['checked_sha256'].get(p)==sha(audit/p) for p in coverage_inputs),
}
scratch = repo/'review_probes'
scratch_files = sorted(str(p.relative_to(repo)) for p in scratch.rglob('*') if p.is_file()) if scratch.exists() else []
checks['probe_files_absent'] = not scratch_files
qa_summary = None
if not args.prepare:
    qa = json.loads((evidence/'root-report-qa.json').read_text())
    checked = {item['path']:item['sha256'] for item in qa['files']}
    current = {p.name:sha(p) for p in audit.glob('*.md')}
    checks['document_qa_pass'] = qa['ok'] and qa.get('fixture_removed') and qa.get('scratch_removed')
    checks['document_qa_current'] = checked==current
    qa_summary = dict(files=len(checked), mermaid_blocks=sum(item['report_mermaid_blocks'] for item in qa['files']))

result = dict(stage='prepared' if args.prepare else 'complete',
              captured_utc=datetime.now(timezone.utc).isoformat(), product_changes=False,
              head=git('rev-parse','HEAD').strip(), tracked_files=len(tracked), checks=checks,
              changed_tracked_files=changed, changed_coordinator_inputs=audit_changes,
              changed_immutable_inputs=immutable_changes, probe_files=scratch_files,
              worker_counts=workers['counts'], document_qa=qa_summary,
              coverage_counts=coverage['counts'],
              scope='Source/index and fixed audit inputs, ID presence, artifact QA, recorded Task/resource settlement; no browser/retrieval/production guarantee.')
result['ok'] = not args.prepare and all(checks.values())
(evidence/'root-final-integrity.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({k:result[k] for k in ['stage','ok','changed_tracked_files','changed_immutable_inputs','document_qa']},ensure_ascii=False))
raise SystemExit(0 if all(checks.values()) else 1)
