const { test, expect } = require("@playwright/test");

const password = "WeeVCrafts-Demo-2026!";
const target = new URL(process.env.WEEVCRAFTS_BASE_URL || "http://127.0.0.1:8080");
const localMutationTarget = ["localhost", "127.0.0.1", "::1"].includes(target.hostname);

async function signIn(page, email, landing, passwordValue = password) {
  await page.goto("/login");
  await page.locator("#page input[name=\"email\"]").fill(email);
  await page.locator("#page input[name=\"password\"]").fill(passwordValue);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, "") === landing.replace(/\/$/, ""), { timeout: 30_000 });
}

async function submitRoleChange(card, role, action, reason, sellerID = "") {
  const form = card.locator("form.role-assignment-form");
  await form.locator('select[name="role"]').selectOption(role);
  if (role === "seller_staff") {
    const seller = form.locator('select[name="seller_id"]');
    await expect(seller).toBeVisible();
    if (sellerID) {
      await seller.selectOption(sellerID);
    } else {
      await seller.selectOption({ index: 1 });
    }
  }
  await form.locator('select[name="action"]').selectOption(action);
  await form.locator('input[name="reason"]').fill(reason);
  await form.getByRole("button", { name: "Save role change" }).click();
  if (action === "grant") {
    await expect(card.locator(".role-badges")).toContainText(role);
  } else {
    await expect(card.locator(".role-badges")).not.toContainText(role);
  }
}

async function deleteRoleUser(page, card, reason) {
  const form = card.locator("form.role-delete-form");
  await form.locator('input[name="reason"]').fill(reason);
  const responsePromise = page.waitForResponse(response =>
    response.url().includes("/super-admin/users/delete") && response.request().method() === "POST"
  );
  page.once("dialog", dialog => dialog.accept());
  await form.getByRole("button", { name: "Delete account" }).click();
  const response = await responsePromise;
  expect(response.status()).toBe(303);
  await page.goto("/super-admin/security");
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

test("super admin logout and user lifecycle work end to end", async ({ browser }) => {
  test.skip(!localMutationTarget, "User management mutations are restricted to the local test deployment.");
  test.setTimeout(180_000);

  const suffix = Date.now().toString();
  const email = `qa.role.${suffix}@example.test`;
  const passwordForNewUser = "QA-Role-Setup-2026!";
  const adminContext = await browser.newContext();
  const userContext = await browser.newContext();
  const admin = await adminContext.newPage();
  let accountCreated = false;
  let accountDeleted = false;

  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/security");
    await expect(admin.getByRole("button", { name: "Log out" })).toBeVisible();
    await admin.getByRole("button", { name: "Log out" }).click();
    await expect(admin).toHaveURL("http://127.0.0.1:8080/");
    await admin.goto("/super-admin/security");
    await expect(admin).toHaveURL(/\/login\?/);
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");

    await admin.goto("/super-admin/security");
    const oldQACards = admin.locator(".role-assignment-card").filter({ hasText: /qa\.role\.\d+@example\.test/ });
    while (await oldQACards.count()) {
      await deleteRoleUser(admin, oldQACards.first(), "Remove account left by an interrupted local browser run");
    }
    const createForm = admin.locator("form.role-user-create-form");
    await createForm.locator('input[name="display_name"]').fill(`QA Finance ${suffix}`);
    await createForm.locator('input[name="email"]').fill(email);
    await createForm.locator('input[name="password"]').fill(passwordForNewUser);
    await createForm.locator('select[name="role"]').selectOption("finance_operator");
    await createForm.locator('input[name="reason"]').fill("Local end-to-end role lifecycle verification");
    await createForm.getByRole("button", { name: "Create account" }).click();
    accountCreated = true;
    await expect(admin).toHaveURL(/\/super-admin\/security/);

    let userCard = admin.locator(".role-assignment-card").filter({ hasText: email });
    await expect(userCard).toContainText("finance_operator");

    const user = await userContext.newPage();
    await signIn(user, email, "/finance-portal", passwordForNewUser);
    await expect(user.locator("body")).toContainText(`QA Finance ${suffix}`);
    await expect(user.locator("body")).toContainText("Finance Operator");

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: email });
    const roleForm = userCard.locator("form.role-assignment-form");
    await roleForm.locator('select[name="role"]').selectOption("seller_staff");
    const sellerSelect = roleForm.locator('select[name="seller_id"]');
    await expect(sellerSelect).toBeVisible();
    await expect.poll(() => sellerSelect.locator("option").count()).toBeGreaterThan(1);
    const sellerID = await sellerSelect.locator("option").nth(1).getAttribute("value");
    await submitRoleChange(userCard, "seller_staff", "grant", "Local seller staff role assignment verification", sellerID);

    await user.goto("/workspaces");
    const availableWorkspaces = user.getByRole("navigation", { name: "Available workspaces" });
    await expect(availableWorkspaces.getByRole("link")).toHaveCount(2);
    await expect(availableWorkspaces).toContainText("Finance");
    await expect(availableWorkspaces).toContainText("Seller staff");
    await availableWorkspaces.getByRole("link", { name: /Seller staff/ }).click();
    await expect(user).toHaveURL(/\/seller-admin\/?$/);
    await expect(user.getByRole("navigation", { name: "Seller portal navigation" })).not.toContainText("Team");
    const deniedCreate = await userContext.request.get("/seller-admin/products/new", { maxRedirects: 0 });
    expect(deniedCreate.status()).toBe(403);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: email });
    await submitRoleChange(userCard, "seller_staff", "revoke", "Local seller staff role revocation verification", sellerID);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: email });
    await deleteRoleUser(admin, userCard, "Local end-to-end role lifecycle cleanup");
    accountDeleted = true;
    await expect(admin.locator(".role-assignment-card").filter({ hasText: email })).toHaveCount(0);
  } finally {
    if (accountCreated && !accountDeleted) {
      await admin.goto("/super-admin/security");
      const card = admin.locator(".role-assignment-card").filter({ hasText: email });
      if (await card.count()) {
        await deleteRoleUser(admin, card, "Cleanup failed local browser run");
      }
    }
    await Promise.all([adminContext.close(), userContext.close()]);
  }
});

