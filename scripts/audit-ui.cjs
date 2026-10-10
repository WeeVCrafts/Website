// Read-only route/accessibility survey. Business forms are never submitted.
// Usage: node scripts/audit-ui.cjs [output-directory] [axe.min.js]
const { chromium } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const baseURL = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080';
const output = path.resolve(process.argv[2] || 'docs/qa/UI-20261009');
const axePath = path.resolve(process.argv[3] || 'tmp/ui-audit/package/axe.min.js');
const roles = [
  { name: 'guest', root: '/', paths: ['/', '/products', '/products/blue-tote', '/products?category=offers', '/products?category=arts', '/search?q=tote', '/search?q=ui-audit-no-such-product', '/categories', '/category/natural-care', '/brands', '/makers/mithila-arts', '/deals', '/about', '/contact', '/faq', '/shipping', '/size-guide', '/terms', '/privacy', '/sustainability', '/press', '/careers', '/login', '/register', '/forgot-password', '/reset-password', '/verify-email'] },
  { name: 'customer', email: 'customer01', root: '/account', paths: ['/account', '/account/profile', '/account/addresses', '/account/notifications', '/account/security', '/account/privacy', '/account/reviews', '/wishlist', '/cart', '/checkout', '/orders', '/tracking', '/returns', '/payments', '/support'] },
  { name: 'seller-owner', email: 'owner01', root: '/seller-admin', paths: ['', '/products', '/products/new', '/inventory', '/orders', '/returns', '/earnings', '/analytics', '/team', '/settings', '/onboarding', '/support', '/terms'] },
  { name: 'seller-staff', email: 'staff01', root: '/seller-admin', paths: ['', '/products', '/inventory', '/orders', '/returns', '/earnings', '/analytics', '/onboarding', '/terms'] },
  { name: 'marketplace-admin', email: 'marketplace01', root: '/marketplace-admin', paths: ['', '/sellers', '/seller-approvals', '/seller-suspensions', '/products', '/inventory', '/orders', '/returns', '/brands', '/category-management', '/customers', '/reviews', '/marketing', '/analytics', '/audit-logs', '/settings', '/system-health', '/search-indexing-health', '/worker-queue-health', '/support'] },
  { name: 'super-admin', email: 'super01', root: '/super-admin', paths: ['', '/sellers', '/seller-approvals', '/seller-suspensions', '/products', '/inventory', '/orders', '/returns', '/brands', '/category-management', '/customers', '/reviews', '/marketing', '/analytics', '/audit-logs', '/settings', '/system-health', '/search-indexing-health', '/worker-queue-health', '/support', '/security', '/roles', '/security-events', '/payments', '/failed-payments', '/refunds', '/settlements', '/finance-reconciliation', '/commission-rules', '/finance'] },
  { name: 'support-agent', email: 'support01', root: '/support-portal', paths: ['', '/cases', '/customers', '/orders'] },
  { name: 'finance-operator', email: 'finance01', root: '/finance-portal', paths: ['', '/payments', '/failed-payments', '/refunds', '/settlements', '/finance-reconciliation', '/commission-rules'] },
  { name: 'returns-operations', email: 'operations01', root: '/operations-portal', paths: ['', '/orders', '/returns', '/refunds', '/disputes'] },
];
const screenshotPaths = new Set(['/', '/products', '/products/blue-tote', '/login', '/register', '/account', '/account/addresses', '/cart', '/checkout', '/seller-admin', '/seller-admin/products', '/seller-admin/products/new', '/seller-admin/orders', '/seller-admin/earnings', '/seller-admin/team', '/marketplace-admin', '/super-admin', '/support-portal', '/finance-portal', '/operations-portal']);
const report = { startedAt: new Date().toISOString(), baseURL, browser: '', widths: [1440, 390], roles: [], pages: [], errors: [] };
const existingReport = path.join(output, 'browser-audit.json');
if (fs.existsSync(existingReport)) {
  const previous = JSON.parse(fs.readFileSync(existingReport, 'utf8'));
  if (previous.baseURL === baseURL && (!previous.finishedAt || process.env.UI_AUDIT_RESUME === '1')) {
    Object.assign(report, previous);
    delete report.finishedAt;
  }
}

function save() { fs.writeFileSync(path.join(output, 'browser-audit.json'), JSON.stringify(report, null, 2)); }
function safeName(value) { return value.replace(/[^a-zA-Z0-9]+/g, '-').replace(/^-|-$/g, '') || 'home'; }

