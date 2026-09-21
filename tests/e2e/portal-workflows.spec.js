const { test, expect } = require("@playwright/test");
const path = require("node:path");

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

async function updateRoleUser(page, card, { email, displayName, status, password = "", reason }) {
  const form = card.locator("form.role-user-edit-form");
  if (email) await form.locator('input[name="email"]').fill(email);
  if (displayName) await form.locator('input[name="display_name"]').fill(displayName);
  if (status) await form.locator('select[name="status"]').selectOption(status);
  if (password) await form.locator('input[name="password"]').fill(password);
  await form.locator('input[name="reason"]').fill(reason);
  const responsePromise = page.waitForResponse(response =>
    response.url().includes("/super-admin/users/update") && response.request().method() === "POST"
  );
  await form.getByRole("button", { name: "Save account" }).click();
  expect((await responsePromise).status()).toBe(303);
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

test("unknown product URLs return not found", async ({ request }) => {
  const response = await request.get("/products/missing-qa-product-" + Date.now());
  expect(response.status()).toBe(404);
});

test("customer address create, edit and delete persist with field validation", async ({ browser, page }) => {
  test.skip(!localMutationTarget, "Customer address mutations are restricted to the local test deployment.");
  const suffix = Date.now().toString();
  const recipient = `QA Address ${suffix}`;
  await signIn(page, "demo.customer03@weevcrafts.local", "/account");
  await page.goto("/account/addresses");

  const createForm = page.locator("form[data-address-create]");
  const csrfToken = await createForm.locator('input[name="csrf_token"]').inputValue();
  const invalidAddress = await page.request.post("/account/addresses", { form: {
    csrf_token: csrfToken, recipient_name: recipient, phone: "9876543210", line1: "12 QA Road", line2: "",
    city: "Bengaluru", state: "Karnataka", postal_code: "12", country_code: "IN",
  } });
  expect(invalidAddress.status()).toBe(422);

  await createForm.locator('input[name="recipient_name"]').fill(recipient);
  await createForm.locator('input[name="phone"]').fill("+91 98765 43210");
  await createForm.locator('input[name="line1"]').fill("12 QA Road");
  await createForm.locator('input[name="line2"]').fill("Near QA Park");
  await createForm.locator('input[name="city"]').fill("Bengaluru");
  await createForm.locator('input[name="state"]').fill("Karnataka");
  await createForm.locator('input[name="postal_code"]').fill("560034");
  const createResponse = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/account/addresses");
  await createForm.getByRole("button", { name: "Add address" }).click();
  expect((await createResponse).status()).toBe(303);

  let addressCard = page.locator(".saved-address").filter({ hasText: recipient });
  await expect(addressCard).toHaveCount(1);
  let addressID = await addressCard.getAttribute("data-address-card");
  let editForm = addressCard.locator("form[data-address-edit]");
  await expect(editForm.locator('input[name="phone"]')).toHaveValue("+91 98765 43210");
  await expect(editForm.locator('input[name="country_code"]')).toHaveValue("IN");

  const otherContext = await browser.newContext();
  const otherCustomer = await otherContext.newPage();
  await signIn(otherCustomer, "demo.customer04@weevcrafts.local", "/account");
  await otherCustomer.goto("/account/addresses");
  const otherForm = otherCustomer.locator("form[data-address-create]");
  const otherCSRF = await otherForm.locator('input[name="csrf_token"]').inputValue();
  const foreignUpdate = await otherCustomer.request.post("/account/addresses", { form: {
    csrf_token: otherCSRF, address_id: addressID, recipient_name: recipient, phone: "+91 98765 43210",
    line1: "12 QA Road", line2: "Near QA Park", city: "Bengaluru", state: "Karnataka", postal_code: "560034", country_code: "IN",
  } });
  expect(foreignUpdate.status()).toBe(404);
  await expect(otherCustomer.locator(`.saved-address[data-address-card="${addressID}"]`)).toHaveCount(0);
  await otherContext.close();

  await editForm.locator('input[name="line2"]').fill("Opposite QA Library");
  const updateResponse = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/account/addresses");
  await editForm.getByRole("button", { name: "Save address" }).click();
  expect((await updateResponse).status()).toBe(303);
  addressCard = page.locator(`.saved-address[data-address-card="${addressID}"]`);
  editForm = addressCard.locator("form[data-address-edit]");
  await expect(editForm.locator('input[name="line2"]')).toHaveValue("Opposite QA Library");

  for (const width of [360, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const dimensions = await page.evaluate(() => ({ document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
    expect(dimensions.document, `address page document overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
    expect(dimensions.body, `address page body overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
  }

  const deleteResponse = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/account/addresses/delete");
  page.once("dialog", dialog => dialog.accept());
  await addressCard.locator("form[data-address-delete]").getByRole("button", { name: "Delete address" }).click();
  expect((await deleteResponse).status()).toBe(303);
  await expect(page.locator(`.saved-address[data-address-card="${addressID}"]`)).toHaveCount(0);
});

test("customer security and privacy controls reach their persisted workflows", async ({ page }) => {
  await signIn(page, "demo.customer03@weevcrafts.local", "/account");

  await page.goto("/account/security");
  await page.getByRole("link", { name: "Manage sessions" }).click();
  await expect(page).toHaveURL(/\/account\/sessions/);
  await expect(page.getByRole("heading", { name: "Your active sessions" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Active sessions" })).toBeVisible();

  await page.goto("/account/privacy");
  await page.getByRole("link", { name: "Manage privacy choices and data requests" }).click();
  await expect(page).toHaveURL(/\/privacy\/center/);
  await expect(page.getByRole("heading", { name: "Privacy center" })).toBeVisible();
  await expect(page.getByLabel("Request type")).toBeVisible();
  await expect(page.getByLabel("Details")).toBeVisible();

  if (localMutationTarget) {
    const marketingForm = page.locator('form[action="/privacy/consent"]').first();
    const state = await page.locator("body").innerText();
    const wasEnabled = /Email marketing is currently enabled/i.test(state);
    await marketingForm.getByRole("button").click();
    await expect(page.locator("body")).toContainText(wasEnabled ? "Email marketing is currently disabled" : "Email marketing is currently enabled");
    await page.locator('form[action="/privacy/consent"]').first().getByRole("button").click();
    await expect(page.locator("body")).toContainText(wasEnabled ? "Email marketing is currently enabled" : "Email marketing is currently disabled");

    const requestForm = page.locator('form[action="/privacy/requests"]');
    await requestForm.locator('select[name="request_type"]').selectOption("access");
    await requestForm.locator('textarea[name="details"]').fill(`Local E2E privacy access request ${Date.now()}`);
    await requestForm.getByRole("button", { name: "Submit request" }).click();
    await expect(page.locator("body")).toContainText(/access.*open/);
  }

  const unknownAccountPage = await page.goto("/account/not-a-real-setting");
  expect(unknownAccountPage.status()).toBe(404);
});

test("customer seller follows persist and seller catalog sorting is applied", async ({ browser }) => {
  test.skip(!localMutationTarget, "Seller follow mutations are restricted to the local test deployment.");
  const context = await browser.newContext();
  const page = await context.newPage();
  try {
    await signIn(page, "demo.customer08@weevcrafts.local", "/account");
    await page.goto("/makers/weavers-touch");
    await expect(page.getByRole("heading", { name: "Weaver's Touch", level: 1 })).toBeVisible();
    const followForm = page.locator("form[data-seller-follow]");
    if (await followForm.getByRole("button", { name: "Unfollow seller" }).count()) {
      const unfollowRequest = page.waitForRequest(request =>
        request.method() === "POST" && new URL(request.url()).pathname === "/ui/seller-follow/weavers-touch"
      );
      await followForm.getByRole("button", { name: "Unfollow seller" }).click();
      expect((await unfollowRequest).postData()).toContain("intent=unfollow");
      await page.waitForLoadState("networkidle");
      await page.reload();
      await expect(page.locator("form[data-seller-follow]").getByRole("button", { name: "Follow seller" })).toBeVisible();
    }
    const followRequest = page.waitForRequest(request =>
      request.method() === "POST" && new URL(request.url()).pathname === "/ui/seller-follow/weavers-touch"
    );
    await followForm.getByRole("button", { name: "Follow seller" }).click();
    expect((await followRequest).postData()).toContain("intent=follow");
    await page.waitForLoadState("networkidle");
    await page.reload();
    await expect(page.locator("form[data-seller-follow]").getByRole("button", { name: "Unfollow seller" })).toBeVisible();

    await page.getByLabel("Sort by").selectOption("price-asc");
    await page.getByRole("button", { name: "Apply" }).click();
    await expect(page).toHaveURL(/sort=price-asc/);
    const prices = await page.locator('[data-backend-products="listing"] .price strong').allTextContents();
    const numericPrices = prices.map(value => Number(value.replace(/[^0-9]/g, "")));
    expect(numericPrices).toEqual([...numericPrices].sort((left, right) => left - right));

    for (const width of [360, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      const dimensions = await page.evaluate(() => ({ document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
      expect(dimensions.document, `seller storefront overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
      expect(dimensions.body, `seller storefront body overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
    }

    await page.locator("form[data-seller-follow]").getByRole("button", { name: "Unfollow seller" }).click();
    await expect(page.locator("form[data-seller-follow]").getByRole("button", { name: "Follow seller" })).toBeVisible();
  } finally {
    await context.close();
  }
});

test("seller owner profile edits persist while seller staff remain blocked", async ({ browser }) => {
  test.skip(!localMutationTarget, "Seller profile mutations are restricted to the local test deployment.");
  const suffix = Date.now().toString();
  const ownerContext = await browser.newContext();
  const staffContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  const staff = await staffContext.newPage();
  let originalName = "";
  let restored = false;

  const saveProfile = async (name, reason) => {
    await owner.goto("/seller-admin/settings");
    const form = owner.locator('form[action="/seller-admin/settings"]');
    await form.locator('input[name="display_name"]').fill(name);
    await form.locator('input[name="reason"]').fill(reason);
    const response = owner.waitForResponse(item => item.request().method() === "POST" && new URL(item.url()).pathname === "/seller-admin/settings");
    await form.getByRole("button", { name: "Save store profile" }).click();
    expect((await response).status()).toBe(303);
  };

  try {
    await signIn(owner, "demo.owner01@weevcrafts.local", "/seller-admin");
    await owner.goto("/seller-admin/settings");
    const editForm = owner.locator('form[action="/seller-admin/settings"]');
    originalName = await editForm.locator('input[name="display_name"]').inputValue();
    const updatedName = `${originalName} QA ${suffix}`;
    await saveProfile(updatedName, "Local end-to-end seller profile update verification");
    await owner.goto("/seller-admin/settings");
    await expect(owner.locator('form[action="/seller-admin/settings"] input[name="display_name"]')).toHaveValue(updatedName);

    await signIn(staff, "demo.staff01@weevcrafts.local", "/seller-admin");
    const staffSettings = await staff.request.get("/seller-admin/settings", { maxRedirects: 0 });
    expect(staffSettings.status()).toBe(403);
    const staffMutation = await staff.request.post("/seller-admin/settings", { form: { display_name: "Unauthorized QA Edit", reason: "Should be rejected" } });
    expect(staffMutation.status()).toBe(403);

    await saveProfile(originalName, "Restore seller profile after local verification");
    restored = true;
  } finally {
    if (originalName && !restored) {
      try {
        await saveProfile(originalName, "Restore seller profile after interrupted local verification");
      } catch {}
    }
    await Promise.all([ownerContext.close(), staffContext.close()]);
  }
});

test("super admin logout and user lifecycle work end to end", async ({ browser }) => {
  test.skip(!localMutationTarget, "User management mutations are restricted to the local test deployment.");
  test.setTimeout(180_000);

  const suffix = Date.now().toString();
  const email = `qa.role.${suffix}@example.test`;
  const updatedEmail = `qa.updated.${suffix}@example.test`;
  const passwordForNewUser = "QA-Role-Setup-2026!";
  const passwordAfterReset = "QA-Role-Updated-2026!";
  const adminContext = await browser.newContext();
  const userContext = await browser.newContext();
  const ownerContext = await browser.newContext();
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
    await updateRoleUser(admin, userCard, {
      email: updatedEmail,
      displayName: `QA Finance Updated ${suffix}`,
      status: "active",
      password: passwordAfterReset,
      reason: "Local account edit, email verification and password reset verification",
    });
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await expect(userCard).toContainText(`QA Finance Updated ${suffix}`);
    await user.goto("/finance-portal");
    await expect(user).toHaveURL(/\/login/);
    await signIn(user, updatedEmail, "/finance-portal", passwordAfterReset);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await updateRoleUser(admin, userCard, { status: "suspended", reason: "Local session revocation verification" });
    await expect(admin.locator(".role-assignment-card").filter({ hasText: updatedEmail })).toContainText("suspended");
    await user.goto("/finance-portal");
    await expect(user).toHaveURL(/\/login/);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await updateRoleUser(admin, userCard, { status: "active", reason: "Reactivate local account for role checks" });
    await user.goto("/finance-portal");
    await expect(user).toHaveURL(/\/login/);
    await signIn(user, updatedEmail, "/finance-portal", passwordAfterReset);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    const roleForm = userCard.locator("form.role-assignment-form");
    await roleForm.locator('select[name="role"]').selectOption("seller_staff");
    const sellerSelect = roleForm.locator('select[name="seller_id"]');
    await expect(sellerSelect).toBeVisible();
    await expect.poll(() => sellerSelect.locator("option").count()).toBeGreaterThan(1);
    const ownerSellerOption = sellerSelect.locator("option").filter({ hasText: "Weaver's Touch" });
    await expect(ownerSellerOption).toHaveCount(1);
    const sellerID = await ownerSellerOption.getAttribute("value");
    await submitRoleChange(userCard, "seller_staff", "grant", "Local seller staff role assignment verification", sellerID);

    await signIn(user, updatedEmail, "/workspaces", passwordAfterReset);
    const availableWorkspaces = user.getByRole("navigation", { name: "Available workspaces" });
    await expect(availableWorkspaces.getByRole("link")).toHaveCount(2);
    await expect(availableWorkspaces).toContainText("Finance");
    await expect(availableWorkspaces).toContainText("Seller staff");
    await availableWorkspaces.getByRole("link", { name: /Seller staff/ }).click();
    await expect(user).toHaveURL(/\/seller-admin\/?$/);
    await expect(user.getByRole("navigation", { name: "Seller portal navigation" })).not.toContainText("Team");
    const deniedCreate = await userContext.request.get("/seller-admin/products/new", { maxRedirects: 0 });
    expect(deniedCreate.status()).toBe(403);
    const deniedInventory = await userContext.request.get("/seller-admin/inventory", { maxRedirects: 0 });
    expect(deniedInventory.status()).toBe(403);
    const allowedOrders = await userContext.request.get("/seller-admin/orders", { maxRedirects: 0 });
    expect(allowedOrders.status()).toBe(200);
    const deniedFulfillment = await userContext.request.post("/seller-admin/orders/fulfill", { maxRedirects: 0 });
    expect(deniedFulfillment.status()).toBe(403);

    const owner = await ownerContext.newPage();
    await signIn(owner, "demo.owner01@weevcrafts.local", "/seller-admin");
    await owner.goto("/seller-admin/team");
    const permissionForm = owner.locator('form[action="/seller-admin/team/update"]').filter({ hasText: `Update permissions for QA Finance Updated ${suffix}` });
    await expect(permissionForm).toHaveCount(1);
    await permissionForm.locator('input[name="permissions"][value="PRODUCT_WRITE"]').check();
    await permissionForm.getByRole("button", { name: "Save permissions" }).click();
    await expect(owner).toHaveURL(/\/seller-admin\/team\?notice=Staff\+permissions\+updated/);
    await owner.goto("/seller-admin/team");
    await expect(owner.locator('form[action="/seller-admin/team/update"]').filter({ hasText: `Update permissions for QA Finance Updated ${suffix}` })).toContainText("Edit products");
    await user.goto("/seller-admin/products/new");
    await expect(user.locator("form.seller-form-grid")).toBeVisible();

    const addStaffForm = owner.locator('form[action="/seller-admin/team/add"]');
    await addStaffForm.locator('input[name="email"]').fill(updatedEmail);
    await addStaffForm.locator('input[name="permissions"][value="PRODUCT_READ"]').check();
    await addStaffForm.locator('input[name="permissions"][value="PRODUCT_WRITE"]').check();
    await addStaffForm.getByRole("button", { name: "Add seller staff" }).click();
    await expect(owner).toHaveURL(/\/seller-admin\/team\?notice=Staff\+access\+added/);
    await owner.goto("/seller-admin/team");
    const staffRow = owner.locator(".seller-product-row").filter({ hasText: updatedEmail });
    await expect(staffRow).toContainText("PRODUCT_WRITE");
    const removeStaffResponse = owner.waitForResponse(response => response.url().includes("/seller-admin/team/remove") && response.request().method() === "POST");
    await staffRow.locator('form[action="/seller-admin/team/remove"] button').click();
    expect((await removeStaffResponse).status()).toBe(303);
    await expect(owner).toHaveURL(/\/seller-admin\/team\?notice=Staff\+access\+removed/);
    await owner.goto("/seller-admin/team");
    await expect(owner.locator(".seller-product-row").filter({ hasText: updatedEmail })).toHaveCount(0);
    const staffDenied = await userContext.request.get("/seller-admin", { maxRedirects: 0 });
    expect(staffDenied.status()).toBe(403);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await submitRoleChange(userCard, "seller_staff", "grant", "Restore staff workspace after seller removal verification", sellerID);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await submitRoleChange(userCard, "seller_staff", "revoke", "Local seller staff role revocation verification", sellerID);

    await admin.goto("/super-admin/security");
    userCard = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
    await deleteRoleUser(admin, userCard, "Local end-to-end role lifecycle cleanup");
    accountDeleted = true;
    await expect(admin.locator(".role-assignment-card").filter({ hasText: updatedEmail })).toHaveCount(0);
  } finally {
    if (accountCreated && !accountDeleted) {
      await admin.goto("/super-admin/security");
      const card = admin.locator(".role-assignment-card").filter({ hasText: updatedEmail });
      if (await card.count()) {
        await deleteRoleUser(admin, card, "Cleanup failed local browser run");
      }
    }
    await Promise.all([adminContext.close(), userContext.close(), ownerContext.close()]);
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
    await expect(productRow.locator(".status")).toHaveText("draft");
    await productRow.getByRole("link", { name: "Edit" }).click();
    const editForm = admin.locator("form.admin-product-editor-form").first();
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
    productRow = admin.locator(".table-row").filter({ hasText: productName });
    await expect(productRow.locator(".status")).toHaveText("archived");
    const restoreForm = productRow.locator('form[action="/super-admin/products/restore"]');
    await restoreForm.locator('input[name="reason"]').fill("Local product restore workflow test");
    admin.once("dialog", dialog => dialog.accept());
    await restoreForm.getByRole("button", { name: "Restore draft" }).click();
    productArchived = false;
    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = admin.locator(".table-row").filter({ hasText: productName });
    await expect(productRow.locator(".status")).toHaveText("draft");
    const archiveForm = productRow.locator('form[action="/super-admin/products/archive"]');
    await archiveForm.locator('input[name="reason"]').fill("Local E2E cleanup after restore");
    admin.once("dialog", dialog => dialog.accept());
    await archiveForm.getByRole("button", { name: "Archive" }).click();
    productArchived = true;
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
    const editForm = seller.locator("form.seller-form-grid").first();
    await editForm.locator('input[name="price_cents"]').fill("259900");
    await editForm.locator('textarea[name="description"]').fill("Updated local browser test product description.");
    await editForm.getByRole("button", { name: "Save draft" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products/);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const updatedSellerProduct = seller.locator(".seller-product-row").filter({ hasText: productName });
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("link", { name: "Edit product" }).click();
    const variantForm = seller.locator('form[action="/seller-admin/products/variants/create"]');
    await variantForm.locator('input[name="name"]').fill("Indigo edition");
    await variantForm.locator('input[name="sku"]').fill(`QA-ALT-${suffix}`);
    await variantForm.locator('input[name="price_cents"]').fill("279900");
    await variantForm.locator('input[name="compare_at_cents"]').fill("299900");
    await variantForm.locator('input[name="initial_stock"]').fill("5");
    await variantForm.getByRole("button", { name: "Add variant" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products\/\d+\/edit/);
    const productEditURL = seller.url();
    const addedVariantForm = seller.locator("form.product-variant-edit").nth(1);
    const addedVariantID = await addedVariantForm.locator('input[name="variant_id"]').inputValue();
    await addedVariantForm.locator('input[name="name"]').fill("Indigo edition updated");
    await addedVariantForm.locator('input[name="sku"]').fill(`QA-ALT-EDIT-${suffix}`);
    await addedVariantForm.locator('input[name="price_cents"]').fill("284900");
    await addedVariantForm.locator('input[name="compare_at_cents"]').fill("309900");
    await addedVariantForm.getByRole("button", { name: "Save variant" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products\/\d+\/edit/);

    await seller.goto("/seller-admin/inventory");
    const inventoryRow = seller.locator(".seller-product-row").filter({ hasText: `QA-ALT-EDIT-${suffix}` });
    await expect(inventoryRow).toHaveCount(1);
    await expect(inventoryRow.locator("span").nth(3)).toHaveText("5");
    const inventoryForm = inventoryRow.locator('form[action="/seller-admin/inventory/adjust"]');
    await inventoryForm.locator('input[name="delta"]').fill("2");
    await inventoryForm.locator('input[name="reason"]').fill("Local E2E inventory adjustment persistence check");
    const inventoryResponse = seller.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/seller-admin/inventory/adjust");
    await inventoryForm.getByRole("button", { name: "Save" }).click();
    expect((await inventoryResponse).status()).toBe(303);
    await expect(seller).toHaveURL(/\/seller-admin\/inventory/);
    await expect(seller.locator(".seller-product-row").filter({ hasText: `QA-ALT-EDIT-${suffix}` }).locator("span").nth(3)).toHaveText("7");

    await seller.goto(productEditURL);
    const mediaPanel = seller.locator("[data-product-media]");
    await mediaPanel.locator("[data-media-file]").setInputFiles(path.resolve(__dirname, "../../web/assets/images/customer/maker-mithila.webp"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product image saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset img")).toHaveCount(1);
    await mediaPanel.locator("[data-media-file]").setInputFiles(path.resolve(__dirname, "fixtures/flower.mp4"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product video saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset video source[type='video/mp4']")).toHaveCount(1);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("button", { name: "Submit for review" }).click();

    await signIn(marketplace, "demo.marketplace01@weevcrafts.local", "/marketplace-admin");
    await marketplace.goto(`/marketplace-admin/products?q=${encodeURIComponent(sku)}`);
    const reviewRow = marketplace.locator(".table-row").filter({ hasText: productName });
    await expect(reviewRow).toHaveCount(1);
    await reviewRow.getByRole("button", { name: "Approve" }).click();

    await signIn(customer, "demo.customer08@weevcrafts.local", "/account");
    await customer.goto("/account/addresses");
    const shippingRecipient = `Browser QA Customer ${suffix}`;
    const savedAddressForm = customer.locator("form[data-address-create]");
    await savedAddressForm.locator('input[name="recipient_name"]').fill(shippingRecipient);
    await savedAddressForm.locator('input[name="phone"]').fill("9876543210");
    await savedAddressForm.locator('input[name="line1"]').fill("12 QA Test Street");
    await savedAddressForm.locator('input[name="line2"]').fill(`QA-${suffix}`);
    await savedAddressForm.locator('input[name="city"]').fill("Bengaluru");
    await savedAddressForm.locator('input[name="state"]').fill("Karnataka");
    await savedAddressForm.locator('input[name="postal_code"]').fill("560034");
    const savedAddressResponse = customer.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/account/addresses");
    await savedAddressForm.getByRole("button", { name: "Add address" }).click();
    expect((await savedAddressResponse).status()).toBe(303);
    await customer.goto("/cart");
    const removeCartItems = customer.getByRole("button", { name: "Remove", exact: true });
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const itemCount = await removeCartItems.count();
      if (itemCount === 0) break;
      await removeCartItems.first().click();
      await expect(removeCartItems).toHaveCount(itemCount - 1);
    }
    await customer.goto(`/products/${slug}`);
    await expect(customer.getByRole("heading", { name: productName })).toBeVisible();
    await expect(customer.locator("#product-panel-description")).toContainText("Updated local browser test product description.");
    await expect(customer.locator("#product-panel-details")).toContainText("Delivery to serviceable PIN codes in India");
    const productImage = customer.locator('.gallery-main img[src^="/media/products/"]').first();
    await expect(productImage).toBeVisible();
    const uploadedImageURL = await productImage.getAttribute("src");
    const imageResponse = await customer.request.get(uploadedImageURL);
    expect(imageResponse.status()).toBe(200);
    expect(imageResponse.headers()["content-type"]).toContain("image/webp");
    const videoButton = customer.getByRole("button", { name: /Show product media/ });
    await expect(videoButton).toBeVisible();
    await videoButton.click();
    const productVideo = customer.locator(".gallery-main video");
    await expect(productVideo).toBeVisible();
    const videoSource = await productVideo.locator("source").getAttribute("src");
    const videoResponse = await customer.request.get(videoSource);
    expect(videoResponse.status()).toBe(200);
    expect(videoResponse.headers()["content-type"]).toContain("video/mp4");
    await expect(customer.locator("#detail-variant option")).toHaveCount(2);
    const variantValue = await customer.locator("#detail-variant option").filter({ hasText: "Indigo edition updated" }).getAttribute("value");
    expect(variantValue).toBeTruthy();
    await customer.locator("#detail-variant").selectOption(variantValue);
    await expect(customer.locator("[data-selected-variant-price]")).toHaveText("INR 2849");
    await expect(customer.locator("[data-selected-variant-sku]")).toHaveText(`QA-ALT-EDIT-${suffix}`);
    const cartResponse = customer.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
    await customer.getByRole("button", { name: "Add to cart" }).click();
    expect((await cartResponse).ok()).toBeTruthy();
    await customer.goto("/cart");
    await expect(customer.locator("body")).toContainText(productName);
    await expect(customer.locator("body")).toContainText("Indigo edition");
    await customer.getByRole("link", { name: /Proceed to checkout/ }).click();
    await expect(customer.getByLabel("Recipient name")).toHaveValue(shippingRecipient);
    await expect(customer.getByLabel("Phone number")).toHaveValue("9876543210");
    await expect(customer.getByLabel("Address line")).toHaveValue("12 QA Test Street");
    await expect(customer.getByLabel("Apartment / landmark (optional)")).toHaveValue(`QA-${suffix}`);
    await expect(customer.getByLabel("City")).toHaveValue("Bengaluru");
    await expect(customer.getByLabel("State / region")).toHaveValue("Karnataka");
    await expect(customer.getByLabel("Postal code")).toHaveValue("560034");
    const country = customer.locator('#checkout-address-form input[name="country_code"]');
    await expect(country).toHaveValue("IN");
    await expect(customer.getByLabel("Delivery country")).toBeDisabled();
    await customer.getByRole("button", { name: /Continue to delivery/ }).click();
    await customer.getByRole("button", { name: /Continue to payment/ }).click();
    await customer.getByRole("button", { name: /Continue to review/ }).click();
    const checkoutRequest = customer.waitForRequest(request => request.url().includes("/api/v1/checkout") && request.method() === "POST");
    await customer.getByRole("button", { name: /Place order/ }).click();
    const checkoutPayload = JSON.parse((await checkoutRequest).postData());
    expect(checkoutPayload.address.phone).toBe("9876543210");
    expect(checkoutPayload.address.country_code).toBe("IN");
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

    await customer.goto("/account/addresses");
    const savedShippingAddress = customer.locator(".saved-address").filter({ hasText: shippingRecipient });
    await expect(savedShippingAddress).toHaveCount(1);
    await expect(savedShippingAddress.locator('input[name="phone"]')).toHaveValue("9876543210");
    customer.once("dialog", dialog => dialog.accept());
    await savedShippingAddress.locator("form[data-address-delete]").getByRole("button", { name: "Delete address" }).click();
    await expect(customer.locator(".saved-address").filter({ hasText: shippingRecipient })).toHaveCount(0);

    await seller.goto(productEditURL);
    const uploadedVideo = seller.locator(".product-media-asset").filter({ has: seller.locator("video source[type='video/mp4']") });
    await expect(uploadedVideo).toHaveCount(1);
    const deleteVideo = uploadedVideo.locator("[data-media-delete]");
    const uploadedVideoURL = await uploadedVideo.locator("video source[type='video/mp4']").getAttribute("src");
    seller.once("dialog", dialog => dialog.accept());
    await deleteVideo.click();
    await expect(seller.locator(".product-media-asset video source[type='video/mp4']")).toHaveCount(0);
    expect((await customer.request.get(uploadedVideoURL)).status()).toBe(404);

    const uploadedImage = seller.locator(".product-media-asset").filter({ has: seller.locator("img") });
    await expect(uploadedImage).toHaveCount(1);
    seller.once("dialog", dialog => dialog.accept());
    await uploadedImage.locator("[data-media-delete]").click();
    await expect(seller.locator(".product-media-asset img")).toHaveCount(0);
    await customer.goto(`/products/${slug}`);
    await expect(customer.locator('.gallery-main img[src^="/media/products/"]')).toHaveCount(0);
    expect((await customer.request.get(uploadedImageURL)).status()).toBe(404);

    const archiveVariant = seller.locator(`form[action="/seller-admin/products/variants/archive"]:has(input[name="variant_id"][value="${addedVariantID}"])`);
    await archiveVariant.locator('input[name="reason"]').fill("Local variant archive verification after checkout");
    seller.once("dialog", dialog => dialog.accept());
    await archiveVariant.getByRole("button", { name: "Archive variant" }).click();
    await seller.goto(productEditURL);
    await expect(seller.locator(".variant-stock-summary").filter({ hasText: "archived" })).toHaveCount(1);
  } finally {
    await Promise.all([sellerContext.close(), marketplaceContext.close(), customerContext.close(), operationsContext.close()]);
  }
});

test("customer and support agent share a persisted ticket conversation", async ({ browser }) => {
  test.skip(!localMutationTarget, "Support workflow mutations are restricted to the local test deployment.");

  const suffix = Date.now().toString();
  const subject = `QA support request ${suffix}`;
  const customerContext = await browser.newContext();
  const supportContext = await browser.newContext();
  const customer = await customerContext.newPage();
  const support = await supportContext.newPage();

  try {
    await signIn(customer, "demo.customer08@weevcrafts.local", "/account");
    await customer.goto("/contact");
    const createForm = customer.locator('form[data-customer-support-create]');
    await expect(createForm).toBeVisible();
    await createForm.locator('input[name="subject"]').fill(subject);
    await createForm.locator('input[name="order_number"]').fill("");
    await createForm.locator('textarea[name="message"]').fill("Please help me check a question about this handmade order.");
    await createForm.getByRole("button", { name: "Send support request" }).click();
    await expect(customer).toHaveURL(/\/account\/support\?/);
    const ticket = customer.locator("article.customer-support-ticket").filter({ hasText: subject });
    await expect(ticket).toBeVisible();
    const ticketID = (await ticket.getAttribute("id")).replace("ticket-", "");
    await expect(ticket).toContainText("Please help me check a question about this handmade order.");

    await signIn(support, "demo.support01@weevcrafts.local", "/support-portal");
    await support.goto(`/support-portal/cases?ticket_id=${ticketID}`);
    await expect(support.locator("body")).toContainText(subject);
    await expect(support.locator("body")).toContainText("Please help me check a question about this handmade order.");
    const agentReply = `We have received your request ${suffix} and will follow up here.`;
    const replyForm = support.locator('form[action="/support-portal/cases/reply"]');
    await replyForm.locator('textarea[name="message"]').fill(agentReply);
    await replyForm.getByRole("button", { name: "Send Reply" }).click();
    await expect(support).toHaveURL(new RegExp(`/support-portal/cases\\?ticket_id=${ticketID}`));

    await customer.goto(`/account/support?ticket_id=${ticketID}`);
    const customerTicket = customer.locator(`#ticket-${ticketID}`);
    await expect(customerTicket).toContainText(agentReply);
    const customerReply = `Additional details for support request ${suffix}.`;
    const customerReplyForm = customerTicket.locator('form[action="/support/customer-reply"]');
    await customerReplyForm.locator('textarea[name="message"]').fill(customerReply);
    await customerReplyForm.getByRole("button", { name: "Send reply" }).click();
    await expect(customer).toHaveURL(new RegExp(`/account/support\\?ticket_id=${ticketID}`));

    await support.goto(`/support-portal/cases?ticket_id=${ticketID}`);
    await expect(support.locator("body")).toContainText(customerReply);
    const updateForm = support.locator('.live-support-row').filter({ hasText: subject }).locator('form[action="/support-portal/cases/status"]');
    await updateForm.locator('select[name="status"]').selectOption("resolved");
    await updateForm.locator('input[name="note"]').fill("Resolved after customer follow-up.");
    await updateForm.getByRole("button", { name: "Save" }).click();
    await expect(support).toHaveURL(new RegExp(`/support-portal/cases\\?ticket_id=${ticketID}`));
    const closeForm = support.locator('.live-support-row').filter({ hasText: subject }).locator('form[action="/support-portal/cases/status"]');
    await closeForm.locator('select[name="status"]').selectOption("closed");
    await closeForm.locator('input[name="note"]').fill("Resolved after customer follow-up.");
    await closeForm.getByRole("button", { name: "Save" }).click();
    await expect(support).toHaveURL(new RegExp(`/support-portal/cases\\?ticket_id=${ticketID}`));
    await customer.goto(`/account/support?ticket_id=${ticketID}`);
    await expect(customer.locator(`#ticket-${ticketID}`)).toContainText("closed");
    await expect(customer.locator(`#ticket-${ticketID} form[action="/support/customer-reply"]`)).toHaveCount(0);
  } finally {
    await Promise.all([customerContext.close(), supportContext.close()]);
  }
});
