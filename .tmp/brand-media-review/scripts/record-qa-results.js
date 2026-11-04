const fs = require('node:fs');
const path = require('node:path');

// Consume explicit execution evidence. A related automated test is supporting
// coverage only; it must never silently pass a complete manual procedure.
function parseCSV(source) {
  const records = [];
  let record = [], value = '', quoted = false;
  source = source.replace(/^\uFEFF/, '');
  for (let i = 0; i < source.length; i++) {
    const char = source[i];
    if (char === '"') {
      if (quoted && source[i + 1] === '"') { value += '"'; i++; }
      else quoted = !quoted;
    } else if (char === ',' && !quoted) { record.push(value); value = ''; }
    else if ((char === '\n' || char === '\r') && !quoted) {
      if (char === '\r' && source[i + 1] === '\n') i++;
      record.push(value); value = '';
      if (record.some(Boolean)) records.push(record);
      record = [];
    } else value += char;
  }
  if (quoted) throw new Error('Unterminated CSV field');
  if (value || record.length) { record.push(value); records.push(record); }
  const headers = records.shift();
  return { headers, rows: records.map(values => {
    if (values.length !== headers.length) throw new Error('CSV column count mismatch');
    return Object.fromEntries(headers.map((header, i) => [header, values[i]]));
  }) };
}
function specs(suite) {
  return [...(suite.specs || []), ...(suite.suites || []).flatMap(specs)];
}
function run(manifestPath) {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const { headers, rows } = parseCSV(fs.readFileSync(manifest.tracker, 'utf8'));
  const index = new Map(rows.map(row => [row['Execution ID'], row]));
  if (index.size !== rows.length) throw new Error('Duplicate execution IDs');
  const roleAccounts = { customer: 'customer01', 'seller-owner': 'owner01', 'seller-staff': 'staff01', 'marketplace-admin': 'marketplace01', 'super-admin': 'super01', 'support-agent': 'support01', 'finance-operator': 'finance01', 'returns-operations': 'operations01' };
  const supporting = [], executions = [], suiteSummaries = [];
  for (const suite of manifest.playwright || []) {
    const report = JSON.parse(fs.readFileSync(suite.file, 'utf8'));
    suiteSummaries.push({ file: suite.file, browser: suite.browser, ...report.stats });
    for (const spec of report.suites.flatMap(specs)) {
      for (const test of spec.tests || []) {
      const result = test?.results?.at(-1);
      if (!result) continue;
      const id = /^([A-Z][A-Z0-9]*-\d{3}--[a-z0-9-]+)$/.exec(spec.title)?.[1];
      const status = result.status === 'passed' ? 'Passed' : result.status === 'skipped' ? 'Blocked' : 'Failed';
      const browser = test.projectName || suite.browser;
      const entry = { title: spec.title, id, status, file: suite.file, source: spec.file, browser, build: suite.build, errors: (result.errors || []).map(error => (error.message || '').replace(/\u001b\[[0-9;]*m/g, '')).join('\n') };
      if (!id || (manifest.partialCases || []).includes(id.split('--')[0])) {
        supporting.push(entry);
        continue;
      }
      const row = index.get(id);
      if (!row) throw new Error(`Test execution absent from tracker: ${id}`);
      if (row['Tested build']) {
        row['Retest status'] = status;
        row['Retest evidence'] = suite.file;
      }
      row.Status = status;
      row['Actual result'] = status === 'Passed' ? 'Executed the named procedure against the running application; all assertions passed.' : entry.errors.slice(0, 2400) || 'Explicit isolated-environment prerequisite was unavailable.';
      row.Environment = suite.environment;
      row['Tested build'] = suite.build;
      row['Browser or client'] = browser;
      row['Test date'] = manifest.date;
      row.Tester = 'Codex';
      row['Evidence location'] = suite.file;
      const roleAccount = roleAccounts[id.split('--')[1]];
      row['Account and record IDs'] = (test.annotations || []).filter(annotation => annotation.type === 'account').map(annotation => annotation.description).join('; ') || suite.accounts || (roleAccount ? `demo.${roleAccount}@weevcrafts.local; isolated QA fixture` : 'Dedicated QA fixtures; context identified by Execution ID.');
      if (id.startsWith('AUTH-003--')) row['Account and record IDs'] = `qa.unknown.${id.split('--')[1]}.<timestamp>@example.invalid; no persisted account; see unknown-account-evidence.txt`;
      row['Scope or blocker notes'] = suite.notes || '';
      if (status === 'Failed') row['Defect ID'] = id === 'GST-020--guest' ? 'QA-GST-001' : id === 'INT-027--qa-and-technical-operator' ? 'QA-PAY-001' : `QA-${id}`;
      if (status === 'Passed' && row['Defect ID'].startsWith('QA-NAV-002')) {
        row['Defect ID'] = '';
        row['Scope or blocker notes'] += ' Initial HTTP-403-only assertion was corrected to accept /account redirects solely to the assigned workspace.';
      }
      executions.push(entry);
      }
    }
  }
  const goSummaries = [];
  const goEvidence = new Map();
  for (const suite of manifest.go || []) {
    const events = fs.readFileSync(suite.file, 'utf8').split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
    const counts = { pass: 0, fail: 0, skip: 0, topLevelPass: 0 };
    for (const event of events.filter(event => event.Test && ['pass', 'fail', 'skip'].includes(event.Action))) {
      counts[event.Action]++;
      if (event.Action === 'pass' && !event.Test.includes('/')) counts.topLevelPass++;
      if (event.Action === 'pass') goEvidence.set(event.Test, suite);
    }
    goSummaries.push({ ...suite, ...counts });
  }
  for (const mapping of manifest.completeMappings || []) {
    const row = index.get(mapping.id);
    if (!row) throw new Error(`Unknown mapped execution ${mapping.id}`);
    let proof = mapping.goTest ? goEvidence.get(mapping.goTest) : supporting.find(entry => entry.title === mapping.title && entry.status === 'Passed');
    let status = 'Passed', actual = mapping.actual;
    if (mapping.k6File) {
      for (const name of mapping.goTests || []) if (!goEvidence.has(name)) throw new Error(`Missing concurrency evidence ${name}`);
      const load = JSON.parse(fs.readFileSync(mapping.k6File, 'utf8'));
      const failed = load.metrics.http_req_failed.values || load.metrics.http_req_failed;
      const duration = load.metrics.http_req_duration.values || load.metrics.http_req_duration;
      const checks = load.metrics.checks.values || load.metrics.checks;
      const requests = load.metrics.http_reqs.values || load.metrics.http_reqs;
      const thresholds = [...Object.values(load.metrics.http_req_failed.thresholds), ...Object.values(load.metrics.http_req_duration.thresholds)];
      if (thresholds.length !== 3) throw new Error('Expected all three documented load thresholds');
      // Legacy --summary-export booleans mean "threshold failed"; the modern
      // handleSummary representation exposes the opposite `ok` field.
      status = thresholds.every(threshold => typeof threshold === 'boolean' ? !threshold : threshold.ok) && checks.fails === 0 ? 'Passed' : 'Failed';
      actual = `${mapping.actual} Requests=${requests.count}; failures=${failed.rate ?? failed.value}; P95=${duration['p(95)']}ms; P99=${duration['p(99)']}ms; threshold verdict=${status}.`;
      proof = { file: mapping.k6File, build: manifest.finalBuild, browser: 'k6 / real PostgreSQL integration clients' };
    }
    if (!proof) throw new Error(`Missing passing evidence for ${mapping.id}`);
    Object.assign(row, { Status: status, 'Actual result': actual, 'Account and record IDs': mapping.accounts || 'Unique disposable fixtures created by the named test.', Environment: mapping.environment || manifest.environment, 'Tested build': manifest.finalBuild, 'Browser or client': proof.browser || 'Go integration client / real PostgreSQL', 'Test date': manifest.date, Tester: 'Codex', 'Evidence location': proof.file, 'Scope or blocker notes': mapping.notes || '' });
    if (status === 'Failed') row['Defect ID'] = `QA-${mapping.id}`;
  }
  for (const blocker of manifest.blockers || []) {
    for (const row of rows.filter(row => blocker.ids.includes(row['Case ID']))) {
      if (row.Status !== 'Not run') continue;
      row.Status = 'Blocked';
      row['Scope or blocker notes'] = blocker.reason;
      row.Environment = manifest.environment;
      row['Test date'] = manifest.date;
      row.Tester = 'Codex';
    }
  }
  for (const row of rows.filter(row => row.Status === 'Not run')) {
    row['Scope or blocker notes'] = row['Scope or blocker notes'] || 'Full manual procedure remains unexecuted. Related automation is supporting evidence only; see the run report and automated-check inventory.';
  }
  const counts = rows.reduce((counts, row) => (counts[row.Status] = (counts[row.Status] || 0) + 1, counts), {});
  const bySection = {};
  for (const row of rows) {
    bySection[row.Section] ||= {};
    bySection[row.Section][row.Status] = (bySection[row.Section][row.Status] || 0) + 1;
  }
  const quote = value => '"' + String(value ?? '').replaceAll('"', '""') + '"';
  fs.writeFileSync(manifest.tracker, [headers, ...rows.map(row => headers.map(header => row[header]))].map(record => record.map(quote).join(',')).join('\r\n') + '\r\n');
  const summary = { run: manifest.run, date: manifest.date, finalBuild: manifest.finalBuild, counts, bySection, go: goSummaries, playwright: suiteSummaries, executions, supporting };
  fs.writeFileSync(path.join(path.dirname(manifestPath), 'run-summary.json'), JSON.stringify(summary, null, 2) + '\n');
  console.log(JSON.stringify({ counts, go: goSummaries, playwright: suiteSummaries }, null, 2));
}
if (require.main === module) run(process.argv[2]);
module.exports = { parseCSV, run };