async function inspect(page, role, route, width) {
  const cached = report.pages.find(p => p.role === role.name && p.route === route && p.width === width && p.dom);
  if (cached) return cached;
  await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
  const runtimeErrors = [], failedResources = [];
  const onError = error => runtimeErrors.push(error.message);
  const onResponse = response => { if (response.status() >= 400 && /\/(assets|media)\//.test(response.url())) failedResources.push({ url: response.url(), status: response.status() }); };
  page.on('pageerror', onError); page.on('response', onResponse);
  let result;
  try {
    const response = await page.goto(new URL(route, baseURL).href, { waitUntil: 'networkidle', timeout: 30000 });
    const runAccessibility = width === 1440 || screenshotPaths.has(route);
    if (runAccessibility) await page.addScriptTag({ path: axePath });
    const accessibility = runAccessibility ? await page.evaluate(async () => {
      const result = await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'best-practice'] } });
      return {
        violations: result.violations.map(v => ({ id: v.id, impact: v.impact, help: v.help, helpUrl: v.helpUrl, tags: v.tags, count: v.nodes.length, nodes: v.nodes.slice(0, 5).map(n => ({ target: n.target, html: n.html, summary: n.failureSummary })) })),
        incomplete: result.incomplete.map(v => ({ id: v.id, count: v.nodes.length })), passes: result.passes.length,
      };
    }) : { notRun: 'Desktop accessibility scan; mobile layout survey only on this route.' };
    const dom = await page.evaluate(() => {
      const visible = el => { const r = el.getBoundingClientRect(), s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none' && !el.closest('[inert]'); };
      const describe = el => ({ tag: el.tagName.toLowerCase(), text: (el.innerText || el.getAttribute('aria-label') || el.getAttribute('placeholder') || '').trim().slice(0, 100), id: el.id, class: el.className, href: el.getAttribute('href') });
      const links = [...document.querySelectorAll('a[href]')].map(el => ({ text: (el.innerText || el.getAttribute('aria-label') || '').trim().slice(0, 100), href: el.getAttribute('href') }));
      const controls = [...document.querySelectorAll('a,button,input:not([type=hidden]),select,textarea,summary')].filter(visible);
      return {
        title: document.title, h1: [...document.querySelectorAll('h1')].map(el => el.innerText),
        headings: [...document.querySelectorAll('h1,h2,h3,h4')].filter(visible).map(el => ({ level: el.tagName, text: el.innerText.slice(0, 120) })),
        bodyWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth,
        overflow: [...document.querySelectorAll('body *')].filter(el => visible(el) && el.getBoundingClientRect().right > innerWidth + 2 && !el.closest('[class*="table"],[class*="sidebar"],[class*="overflow-x"]')).slice(0, 8).map(describe),
        smallTargets: controls.filter(el => { const r = el.getBoundingClientRect(); return r.width < 44 || r.height < 44; }).slice(0, 30).map(el => ({ ...describe(el), width: Math.round(el.getBoundingClientRect().width), height: Math.round(el.getBoundingClientRect().height) })),
        brokenImages: [...document.images].filter(el => visible(el) && el.complete && !el.naturalWidth).map(el => ({ src: el.src, alt: el.alt })),
        images: [...document.images].map(el => ({ src: el.src, width: el.getAttribute('width'), height: el.getAttribute('height'), loading: el.getAttribute('loading'), naturalWidth: el.naturalWidth })),
        inertControls: controls.filter(el => (el.tagName === 'A' && ['#', '', 'javascript:void(0)'].includes(el.getAttribute('href'))) || (el.tagName === 'BUTTON' && el.type === 'button' && !['onclick', '@click', 'x-on:click', 'hx-get', 'hx-post', 'data-action', 'data-open-dialog'].some(a => el.hasAttribute(a)))).map(describe),
        links, forms: [...document.forms].map(f => ({ action: f.getAttribute('action'), method: f.method, fields: [...f.elements].filter(el => el.type !== 'hidden').map(el => ({ tag: el.tagName, name: el.name, type: el.type, required: el.required, labels: [...(el.labels || [])].map(l => l.innerText), autocomplete: el.autocomplete })) })),
        resources: performance.getEntriesByType('resource').map(r => ({ name: r.name, bytes: r.transferSize, duration: Math.round(r.duration) })),
      };
    });
    result = { role: role.name, route, width, status: response.status(), finalURL: page.url(), accessibility, dom, runtimeErrors, failedResources };
    if (screenshotPaths.has(route)) {
      const file = `${role.name}-${safeName(route)}-${width}.png`;
      await page.screenshot({ path: path.join(output, 'screenshots', file), fullPage: true });
      await page.screenshot({ path: path.join(output, 'screenshots', file.replace('.png', '-viewport.png')), fullPage: false });
      result.screenshot = `screenshots/${file}`;
    }
    // Opening and closing responsive navigation is reversible.
    if (width === 390 && route === role.root) {
      const menu = page.getByRole('button', { name: /Open .*navigation|Open menu|Menu/i }).first();
      if (await menu.count() && await menu.isVisible()) {
        await menu.click();
        result.mobileNavigation = await page.evaluate(() => ({ focus: document.activeElement?.outerHTML.slice(0, 350), expanded: [...document.querySelectorAll('[aria-expanded]')].map(el => ({ label: el.getAttribute('aria-label'), value: el.getAttribute('aria-expanded') })) }));
        await page.screenshot({ path: path.join(output, 'screenshots', `${role.name}-navigation-390.png`), fullPage: false });
        await page.keyboard.press('Escape');
        result.mobileNavigation.focusAfterEscape = await page.evaluate(() => document.activeElement?.getAttribute('aria-label'));
      }
    }
  } catch (error) {
    result = { role: role.name, route, width, error: error.message, runtimeErrors, failedResources };
  } finally {
    page.off('pageerror', onError); page.off('response', onResponse);
  }
  report.pages.push(result); save();
  return result;
}

