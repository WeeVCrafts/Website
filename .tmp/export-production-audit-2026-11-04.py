from pathlib import Path
import collections
import csv
import json
import re

root = Path(__file__).resolve().parents[1]
report = root / 'docs/production-readiness-audit-2026-11-04.md'
target = root / 'docs/production-readiness-backlog-2026-11-04.csv'
area = owner = ''
rows = []
types = {'G': 'Confirmed implementation/control gap', 'V': 'Verification required', 'D': 'Documentation or training', 'O': 'Measure and optimize', 'B': 'Business/legal/accounting decision'}
for line in report.read_text(encoding='utf-8').splitlines():
    if re.match(r'^## \d+\.', line):
        area = line.removeprefix('## ')
    if line.startswith('Owner: '):
        owner = line.removeprefix('Owner: ').rstrip('.')
    match = re.match(r'^\| ([A-Z0-9]+-\d+) \| (P[012]) \| ([GVDOB]) \| (.*) \|$', line)
    if match:
        ident, priority, kind, description = match.groups()
        rows.append({'ID': ident, 'Area': area, 'Priority': priority, 'Work type': types[kind], 'Proposed owner': owner, 'Task and completion criterion': description, 'Assignee': '', 'Status': 'Open assessment item', 'Target date': '', 'Evidence / tested SHA': '', 'Dependencies / notes': ''})
assert len({r['ID'] for r in rows}) == len(rows)
assert all(r['Area'] and r['Proposed owner'] for r in rows)
with target.open('w', encoding='utf-8-sig', newline='') as output:
    writer = csv.DictWriter(output, fieldnames=list(rows[0]))
    writer.writeheader()
    writer.writerows(rows)
with target.open(encoding='utf-8-sig', newline='') as saved:
    assert len(list(csv.DictReader(saved))) == len(rows)
print(json.dumps({'tasks': len(rows), 'priorities': dict(collections.Counter(r['Priority'] for r in rows)), 'types': dict(collections.Counter(r['Work type'] for r in rows)), 'areas': len({r['Area'] for r in rows}), 'report': str(report), 'csv': str(target)}))
