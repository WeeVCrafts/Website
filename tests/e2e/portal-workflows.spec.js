const { test, expect } = require("@playwright/test");

const password = "WeeVCrafts-Demo-2026!";
const target = new URL(process.env.WEEVCRAFTS_BASE_URL || "http://127.0.0.1:8080");
const localMutationTarget = ["localhost", "127.0.0.1", "::1"].includes(target.hostname);

async function signIn(page, email, landing) {
  await page.goto("/login");
  await page.locator("#page input[name=\"email\"]").fill(email);
  await page.locator("#page input[name=\"password\"]").fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, "") === landing.replace(/\/$/, ""), { timeout: 30_000 });
}

async function submitRoleChange(card, role, action, reason) {
  const form = card.locator("form.role-assignment-form");
  await form.locator('select[name="role"]').selectOption(role);
  await form.locator('select[name="action"]').selectOption(action);
  await form.locator('input[name="reason"]').fill(reason);
  await form.getByRole("button", { name: "Save role change" }).click();
  if (action === "grant") {
    await expect(card.locator(".role-badges")).toContainText(role);
  } else {
    await expect(card.locator(".role-badges")).not.toContainText(role);
  }
}

test("multi-role login and workspace switcher expose only assigned roles", async ({ browser }) => {
  test.skip(!localMutationTarget, "Role assignment mutations are restricted to the local test deployment.");
  test.setTimeout(120_000);

  const superContext = await browser.newContext();
  const multiContext = await browser.newContext();
  const admin = await superContext.newPage();
  let addedFinanceRole = false;

  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/security");
    const card = admin.locator(".role-assignment-card").filter({ hasText: "demo.customer08@weevcrafts.local" });
    await expect(card).toHaveCount(1);
    const financeBadge = card.locator(".role-badges").getByText("finance_operator", { exact: true });
    if (!(await financeBadge.count())) {
      await submitRoleChange(card, "finance_operator", "grant", "Local E2E multi-role chooser verification");
      addedFinanceRole = true;
    }

    const customer = await multiContext.newPage();
    await signIn(customer, "demo.customer08@weevcrafts.local", "/workspaces");
    await expect(customer).toHaveURL(/\/workspaces\/?$/);
    const choices = customer.getByRole("navigation", { name: "Available workspaces" });
    await expect(choices.getByRole("link")).toHaveCount(2);
    await expect(choices).toContainText("Customer account");
    await expect(choices).toContainText("Finance");
    await expect(choices).not.toContainText("Super Admin");

    await choices.getByRole("link", { name: /Finance/ }).click();
    await expect(customer).toHaveURL(/\/finance-portal\/?$/);
    await customer.getByLabel("Switch workspace").click();
    await expect(customer).toHaveURL(/\/workspaces\/?$/);
    await expect(customer.getByRole("navigation", { name: "Available workspaces" }).getByRole("link")).toHaveCount(2);
  } finally {
    if (addedFinanceRole) {
      await admin.goto("/super-admin/security");
      const card = admin.locator(".role-assignment-card").filter({ hasText: "demo.customer08@weevcrafts.local" });
      await submitRoleChange(card, "finance_operator", "revoke", "Local E2E multi-role chooser cleanup");
    }
    await multiContext.close();
    await superContext.close();
  }
});