(async () => {
  fs.mkdirSync(path.join(output, 'screenshots'), { recursive: true });
  const browser = await chromium.launch(); report.browser = browser.version();
  try {
    for (const role of roles.filter(role => !process.env.UI_AUDIT_ROLES || process.env.UI_AUDIT_ROLES.split(',').includes(role.name))) {
      console.log(`Inspecting ${role.name}`);
      const context = await browser.newContext({ reducedMotion: 'reduce' });
      const page = await context.newPage();
      try {
        if (role.email) {
          await page.goto(new URL('/login', baseURL).href);
          await page.locator('#page input[name=email]').fill(`demo.${role.email}@weevcrafts.local`);
          await page.locator('#page input[name=password]').fill('WeeVCrafts-Demo-2026!');
          await page.getByRole('button', { name: /sign in/i }).click();
          await page.waitForURL(url => url.pathname.replace(/\/$/, '') === role.root, { timeout: 20000 });
        }
        const routes = new Set(role.paths.map(p => role.root !== '/' && role.name !== 'customer' ? role.root + p : p));
        const visited = new Set();
        const recordTemplates = new Set();
        // Discover linked records and query-selected tabs, keeping only this role's surface.
        for (const route of routes) {
          if (visited.has(route)) continue;
          visited.add(route);
          const desktop = await inspect(page, role, route, 1440);
          await inspect(page, role, route, 390);
          for (const link of desktop.dom?.links || []) {
            try {
              const url = new URL(link.href, baseURL);
              if (url.origin !== new URL(baseURL).origin || url.hash || /logout|\/delete|\/archive|\/approve|\/reject|\/fulfill|\/receipt|\/label|\/export|\/attachments\//.test(url.pathname)) continue;
              const inScope = role.name === 'guest' ? /^\/(product\/|category\/|brands\/|sellers\/)/.test(url.pathname) : role.name === 'customer' ? /^\/(account\/|orders\/|returns\/|support\/)/.test(url.pathname) : url.pathname.startsWith(role.root + '/');
              const recordPattern = url.pathname.replace(/\/(product|category|brands|sellers)\/[^/]+/, '/$1/:record').replace(/\/(products|orders|cases|customers|returns)\/[^/]+(\/edit)?$/, '/$1/:record$2');
              if (recordPattern !== url.pathname && recordTemplates.has(recordPattern)) continue;
              // Limit pagination/search combinations; preserve meaningful tabs only.
              if (routes.size < 65 && inScope && (!url.search || [...url.searchParams.keys()].every(key => ['tab', 'view', 'status', 'period'].includes(key)))) {
                routes.add(url.pathname + url.search);
                if (recordPattern !== url.pathname) recordTemplates.add(recordPattern);
              }
            } catch {}
          }
        }
        report.roles = report.roles.filter(item => item.name !== role.name);
        report.roles.push({ name: role.name, routes: visited.size, authenticated: !!role.email });
        console.log(`${role.name}: ${visited.size} routes, desktop + mobile`);
      } catch (error) { report.errors.push({ role: role.name, error: error.message }); console.log(`${role.name}: ${error.message}`); }
      finally { await context.close(); save(); }
    }
  } finally { await browser.close(); report.finishedAt = new Date().toISOString(); save(); }
  console.log(JSON.stringify({ pages: report.pages.length, errors: report.errors, output }));
})().catch(error => { console.error(error); process.exitCode = 1; });
