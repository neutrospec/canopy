#!/usr/bin/env python3
"""Check audit ID coverage, not the correctness of the review judgments.

Run from any directory. Use --partial while the debate is still in progress.
The final check also requires final challenge/response coverage and a complete ledger.
"""

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
GROUPS = [
    ([('DATA', '01-data.md'), ('STATE', '04-state.md')],
     '07-red-data.md', '10-defense-data.md', 'A'),
    ([('SEARCH', '02-search.md'), ('OPS', '06-ops.md')],
     '08-red-search.md', '11-defense-search.md', 'B'),
    ([('WEB', '03-web.md'), ('DOC', '05-docs.md')],
     '09-red-web-docs.md', '12-defense-web-docs.md', 'C'),
]


def ids(text, prefix):
    return set(re.findall(r'\b' + re.escape(prefix) + r'-\d{2}\b', text))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--partial', action='store_true')
    args = parser.parse_args()
    errors, pending, counts, all_ids = [], [], {}, set()

    def read(name):
        path = ROOT / name
        if not path.is_file():
            (pending if args.partial else errors).append('Missing: ' + name)
            return ''
        return path.read_text(encoding='utf-8')

    for originals, red_file, defense_file, group in GROUPS:
        red = read(red_file)
        defense = read(defense_file)
        original_ids = set()
        for prefix, name in originals:
            text = read(name)
            found = ids(text, prefix)
            counts[prefix] = len(found)
            if text and not found:
                errors.append('No stable IDs: ' + name)
            original_ids.update(found)
        new_red = ids(red, 'RED-' + group)
        new_defense = ids(defense, 'DEF-' + group)
        counts['RED-' + group] = len(new_red)
        counts['DEF-' + group] = len(new_defense)
        for finding in sorted(original_ids):
            for name, text in [(red_file, red), (defense_file, defense)]:
                if text and finding not in ids(text, finding.rsplit('-', 1)[0]):
                    errors.append(f'{finding} is not mentioned in {name}')
        for finding in sorted(new_red):
            if defense and finding not in ids(defense, 'RED-' + group):
                errors.append(f'{finding} has no defense reference')
        all_ids.update(original_ids | new_red | new_defense)

    ledger = {}
    ledger_path = ROOT / 'adjudication.json'
    if ledger_path.is_file():
        ledger = json.loads(ledger_path.read_text(encoding='utf-8'))
        rows = ledger.get('findings', [])
        row_ids = [row['id'] for row in rows]
        if len(row_ids) != len(set(row_ids)):
            errors.append('Duplicate adjudication IDs')
        for missing in sorted(all_ids - set(row_ids)):
            errors.append('Unadjudicated: ' + missing)
        for extra in sorted(set(row_ids) - all_ids):
            errors.append('Unknown adjudication ID: ' + extra)
        for row in rows:
            for key in ['claim_verdict', 'remedy_verdict', 'scope', 'decision']:
                if not row.get(key):
                    errors.append(f'{row["id"]}: missing {key}')
            for key in ['claim_verdict', 'remedy_verdict']:
                if row.get(key) not in ['accept', 'narrow', 'reject', 'defer']:
                    errors.append(f'{row["id"]}: invalid {key}')
        counts['adjudicated'] = len(rows)
        if not args.partial and ledger.get('stage') != 'complete':
            errors.append('Adjudication stage is not complete')
    else:
        (pending if args.partial else errors).append('Missing: adjudication.json')

    final = read('13-final-challenge.md')
    response = read('14-final-response.md')
    plan = read('plan.md')
    if final:
        for finding in sorted(all_ids):
            if finding not in ids(final, finding.rsplit('-', 1)[0]):
                errors.append(f'{finding}: no final-review reference')
        for item in sorted(ids(plan, 'PLAN')):
            if item not in ids(final, 'PLAN'):
                errors.append(f'{item}: no final-review reference')
        objections = ids(final, 'FINAL')
        counts['final_objections'] = len(objections)
        if response:
            for objection in sorted(objections):
                if objection not in ids(response, 'FINAL'):
                    errors.append(f'{objection}: no coordinator response reference')
            for extra in sorted(ids(response, 'FINAL') - objections):
                errors.append('Unknown response objection: ' + extra)
        if ledger.get('stage') == 'complete':
            final_rows = ledger.get('final_objections', [])
            final_row_ids = [row.get('id') for row in final_rows]
            if len(final_row_ids) != len(set(final_row_ids)):
                errors.append('Duplicate final-objection adjudication IDs')
            if set(final_row_ids) != objections:
                errors.append('Final-objection ledger does not match final report IDs')
            for row in final_rows:
                for key in ['targets', 'decision', 'challenge', 'response']:
                    if not row.get(key):
                        errors.append(f'{row.get("id")}: missing {key}')
                for key in ['claim_verdict', 'remedy_verdict']:
                    if row.get(key) not in ['accept', 'narrow', 'reject', 'defer']:
                        errors.append(f'{row.get("id")}: invalid {key}')
            counts['adjudicated_final_objections'] = len(final_rows)

    print(json.dumps({'ok': not errors, 'counts': counts, 'pending': pending,
                      'errors': errors, 'limit': 'ID presence only; read the debate matrices.'},
                     ensure_ascii=False, indent=2))
    return 1 if errors else 0


if __name__ == '__main__':
    raise SystemExit(main())
