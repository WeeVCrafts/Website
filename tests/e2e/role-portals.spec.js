const { test, expect } = require("@playwright/test");
const fs = require("node:fs");
const path = require("node:path");

const password = process.env.WEEVCRAFTS_ROLE_TEST_PASSWORD || "WeeVCrafts-Demo-2026!";
const accountSet = process.env.WEEVCRAFTS_ROLE_TEST_ACCOUNT_SET || "demo";
const viewportWidths = [360, 390, 768, 1024, 1440];
const demoAccounts = [
  { id: "customer", email: "demo.customer01@weevcrafts.local", name: "Priya Sharma", landing: "/account", page: "/returns", roleText: "My account", forbidden: "/super-admin" },
  { id: "seller", email: "demo.owner01@weevcrafts.local", name: "Aditi Rao", landing: "/seller-admin", page: "/seller-admin/products/new", roleText: "Seller", forbidden: "/admin" },
  { id: "admin", email: "demo.marketplace01@weevcrafts.local", name: "Marketplace Reviewer", landing: "/admin", page: "/admin/products", roleText: "Admin", forbidden: "/super-admin" },
  { id: "super-admin", email: "demo.super01@weevcrafts.local", name: "Super Admin One", landing: "/super-admin", page: "/super-admin/security", roleText: "Super Admin", forbidden: "/seller-admin" },
];

const launchAccounts = [
  { id: "customer", email: "customer@weevcrafts.test", name: "QA Customer", landing: "/account", page: "/returns", roleText: "My account", forbidden: "/super-admin" },
  { id: "seller", email: "seller@weevcrafts.test", name: "QA Seller", landing: "/seller-admin", page: "/seller-admin/products/new", roleText: "Seller", forbidden: "/admin" },
  { id: "admin", email: "admin@weevcrafts.test", name: "QA Admin", landing: "/admin", page: "/admin/products", roleText: "Admin", forbidden: "/super-admin" },
  { id: "super-admin", email: "superadmin@weevcrafts.test", name: "QA Super Admin", landing: "/super-admin", page: "/super-admin/security", roleText: "Super Admin", forbidden: "/seller-admin" },
];

const accounts = accountSet === "launch" ? launchAccounts : demoAccounts;

async function signIn(page, email, landing) {
  await page.goto("/login");
  await page.locator("#page input[name=\"email\"]").fill(email);
  await page.locator("#page input[name=\"password\"]").fill(password);
  await page.getByRole("button", { name: /continue with email/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, "") === (landing === "/account" ? "/products" : landing).replace(/\/$/, ""), { timeout: 30_000 });
  if (landing === "/account") await page.goto("/account");
}

test.describe("role-specific portals", () => {
  for (const account of accounts) {
    test(`${account.id}: landing, boundaries and responsive layouts`, async ({ page }, testInfo) => {
      const browserErrors = [];
      page.on("pageerror", error => browserErrors.push(error.message));
      page.on("console", message => { if (message.type() === "error") browserErrors.push(message.text()); });
      page.on("response", response => {
        if (response.status() === 404) browserErrors.push(`404 ${new URL(response.url()).pathname}`);
      });
      await signIn(page, account.email, account.landing);
      const landingPattern = new RegExp(`${account.landing.replaceAll("/", "\\/")}\\/?$`);
      await expect(page).toHaveURL(landingPattern);
      await expect(page.locator("body")).toContainText(account.name);
      await expect(page.locator("body")).toContainText(account.roleText);

      const forbidden = await page.request.get(account.forbidden, { maxRedirects: 0 });
      expect(forbidden.status(), `expected ${account.id} to be blocked from ${account.forbidden}`).toBe(403);

      for (const width of viewportWidths) {
        await page.setViewportSize({ width, height: 1000 });
        const response = await page.goto(account.page);
        expect(response && response.status(), `${account.id} page ${account.page} at ${width}px`).toBe(200);

        if (width <= 820 && account.id !== "customer") {
          const menu = page.locator('button[aria-label="Open seller navigation"], button[aria-label="Open admin navigation"], button[aria-label="Open support navigation"]').first();
          await expect(menu).toBeVisible();
          await menu.click();
          await expect(menu).toHaveAttribute("aria-expanded", "true");
          const sidebarId = account.id.startsWith("seller") ? "seller-sidebar" : "admin-sidebar";
          const close = page.locator(`#${sidebarId} button[aria-label^="Close"]`);
          await expect(close).toBeVisible();
          await close.click();
          await expect(menu).toHaveAttribute("aria-expanded", "false");
        }

        const dimensions = await page.evaluate(() => ({
          viewport: window.innerWidth,
          document: document.documentElement.scrollWidth,
          body: document.body.scrollWidth,
        }));
        expect(dimensions.document, `${account.id} document overflow at ${width}px: ${JSON.stringify(dimensions)}`).toBeLessThanOrEqual(width + 1);
        expect(dimensions.body, `${account.id} body overflow at ${width}px: ${JSON.stringify(dimensions)}`).toBeLessThanOrEqual(width + 1);

        const screenshotPath = testInfo.outputPath(`${account.id}-${width}.png`);
        fs.mkdirSync(path.dirname(screenshotPath), { recursive: true });
        await page.screenshot({ path: screenshotPath });
      }

      if (account.id === "seller") {
        await expect(page.getByRole("navigation", { name: "Seller portal navigation" })).not.toContainText("Team");
      }
      if (account.id === "seller") {
        await page.goto("/seller-admin");
        const policies = page.getByRole("navigation", { name: "Policies" });
        await expect(policies.getByRole("link", { name: "Seller Terms & Conditions" })).toBeVisible();
        await expect(policies.getByRole("link", { name: "Privacy Policy" })).toBeVisible();
        await policies.getByRole("link", { name: "Seller Terms & Conditions" }).click();
        await expect(page).toHaveURL(/\/seller-admin\/terms$/);
        await expect(page.getByRole("heading", { name: "6. Code of conduct & account termination" })).toBeVisible();
        await page.getByRole("navigation", { name: "Policies" }).getByRole("link", { name: "Privacy Policy" }).click();
        await expect(page).toHaveURL(/\/privacy$/);
        await expect(page.locator("body")).toContainText("Privacy policy");
      }
      if (account.id === "super-admin") {
        await expect(page.getByRole("navigation", { name: "Admin navigation" })).toContainText("Security");
      }
      expect(browserErrors, `${account.id} browser errors`).toEqual([]);
    });
  }
});
