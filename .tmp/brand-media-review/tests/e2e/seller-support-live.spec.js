const { test, expect } = require("@playwright/test");

const password = process.env.WEEVCRAFTS_ROLE_TEST_PASSWORD || "WeeVCrafts-Demo-2026!";
const launch = process.env.WEEVCRAFTS_ROLE_TEST_ACCOUNT_SET === "launch";
const owner = launch ? "seller@weevcrafts.test" : "demo.owner01@weevcrafts.local";
const staff = launch ? "sellerstaff@weevcrafts.test" : "demo.staff01@weevcrafts.local";
const customer = launch ? "customer@weevcrafts.test" : "demo.customer01@weevcrafts.local";

async function signIn(page, email) {
  await page.goto("/login");
  await page.locator('#page input[name="email"]').fill(email);
  await page.locator('#page input[name="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname !== "/login");
}

test("seller support saves a real request and reply across reloads", async ({ page }) => {
  const subject = `Seller support backend QA ${Date.now()}`;
  const initialMessage = "Please check my real catalogue request and preserve this conversation.";
  const replyMessage = `Seller follow-up persisted ${Date.now()}`;
  await signIn(page, owner);
  await page.goto("/seller-admin/support");
  const create = page.locator('form[action="/seller-admin/support/create"]');
  await create.locator('[name="subject"]').fill(subject);
  await create.locator('[name="message"]').fill(initialMessage);
  await create.getByRole("button", { name: "Send request" }).click();
  await page.waitForURL(url => url.pathname === "/seller-admin/support");
  const ticket = page.locator('article[id^="ticket-"]').filter({ hasText: subject });
  await expect(ticket).toContainText(initialMessage);
  await ticket.locator('textarea[name="message"]').fill(replyMessage);
  await ticket.getByRole("button", { name: "Send reply" }).click();
  await page.waitForURL(url => url.pathname === "/seller-admin/support");
  await page.reload();
  await expect(page.locator('article[id^="ticket-"]').filter({ hasText: subject })).toContainText(replyMessage);

  const filters = page.locator('form[action="/seller-admin/support"]').last();
  await filters.locator('[name="q"]').fill(replyMessage);
  await filters.getByRole("button", { name: "Apply filters" }).click();
  await expect(page.locator('article[id^="ticket-"]')).toHaveCount(1);
  await expect(page.locator('article[id^="ticket-"]')).toContainText(subject);
});

for (const [role, email] of [["seller staff", staff], ["customer", customer]]) {
  test(`${role} cannot access seller owner support`, async ({ page }) => {
    await signIn(page, email);
    const response = await page.request.get("/seller-admin/support", { maxRedirects: 0 });
    expect(response.status()).toBe(403);
    const mutation = await page.request.post("/seller-admin/support/create", {
      form: { subject: "Unauthorized request", message: "This request must never persist." },
      maxRedirects: 0,
    });
    expect(mutation.status()).toBe(403);
  });
}
