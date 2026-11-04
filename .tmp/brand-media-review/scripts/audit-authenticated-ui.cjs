// Read-only browser audit of the isolated local role portals.
const { chromium } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

const baseURL = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18080';
if (baseURL !== 'http://127.0.0.1:18080') throw new Error('Use the isolated local UI audit instance on port 18080.');
const output = path.resolve(process.env.WEEVCRAFTS_UI_ARTIFACTS || 'tmp/authenticated-ui-audit');
const admin = ['', 'products', 'products/new', 'orders', 'inventory', 'support', 'sellers', 'seller-approvals', 'seller-suspensions', 'returns', 'finance', 'brands', 'customers', 'category-management', 'commission-rules', 'reviews', 'marketing', 'analytics', 'payments', 'failed-payments', 'refunds', 'settlements', 'finance-reconciliation', 'storefront'];
const roles = {
  customer: { account: 'customer01', routes: ['/products', '/products/blue-tote', '/cart', '/checkout', '/wishlist', '/account', '/account/profile', '/account/addresses', '/account/notifications', '/account/security', '/account/sessions', '/account/privacy', '/account/reviews', '/account/support', '/orders', '/tracking', '/returns', '/payments', '/notifications', '/privacy/center'] },
  seller: { account: 'owner01', routes: ['', 'products', 'products/new', 'inventory', 'orders', 'returns', 'earnings', 'analytics', 'settings', 'onboarding', 'support', 'terms'].map(p => '/seller-admin' + (p ? '/' + p : '')) },
  admin: { account: 'marketplace01', routes: admin.map(p => '/admin' + (p ? '/' + p : '')) },
  superadmin: { account: 'super01', routes: [...admin, 'users', 'roles', 'security', 'settings', 'audit-logs', 'security-events', 'system-health', 'search-indexing-health', 'worker-queue-health'].map(p => '/super-admin' + (p ? '/' + p : '')) },
};
if (process.env.WEEVCRAFTS_AUDIT_EXTRA === 'true') {
  const draft = process.env.WEEVCRAFTS_AUDIT_DRAFT;
  if (!/^[1-9][0-9]*$/.test(draft || '')) throw new Error('Choose an existing disposable QA draft for the editor audit.');
  roles.customer.routes = ['/contact'];
  roles.seller.routes = [];
  for (const role of ['admin', 'superadmin']) {
    const root = role === 'admin' ? '/admin' : '/super-admin';
    roles[role].routes = ['homepage', 'hero', 'categories', 'templates', 'theme', 'navigation', 'media', 'pages', 'settings', 'promotions', 'publishing'].map(tab => `${root}/storefront?draft=${draft}&tab=${tab}`);
  }
}

async function main() {
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch();
  const report = { baseURL, createdAt: new Date().toISOString(), screens: [] };
  try {
    for (const [role, config] of Object.entries(roles)) {
      if (process.env.WEEVCRAFTS_AUDIT_ROLES && !process.env.WEEVCRAFTS_AUDIT_ROLES.split(',').includes(role)) continue;
      const context = await browser.newContext({ baseURL, reducedMotion: 'reduce' });
      const response = await context.request.post('/api/v1/auth/login', { data: { email: `demo.${config.account}@weevcrafts.local`, password: 'WeeVCrafts-Demo-2026!' } });
      if (!response.ok()) throw new Error(`${role} login: ${response.status()}`);
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      const routes = [...config.routes];
      // Discover real, owned detail screens from the existing dataset.
      for (const route of routes) {
        const entry = { role, route, widths: [] };
        try {
          const response = await page.goto(route, { waitUntil: 'load', timeout: 30000 });
          entry.status = response.status(); entry.destination = new URL(page.url()).pathname;
          entry.title = await page.title();
          if (route.endsWith('/products') || route === '/orders' || route.endsWith('/orders')) {
            const details = await page.locator('main a[href]').evaluateAll(links => links.map(a => a.getAttribute('href')).filter(h => /\/(products\/\d+\/edit|orders\/[^/?]+)(\?|$)/.test(h)));
            for (const detail of [...new Set(details)].slice(0, 2)) if (!routes.includes(detail)) routes.push(detail);
          }
          for (const width of [1440, 390, 320]) {
            await page.setViewportSize({ width, height: width > 800 ? 1000 : 844 });
            await page.evaluate(async () => { await document.fonts.ready; await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
            const geometry = await page.evaluate(() => ({
              overflow: document.documentElement.scrollWidth > innerWidth + 1,
              overflowElements: [...document.querySelectorAll('main *')].filter(el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width && s.position !== 'fixed' && (r.right > innerWidth + 2 || r.left < -2) && !el.closest('[hidden], [inert], .ui-table-wrap, .storefront-preview, .product-rail, .home-rail'); }).slice(0, 8).map(el => ({ tag: el.tagName, class: el.className, text: el.textContent.trim().slice(0, 70) })),
              unnamed: [...document.querySelectorAll('button, input:not([type=hidden]), select, textarea')].filter(el => el.getClientRects().length && !el.closest('[inert]') && !el.getAttribute('aria-label') && !el.getAttribute('aria-labelledby') && !(el.labels?.length) && !(el.tagName === 'BUTTON' && el.textContent.trim())).map(el => el.outerHTML.slice(0, 180)),
            }));
            const image = `${role}-${route.replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '')}-${width}.png`;
            if (width !== 320) await page.screenshot({ path: path.join(output, image), fullPage: true });
            entry.widths.push({ width, ...geometry, screenshot: width === 320 ? null : image });
          }
          if (process.env.WEEVCRAFTS_AUDIT_AXE !== 'false') {
            await page.addScriptTag({ path: require.resolve('axe-core/axe.min.js') });
            entry.accessibility = [];
            for (const width of (process.env.WEEVCRAFTS_AUDIT_MOBILE_AXE === 'true' ? [1440, 390] : [1440])) {
              await page.setViewportSize({ width, height: width > 800 ? 1000 : 844 });
              const violations = await page.evaluate(async () => (await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'] } })).violations.map(v => ({ id: v.id, impact: v.impact, description: v.help, nodes: v.nodes.map(n => ({ target: n.target, html: n.html, summary: n.failureSummary })) })));
              entry.accessibility.push(...violations.map(v => ({ width, ...v })));
            }
          }
        } catch (error) { entry.error = error.message; }
        entry.pageErrors = errors.splice(0);
        report.screens.push(entry);
        await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
        console.log(`${role} ${route}: ${entry.status || entry.error} overflow=${entry.widths.filter(w => w.overflow).map(w => w.width)} axe=${entry.accessibility?.map(v => v.id).join(',') || '-'}`);
      }
      // Detail routes discovered above are recorded separately to avoid hidden fixture assumptions.
      const discovered = routes.filter(r => !config.routes.includes(r));
      await fs.writeFile(path.join(output, `${role}-detail-routes.json`), JSON.stringify(discovered, null, 2));
      await context.close();
    }
  } finally { await browser.close(); }
  const failures = report.screens.filter(s => s.error || s.status !== 200 || s.widths.some(w => w.overflow) || s.accessibility?.length || s.pageErrors.length);
  console.log(`${report.screens.length} screens; ${failures.length} with automated findings.`);
  if (failures.length) process.exitCode = 1;
  console.log(`Audit evidence: ${output}`);
}
main().catch(error => { console.error(error); process.exitCode = 1; });
