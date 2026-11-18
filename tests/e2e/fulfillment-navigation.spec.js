const { test, expect } = require('@playwright/test');
const axe = require('axe-core');

test('guided fulfillment uses the shared sidebar and preserves collapse across workspaces', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/super-admin/navigation-reference');
  const readSidebar = () => page.locator('#admin-sidebar').evaluate(sidebar => ({
    background: getComputedStyle(sidebar).backgroundColor,
    width: sidebar.getBoundingClientRect().width,
    links: [...sidebar.querySelectorAll('nav > a')].map(a => [a.getAttribute('href'), a.getAttribute('aria-label')]),
  }));
  const standard = await readSidebar();
  await page.goto('/super-admin/fulfillment');
  expect(await readSidebar()).toEqual(standard);
  await expect(page.locator('#admin-sidebar [aria-current="page"]')).toHaveCount(1);
  await expect(page.locator('#admin-sidebar [aria-current="page"]')).toHaveAttribute('href', '/super-admin/fulfillment');
  await page.getByRole('button', { name: 'Collapse sidebar', exact: true }).click();
  await expect(page.locator('.admin-shell')).toHaveClass(/is-collapsed/);
  const collapsedWidth = (await readSidebar()).width;
  expect(collapsedWidth).toBeLessThan(standard.width);
  await page.goto('/super-admin/navigation-reference');
  await expect(page.locator('.admin-shell')).toHaveClass(/is-collapsed/);
  await page.goto('/super-admin/fulfillment');
  await expect(page.locator('.admin-shell')).toHaveClass(/is-collapsed/);
  expect((await readSidebar()).width).toBe(collapsedWidth);
});

test('pilot setup explains the production switches and shared mobile navigation works', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto('/super-admin/fulfillment');
  await page.getByRole('button', { name: 'Open admin navigation', exact: true }).click();
  await expect(page.locator('#admin-sidebar').getByRole('link', { name: 'User Management', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'Open admin navigation', exact: true })).toBeFocused();
  await page.getByText('How to enable the one-order pilot', { exact: true }).click();
  await expect(page.locator('.guided-workspace')).toContainText('WECRATFS_CHECKOUT_ENABLED=true');
  await expect(page.locator('.guided-workspace')).toContainText('WECRATFS_SHIPROCKET_BOOKINGS_ENABLED=true');
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2)).toBe(false);
  await page.addScriptTag({ content: axe.source });
  const audit = await page.evaluate(async () => {
    const result = await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'] } });
    return result.violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => n.target) }));
  });
  expect(audit).toEqual([]);
});