test("seller listing approval, mock-paid order, fulfilment and return handling work across roles", async ({ browser }) => {
  test.skip(!localMutationTarget, "Cross-role workflow mutations are restricted to the local test deployment.");
  test.setTimeout(240_000);

  const suffix = Date.now().toString();
  const productName = `Browser QA Craft ${suffix}`;
  const slug = `browser-qa-craft-${suffix}`;
  const sku = `QA-${suffix}`;
  const sellerContext = await browser.newContext();
  const marketplaceContext = await browser.newContext();
  const customerContext = await browser.newContext();
  const operationsContext = await browser.newContext();
  const seller = await sellerContext.newPage();
  const marketplace = await marketplaceContext.newPage();
  const customer = await customerContext.newPage();
  const operations = await operationsContext.newPage();

  try {
    await signIn(seller, "demo.owner01@weevcrafts.local", "/seller-admin");
    await seller.goto("/seller-admin/products/new");
    await seller.getByLabel("Product name").fill(productName);
    await seller.getByLabel("Product URL slug").fill(slug);
    await seller.getByLabel("Brand name").fill(`QA Artisan ${suffix}`);
    await seller.getByLabel("Brand slug").fill(`qa-artisan-${suffix}`);
    await seller.getByLabel("Category name").fill("Crafts");
    await seller.getByLabel("Category slug").fill("crafts");
    await seller.getByLabel("SKU").fill(sku);
    await seller.getByLabel("Price in paise").fill("249900");
    await seller.getByLabel("Initial stock").fill("5");
    await seller.getByLabel("Delivery note").fill("Delivery to serviceable PIN codes in India");
    await seller.getByLabel("Description").fill("A locally created browser test listing used to verify the WeeVCrafts cross-role order flow.");
    await seller.getByRole("button", { name: "Save draft" }).click();

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const sellerProduct = seller.locator(".seller-product-row").filter({ hasText: productName });
    await expect(sellerProduct).toHaveCount(1);
    await sellerProduct.getByRole("button", { name: "Submit for review" }).click();

    await signIn(marketplace, "demo.marketplace01@weevcrafts.local", "/marketplace-admin");
    await marketplace.goto(`/marketplace-admin/products?q=${encodeURIComponent(sku)}`);
    const reviewRow = marketplace.locator(".table-row").filter({ hasText: productName });
    await expect(reviewRow).toHaveCount(1);
    await reviewRow.getByRole("button", { name: "Approve" }).click();

    await signIn(customer, "demo.customer08@weevcrafts.local", "/account");
    await customer.goto(`/products/${slug}`);
    await expect(customer.getByRole("heading", { name: productName })).toBeVisible();
    const cartResponse = customer.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
    await customer.getByRole("button", { name: "Add to cart" }).click();
    expect((await cartResponse).ok()).toBeTruthy();
    await customer.goto("/cart");
    await expect(customer.locator("body")).toContainText(productName);
    await customer.getByRole("link", { name: /Proceed to checkout/ }).click();
    await customer.getByLabel("Recipient name").fill("Browser QA Customer");
    await customer.getByLabel("Phone number").fill("9876543210");
    await customer.getByLabel("Address line").fill("12 QA Test Street");
    await customer.getByLabel("City").fill("Bengaluru");
    await customer.getByLabel("State / region").fill("Karnataka");
    await customer.getByLabel("Postal code").fill("560034");
    await customer.getByRole("button", { name: /Continue to delivery/ }).click();
    await customer.getByRole("button", { name: /Continue to payment/ }).click();
    await customer.getByRole("button", { name: /Continue to review/ }).click();
    await customer.getByRole("button", { name: /Place order/ }).click();
    const capture = customer.getByRole("button", { name: "Simulate successful payment" });
    await expect(capture).toBeVisible();
    const orderNumber = await capture.getAttribute("data-order-number");
    expect(orderNumber).toBeTruthy();
    await capture.click();
    await expect(customer).toHaveURL(new RegExp(`/orders/${orderNumber}$`));
    await expect(customer.locator("body")).toContainText("Confirmed");

    const sellerOrdersPath = `/seller-admin/orders?q=${encodeURIComponent(orderNumber)}`;
    for (const status of ["processing", "shipped", "delivered"]) {
      await seller.goto(sellerOrdersPath);
      const sellerOrder = seller.locator(".seller-product-row").filter({ hasText: orderNumber });
      await expect(sellerOrder).toHaveCount(1);
      const fulfilmentForm = sellerOrder.locator('form[action="/seller-admin/orders/fulfill"]');
      await fulfilmentForm.locator('select[name="status"]').selectOption(status);
      await fulfilmentForm.locator('input[name="carrier"]').fill("Browser QA Carrier");
      await fulfilmentForm.locator('input[name="tracking_number"]').fill(`QA${suffix}`);
      const fulfillmentResponse = seller.waitForResponse(response => response.url().includes("/seller-admin/orders/fulfill") && response.request().method() === "POST");
      await fulfilmentForm.getByRole("button", { name: "Update fulfilment" }).click();
      expect((await fulfillmentResponse).status()).toBe(303);
      await seller.waitForURL(url => url.pathname === "/seller-admin/orders");
    }

    await customer.goto(`/orders/${orderNumber}`);
    await expect(customer.getByRole("heading", { name: "Request a return" })).toBeVisible();
    await customer.getByLabel("Reason for return").fill("The item arrived with a damaged corner during this local workflow test.");
    await customer.getByRole("button", { name: "Submit return request" }).click();
    await expect(customer.locator("body")).toContainText("Pending review");

    await signIn(operations, "demo.operations01@weevcrafts.local", "/operations-portal");
    await operations.goto(`/operations-portal/returns?q=${encodeURIComponent(orderNumber)}`);
    const returnRow = operations.locator(".portal-return-item").filter({ hasText: orderNumber });
    await expect(returnRow).toHaveCount(1);
    await returnRow.locator('select[name="status"]').selectOption("approved");
    await returnRow.getByRole("button", { name: "Save decision" }).click();
    await operations.goto(`/operations-portal/returns?q=${encodeURIComponent(orderNumber)}`);
    const approvedReturn = operations.locator(".portal-return-item").filter({ hasText: orderNumber });
    await expect(approvedReturn).toContainText("approved");
    await approvedReturn.getByRole("button", { name: "Mark item received" }).click();

    await customer.goto("/returns");
    await expect(customer.locator("body")).toContainText("Item received");
    await expect(customer.locator("body")).toContainText(orderNumber);
  } finally {
    await Promise.all([sellerContext.close(), marketplaceContext.close(), customerContext.close(), operationsContext.close()]);
  }
});
