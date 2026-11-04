const fs = require('node:fs/promises');
const path = require('node:path');
const root = path.resolve('tmp/authenticated-ui-audit');
const destination = path.resolve('docs/qa/UI-UX-20261102/comparison.html');
const escape = text => String(text ?? '').replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
const relative = file => encodeURI(path.relative(path.dirname(destination), file).replaceAll('\\', '/'));
const studies = [
  ['Customer catalogue', 'customer', '/products', 'catalogue'],
  ['Customer account', 'customer', '/account', 'customer'],
  ['Seller dashboard', 'seller', '/seller-admin', 'seller'],
  ['Admin dashboard', 'admin', '/admin', 'admin'],
  ['Super Admin dashboard', 'superadmin', '/super-admin', 'superadmin'],
  ['Category presentation', 'superadmin', '/super-admin/category-management', 'categories'],
];
async function main() {
  const before = JSON.parse(await fs.readFile(path.join(root, 'before/report.json')));
  const after = JSON.parse(await fs.readFile(path.join(root, 'after/report.json')));
  const extra = JSON.parse(await fs.readFile(path.join(root, 'editor/report.json')));
  after.excludedEndpoints = extra.screens.filter(s => s.route.endsWith('/storefront/library')).map(s => ({ route:s.route, reason:'HTML fragment fetched by the existing media picker, not a navigable application screen.' }));
  for (const item of extra.screens.filter(s => !s.route.endsWith('/storefront/library'))) {
    if (after.screens.some(s => s.role === item.role && s.route === item.route)) continue;
    for (const width of item.widths.filter(w => w.screenshot)) await fs.copyFile(path.join(root, 'editor', width.screenshot), path.join(root, 'after', width.screenshot));
    after.screens.push(item);
  }
  await fs.writeFile(path.join(root, 'after/report.json'), JSON.stringify(after, null, 2));
  const screen = (report, role, route) => report.screens.find(s => s.role === role && s.route === route);
  const image = (stage, file, label) => `<figure><figcaption>${escape(label)}</figcaption><a href="${relative(path.join(root, stage, file))}"><img loading="lazy" src="${relative(path.join(root, stage, file))}" alt="${escape(label)} — full screenshot"/></a></figure>`;
  let comparisons = '';
  for (const [label, role, route, study] of studies) {
    for (const width of [390, 1440]) {
      const b = screen(before, role, route)?.widths.find(w => w.width === width)?.screenshot;
      const a = screen(after, role, route)?.widths.find(w => w.width === width)?.screenshot;
      if (!b || !a) continue;
      comparisons += `<section><h2>${escape(label)} · ${width}px</h2><div class="comparison">${image('before', b, 'Before')}${image('designs', study + '-' + width + '.png', 'Design study')}${image('after', a, 'Implemented')}</div></section>`;
    }
  }
  const rows = after.screens.map(s => {
    const baseline = screen(before, s.role, s.route);
    const issues = [...(s.status !== 200 ? ['HTTP ' + s.status] : []), ...(s.error ? [s.error] : []), ...s.widths.filter(w => w.overflow).map(w => w.width + 'px overflow'), ...(s.accessibility || []).map(v => v.width + 'px ' + v.id), ...s.pageErrors];
    const shots = s.widths.filter(w => w.screenshot).map(w => `<a href="${relative(path.join(root, 'after', w.screenshot))}">${w.width}px</a>`).join(' · ');
    return `<tr data-role="${escape(s.role)}"><td>${escape(s.role)}</td><th scope="row">${escape(s.route)}</th><td>${escape(baseline ? baseline.accessibility?.map(v => v.id).join(', ') || 'No axe findings' : 'Additional final coverage')}</td><td>${escape(issues.join('; ') || 'Pass')}</td><td>${shots}</td></tr>`;
  }).join('');
  const failures = after.screens.filter(s => s.error || s.status !== 200 || s.widths.some(w => w.overflow) || s.accessibility?.length || s.pageErrors.length);
  const html = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Weevcrafts authenticated UI review</title><style>
  *{box-sizing:border-box}body{margin:0;padding:clamp(16px,4vw,48px);font:16px/1.6 Arial,sans-serif;color:#20362d;background:#fffdf8}main{max-width:1500px;margin:auto}h1{line-height:1.2}h2{margin:32px 0 12px}a{color:#123f32;text-underline-offset:3px}a:focus-visible,input:focus-visible{outline:3px solid #9d4031;outline-offset:3px}.comparison{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}figure{margin:0;border:1px solid #dfd8ca}figcaption{padding:10px 14px;font-weight:700}figure a{display:block;height:500px;overflow:hidden}img{display:block;width:100%;height:auto}section{border-top:1px solid #dfd8ca;padding-bottom:24px}.table-wrap{overflow:auto}table{width:100%;border-collapse:collapse;font-size:14px}th,td{padding:12px;border:1px solid #dfd8ca;text-align:left;vertical-align:top;overflow-wrap:anywhere}th{font-weight:600}label{display:block;margin:24px 0 12px}input{display:block;padding:12px;width:min(100%,500px);font:inherit;border:1px solid #56665d}small{color:#56665d}@media(max-width:800px){.comparison{grid-template-columns:1fr}figure a{height:540px}table{min-width:800px}}
  </style></head><body><main><h1>Weevcrafts: authenticated UI review</h1><p>2 November 2026 · Customer, Seller, Admin and Super Admin</p><p>${after.screens.length} real route screens checked at 320px, 390px and 1440px. ${failures.length} screens with remaining automated findings. Click any screenshot to inspect its full length.</p><p>Design studies use the running application's real components and fixture data. The seller implementation refines the initial study by putting stock guidance at the existing alert position instead of adding a generic instruction above the page title.</p><p><a href="responsive-designs.md">Design decisions</a> · <a href="README.md">Verification and limitations</a> · <a href="${relative(path.join(root,'before/report.json'))}">Baseline JSON</a> · <a href="${relative(path.join(root,'after/report.json'))}">Final JSON</a></p>${comparisons}<section><h2>Every audited screen</h2><label>Find a route or role<input type="search" id="filter" placeholder="e.g. customer, orders, seller-admin"></label><p id="count" role="status">${after.screens.length} screens</p><div class="table-wrap" tabindex="0" role="region" aria-label="Audited screens"><table><thead><tr><th scope="col">Role</th><th scope="col">Route</th><th scope="col">Baseline axe</th><th scope="col">Final automated checks</th><th scope="col">Screenshots</th></tr></thead><tbody>${rows}</tbody></table></div></section><small>Automated checks cover rendered pages, reflow and axe findings. They do not establish physical-device or screen-reader certification, live payment processing, or uploaded-media storage availability.</small></main><script>document.getElementById('filter').addEventListener('input',function(){let count=0;document.querySelectorAll('tbody tr').forEach(row=>{row.hidden=!row.textContent.toLowerCase().includes(this.value.toLowerCase());if(!row.hidden)count++;});document.getElementById('count').textContent=count+' screens';});</script></body></html>`;
  await fs.writeFile(destination, html); console.log(destination);
}
main().catch(error => { console.error(error); process.exitCode = 1; });