test("super admin can create, edit and archive product listings", async ({ browser }) => {
  test.skip(!localMutationTarget, "Product catalogue mutations are restricted to the local test deployment.");
  test.setTimeout(180_000);

  const suffix = Date.now().toString();
  const productName = `QA Admin Craft ${suffix}`;
  const slug = `qa-admin-craft-${suffix}`;
  const sku = `QA-ADMIN-${suffix}`;
  const context = await browser.newContext();
  const admin = await context.newPage();
  let productCreated = false;
  let productArchived = false;

  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/products/new");
    const form = admin.locator("form.admin-product-editor-form");
    const seller = form.locator('select[name="seller_id"]');
    await expect.poll(() => seller.locator("option").count()).toBeGreaterThan(1);
    await seller.selectOption({ index: 1 });
    await form.locator('input[name="name"]').fill(productName);
    await form.locator('input[name="slug"]').fill(slug);
    await form.locator('input[name="brand_name"]').fill(`QA Maker ${suffix}`);
    await form.locator('input[name="brand_slug"]').fill(`qa-maker-${suffix}`);
    await form.locator('input[name="category_name"]').fill("QA Crafts");
    await form.locator('input[name="category_slug"]').fill(`qa-crafts-${suffix}`);
    await form.locator('input[name="sku"]').fill(sku);
    await form.locator('input[name="price_cents"]').fill("249900");
    await form.locator('input[name="compare_at_cents"]').fill("299900");
    await form.locator('input[name="initial_stock"]').fill("3");
    await form.locator('input[name="delivery_label"]').fill("Delivery to serviceable PIN codes in India");
    await form.locator('textarea[name="description"]').fill("Local end-to-end test listing created by Super Admin.");
    await form.getByRole("button", { name: "Save product" }).click();
    productCreated = true;

    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    let productRow = admin.locator(".table-row").filter({ hasText: productName });
    await expect(productRow).toContainText("draft");
    await productRow.getByRole("link", { name: "Edit" }).click();
    const editForm = admin.locator("form.admin-product-editor-form");
    await editForm.locator('input[name="price_cents"]').fill("259900");
    await editForm.locator('textarea[name="description"]').fill("Updated description, price and catalogue details.");
    await editForm.getByRole("button", { name: "Save product" }).click();

    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = admin.locator(".table-row").filter({ hasText: productName });
    await expect(productRow).toContainText("INR 2599.00");
    await productRow.locator('form[action="/super-admin/products/archive"] input[name="reason"]').fill("Local product deletion workflow test");
    admin.once("dialog", dialog => dialog.accept());
    await productRow.getByRole("button", { name: "Archive" }).click();
    productArchived = true;
    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    await expect(admin.locator(".table-row").filter({ hasText: productName })).toContainText("archived");
  } finally {
    if (productCreated && !productArchived) {
      await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
      const row = admin.locator(".table-row").filter({ hasText: productName });
      if (await row.count()) {
        const reason = row.locator('form[action="/super-admin/products/archive"] input[name="reason"]');
        if (await reason.count()) {
          await reason.fill("Local E2E cleanup");
          admin.once("dialog", dialog => dialog.accept());
          await row.getByRole("button", { name: "Archive" }).click();
        }
      }
    }
    await context.close();
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
    await seller.getByRole("spinbutton", { name: "Price in paise", exact: true }).fill("249900");
    await seller.getByLabel("Initial stock").fill("5");
    await seller.getByLabel("Delivery note").fill("Delivery to serviceable PIN codes in India");
    await seller.getByLabel("Description").fill("A locally created browser test listing used to verify the WeeVCrafts cross-role order flow.");
    await seller.getByRole("button", { name: "Save draft" }).click();

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const sellerProduct = seller.locator(".seller-product-row").filter({ hasText: productName });
    await expect(sellerProduct).toHaveCount(1);
    await sellerProduct.getByRole("link", { name: "Edit product" }).click();
    const editForm = seller.locator("form.seller-form-grid");
    await editForm.locator('input[name="price_cents"]').fill("259900");
    await editForm.locator('textarea[name="description"]').fill("Updated local browser test product description.");
    await editForm.getByRole("button", { name: "Save draft" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products/);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const updatedSellerProduct = seller.locator(".seller-product-row").filter({ hasText: productName });
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("button", { name: "Submit for review" }).click();

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
