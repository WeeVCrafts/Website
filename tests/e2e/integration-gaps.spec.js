const { test, expect } = require("@playwright/test");

const password = process.env.WEEVCRAFTS_ROLE_TEST_PASSWORD || "WeeVCrafts-Demo-2026!";
const launch = process.env.WEEVCRAFTS_ROLE_TEST_ACCOUNT_SET === "launch";

async function signIn(page, email) {
  await page.goto("/login");
  await page.locator('#page input[name="email"]').fill(email);
  await page.locator('#page input[name="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname !== "/login");
}

test("category selection can be cleared and product structured data is valid", async ({ page }) => {
  await page.goto("/products?category=wellness");
  const category = page.locator('input[type="checkbox"][name="category"][value="wellness"]');
  await expect(category).toBeChecked();
  await category.uncheck();
  await page.waitForURL(url => !url.searchParams.has("category"));
  await expect(category).not.toBeChecked();
  await page.goto("/products/natural-soap");
  const schema = JSON.parse(await page.locator('script[type="application/ld+json"]').textContent());
  expect(schema["@type"]).toBe("Product");
  expect(schema.name).toBeTruthy();
});

test("admin directories persist catalog changes and settings affect storefront", async ({ page }) => {
  await signIn(page, launch ? "superadmin@weevcrafts.test" : "demo.super01@weevcrafts.local");
  for (const route of ["customers", "brands", "category-management", "commission-rules", "settings", "reviews"]) {
    const response = await page.goto(`/super-admin/${route}`);
    expect(response.status()).toBe(200);
    await expect(page.locator("main")).not.toContainText("preview workspace");
  }
  const missing = await page.request.get("/super-admin/does-not-exist");
  expect(missing.status()).toBe(404);
  await page.goto("/super-admin/analytics");
  for (const service of ["Database", "Redis", "Search", "Storage"]) {
    const status = page.locator('.admin-table .table-row').filter({ has: page.locator('strong', { hasText: new RegExp(`^${service}$`) }) });
    await expect(status).toContainText("Ready");
  }
  await expect(page.locator('.admin-table .table-row').filter({ hasText: "Payment mode" })).toContainText("local");

  const name = `Backend QA Brand ${Date.now()}`;
  const slug = name.toLowerCase().replaceAll(" ", "-");
  await page.goto("/super-admin/brands");
  const token = await page.locator('input[name="csrf_token"]').first().inputValue();
  const post = async (route, form) => page.request.post(route, { form: { csrf_token: token, reason: "Backend integration regression check", ...form }, maxRedirects: 0 });
  expect((await page.request.post("/super-admin/brands/create", { form: { name, reason: "Must reject missing CSRF" }, maxRedirects: 0 })).status()).toBe(403);
  expect((await post("/super-admin/brands/create", { name, id: slug })).status()).toBe(303);
  try {
    await page.goto(`/super-admin/brands?q=${encodeURIComponent(name)}`);
    await expect(page.locator("main")).toContainText(name);
    await page.reload();
    await expect(page.locator("main")).toContainText(name);
    expect((await post("/super-admin/brands/update", { id: slug, name: `${name} Updated` })).status()).toBe(303);
    await page.goto(`/super-admin/brands?q=${encodeURIComponent(name)}`);
    await expect(page.locator("main")).toContainText(`${name} Updated`);
  } finally {
    expect((await post("/super-admin/brands/delete", { id: slug })).status()).toBe(303);
  }

  await page.goto("/super-admin/settings");
  const email = await page.locator('[name="support_email"]').inputValue();
  const shipping = await page.locator('[name="shipping_information"]').inputValue();
  const changedEmail = `qa-${Date.now()}@weevcrafts.test`;
  const changedShipping = `Persisted delivery instructions QA ${Date.now()}`;
  try {
    expect((await post("/super-admin/settings/save", { support_email: changedEmail, shipping_information: changedShipping })).status()).toBe(303);
    await page.goto("/contact");
    await expect(page.locator("main")).toContainText(changedEmail);
    await page.goto("/shipping");
    await expect(page.locator("main")).toContainText(changedShipping);
  } finally {
    expect((await post("/super-admin/settings/save", { support_email: email, shipping_information: shipping })).status()).toBe(303);
  }
});

test("marketplace cannot change commissions and finance can open the editor", async ({ page }) => {
  await signIn(page, launch ? "marketplaceadmin@weevcrafts.test" : "demo.marketplace01@weevcrafts.local");
  expect((await page.request.get("/marketplace-admin/commission-rules")).status()).toBe(403);
  expect((await page.request.post("/marketplace-admin/commission-rules/create", {
    form: { name: "Unauthorized commission", basis_points: "100", reason: "Role boundary check" }, maxRedirects: 0,
  })).status()).toBe(403);
  await page.context().clearCookies();
  await signIn(page, launch ? "finance@weevcrafts.test" : "demo.finance01@weevcrafts.local");
  expect((await page.goto("/finance-portal/commission-rules")).status()).toBe(200);
  await expect(page.locator('form[action="/finance-portal/commission-rules/create"]')).toHaveCount(1);
});
