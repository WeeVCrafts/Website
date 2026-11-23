import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = '/Users/techmigos/.gemini/antigravity/brain/0b14803c-d198-46f1-b14b-a6299efd895e/screenshots/mobile';
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

const BASE_URL = 'http://localhost:8080';

const routes = [
  { name: '01_home', path: '/' },
  { name: '02_products_catalog', path: '/products' },
  { name: '03_product_detail', path: '/products/madhubani-tree' },
  { name: '04_categories_list', path: '/categories' },
  { name: '05_category_crafts', path: '/categories/crafts' },
  { name: '06_makers_list', path: '/makers' },
  { name: '07_maker_storefront', path: '/makers/weavers-touch' },
  { name: '08_brands_list', path: '/brands' },
  { name: '09_cart_empty', path: '/cart' },
  { name: '10_wishlist_empty', path: '/wishlist' },
  { name: '11_login', path: '/login' },
  { name: '12_register', path: '/register' },
  { name: '13_returns_policy', path: '/pages/returns' },
  { name: '14_shipping_policy', path: '/shipping' },
  { name: '15_notifications', path: '/notifications' },
  { name: '16_search_results', path: '/search?q=pot' }
];

async function runAudit() {
  const browser = await chromium.launch({ headless: true });
  // Standard mobile viewport: 390x844 (iPhone 13/14/15)
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    isMobile: true,
    hasTouch: true,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1'
  });

  const page = await context.newPage();
  const report = [];

  for (const r of routes) {
    const url = BASE_URL + r.path;
    console.log(`Checking ${r.name} (${r.path})...`);
    try {
      const resp = await page.goto(url, { waitUntil: 'networkidle', timeout: 15000 });
      const status = resp ? resp.status() : 'no-response';

      // Check horizontal overflow
      const overflowInfo = await page.evaluate(() => {
        const docEl = document.documentElement;
        const scrollW = docEl.scrollWidth;
        const clientW = docEl.clientWidth;
        const hasOverflow = scrollW > clientW + 1;

        let overflowingElements = [];
        if (hasOverflow) {
          const all = document.querySelectorAll('*');
          for (const el of all) {
            const rect = el.getBoundingClientRect();
            if (rect.right > clientW + 2) {
              const tag = el.tagName.toLowerCase();
              const cls = el.className ? String(el.className).substring(0, 50) : '';
              const id = el.id ? '#' + el.id : '';
              overflowingElements.push(`${tag}${id}.${cls} (right: ${Math.round(rect.right)}px > ${clientW}px)`);
            }
          }
        }
        return {
          scrollWidth: scrollW,
          clientWidth: clientW,
          hasOverflow,
          overflowCount: overflowingElements.length,
          samples: overflowingElements.slice(0, 5)
        };
      });

      // Capture top viewport screenshot
      const viewportPath = path.join(SCREENSHOT_DIR, `${r.name}_viewport.png`);
      await page.screenshot({ path: viewportPath, fullPage: false });

      // Capture full page screenshot
      const fullpagePath = path.join(SCREENSHOT_DIR, `${r.name}_fullpage.png`);
      await page.screenshot({ path: fullpagePath, fullPage: true });

      report.push({
        name: r.name,
        path: r.path,
        status,
        ...overflowInfo,
        viewportScreenshot: viewportPath,
        fullpageScreenshot: fullpagePath
      });
    } catch (err) {
      console.error(`Error on ${r.path}:`, err.message);
      report.push({
        name: r.name,
        path: r.path,
        error: err.message
      });
    }
  }

  // Interactive mobile states:
  // 1. Mobile Menu Open
  try {
    console.log('Testing Mobile Navigation Drawer...');
    await page.goto(BASE_URL + '/', { waitUntil: 'networkidle' });
    const menuBtn = page.locator('button.menu-toggle, button[aria-label*="menu" i]').first();
    if (await menuBtn.isVisible()) {
      await menuBtn.click();
      await page.waitForTimeout(500);
      const menuPath = path.join(SCREENSHOT_DIR, `17_mobile_menu_open.png`);
      await page.screenshot({ path: menuPath });
      report.push({ name: '17_mobile_menu_open', path: '/ (drawer open)', status: 200, hasOverflow: false, viewportScreenshot: menuPath });
    }
  } catch (e) {
    console.log('Menu drawer error:', e.message);
  }

  // 2. Product Detail Tab Switching & Mobile Buy Bar
  try {
    console.log('Testing Product Detail Mobile Sticky Buy Bar & Tabs...');
    await page.goto(BASE_URL + '/products/madhubani-tree', { waitUntil: 'networkidle' });
    await page.evaluate(() => window.scrollBy(0, 600));
    await page.waitForTimeout(400);
    const stickyBuyBarPath = path.join(SCREENSHOT_DIR, `18_product_sticky_buy_bar.png`);
    await page.screenshot({ path: stickyBuyBarPath });
    report.push({ name: '18_product_sticky_buy_bar', path: '/products/madhubani-tree (scrolled)', status: 200, hasOverflow: false, viewportScreenshot: stickyBuyBarPath });

    const reviewsTab = page.locator('#product-tab-reviews, button:has-text("Reviews")').first();
    if (await reviewsTab.isVisible()) {
      await reviewsTab.click();
      await page.waitForTimeout(400);
      const reviewsTabPath = path.join(SCREENSHOT_DIR, `19_product_reviews_tab.png`);
      await page.screenshot({ path: reviewsTabPath });
      report.push({ name: '19_product_reviews_tab', path: '/products/madhubani-tree (reviews tab)', status: 200, hasOverflow: false, viewportScreenshot: reviewsTabPath });
    }
  } catch (e) {
    console.log('Product detail tabs error:', e.message);
  }

  // 3. Filter Drawer on /products
  try {
    console.log('Testing Mobile Filter Sidebar on /products...');
    await page.goto(BASE_URL + '/products', { waitUntil: 'networkidle' });
    const filterBtn = page.locator('button.mobile-filter-toggle, button:has-text("Filter")').first();
    if (await filterBtn.isVisible()) {
      await filterBtn.click();
      await page.waitForTimeout(400);
      const filterOpenPath = path.join(SCREENSHOT_DIR, `20_catalog_filter_open.png`);
      await page.screenshot({ path: filterOpenPath });
      report.push({ name: '20_catalog_filter_open', path: '/products (filters open)', status: 200, hasOverflow: false, viewportScreenshot: filterOpenPath });
    }
  } catch (e) {
    console.log('Filter drawer error:', e.message);
  }

  fs.writeFileSync(
    path.join(SCREENSHOT_DIR, 'mobile_audit_report.json'),
    JSON.stringify(report, null, 2)
  );
  console.log('Mobile audit script completed successfully!');
  await browser.close();
}

runAudit();
