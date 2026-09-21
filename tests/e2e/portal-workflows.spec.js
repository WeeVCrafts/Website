const { test, expect } = require("@playwright/test");
const path = require("node:path");

const password = "WeeVCrafts-Demo-2026!";
const target = new URL(process.env.WEEVCRAFTS_BASE_URL || "http://127.0.0.1:8080");
const localMutationTarget = ["localhost", "127.0.0.1", "::1"].includes(target.hostname);

async function canvasImageFile(page, contentType) {
  const dataURL = await page.evaluate(async type => {
    const image = new Image();
    image.src = "/assets/images/customer/maker-mithila.webp";
    await image.decode();
    const canvas = document.createElement("canvas");
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    canvas.getContext("2d").drawImage(image, 0, 0);
    return canvas.toDataURL(type);
  }, contentType);
  return {
    name: contentType === "image/png" ? "qa-image.png" : "qa-image.jpg",
    mimeType: contentType,
    buffer: Buffer.from(dataURL.split(",", 2)[1], "base64"),
  };
}

async function canvasWebMFile(page) {
  const base64 = await page.evaluate(() => new Promise((resolve, reject) => {
    const canvas = document.createElement("canvas");
    canvas.width = 32;
    canvas.height = 32;
    const context = canvas.getContext("2d");
    const stream = canvas.captureStream(5);
    let recorder;
    try {
      recorder = new MediaRecorder(stream, { mimeType: "video/webm" });
    } catch (error) {
      stream.getTracks().forEach(track => track.stop());
      reject(error);
      return;
    }
    const chunks = [];
    recorder.ondataavailable = event => { if (event.data.size) chunks.push(event.data); };
    recorder.onerror = event => reject(event.error || new Error("WebM recording failed"));
    recorder.onstop = () => {
      stream.getTracks().forEach(track => track.stop());
      const reader = new FileReader();
      reader.onerror = () => reject(reader.error || new Error("WebM encoding failed"));
      reader.onload = () => resolve(String(reader.result).split(",", 2)[1]);
      reader.readAsDataURL(new Blob(chunks, { type: "video/webm" }));
    };
    let frame = 0;
    const timer = setInterval(() => {
      context.fillStyle = frame % 2 ? "#355" : "#953";
      context.fillRect(0, 0, canvas.width, canvas.height);
      frame += 1;
      if (frame === 5) {
        clearInterval(timer);
        recorder.stop();
      }
    }, 100);
    recorder.start();
  }));
  return { name: "qa-video.webm", mimeType: "video/webm", buffer: Buffer.from(base64, "base64") };
}

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
  const responsePromise = page.waitForResponse(
    response => response.url().includes("/super-admin/users/delete") && response.request().method() === "POST",
    { timeout: 30_000 }
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

async function waitForMail(request, recipient, subject, timeoutMs = 15_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const response = await request.get("http://127.0.0.1:8025/api/v1/messages?limit=50");
    expect(response.ok(), "Mailpit API should be available for local auth-flow checks").toBeTruthy();
    const mailbox = await response.json();
    const message = (mailbox.messages || []).find(item =>
      item.Subject?.includes(subject) &&
      (item.To || []).some(address => address.Address?.toLowerCase() === recipient.toLowerCase())
    );
    if (message) {
      const detailResponse = await request.get(`http://127.0.0.1:8025/api/v1/message/${message.ID}`);
      expect(detailResponse.ok()).toBeTruthy();
      return detailResponse.json();
    }
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  throw new Error(`Timed out waiting for ${subject} email to ${recipient}`);
}

function emailActionURL(message) {
  const body = `${message.Text || ""}\n${message.HTML || ""}`;
  const links = body.match(/https?:\/\/[^\s<>"']+/g) || [];
  const action = links.find(link => /\/(verify-email|reset-password)\?token=/.test(link));
  if (!action) throw new Error("Expected an account action URL in the email body");
  return action.replaceAll("&amp;", "&");
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

test("registration, email verification, password recovery and login complete end to end", async ({ browser, page, request }) => {
  test.skip(!localMutationTarget, "Account creation and recovery tests are restricted to local deployments.");
  test.setTimeout(120_000);
  const suffix = Date.now();
  const email = `qa.auth.${suffix}@weevcrafts.local`;
  const name = `QA Auth ${suffix}`;
  const initialPassword = "Local-Auth-Flow-2026!";
  const recoveredPassword = "Recovered-Auth-2026!";
  let accountCreated = false;
  let adminContext;
  let admin;

  try {
    adminContext = await browser.newContext();
    admin = await adminContext.newPage();
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/security");
    const staleAccounts = admin.locator(".role-assignment-card").filter({ hasText: /qa\.auth\./ });
    for (let index = await staleAccounts.count() - 1; index >= 0; index -= 1) {
      const staleCard = admin.locator(".role-assignment-card").filter({ hasText: /qa\.auth\./ }).nth(index);
      const staleEmail = await staleCard.locator('input[name="email"]').inputValue();
      if (staleEmail.startsWith("qa.auth.")) await deleteRoleUser(admin, staleCard, "Clean up interrupted local E2E auth-flow account");
    }

    await page.goto("/register");
    await page.locator('#page input[name="name"]').fill(name);
    await page.locator('#page input[name="email"]').fill(email);
    await page.locator('#page input[name="password"]').fill(initialPassword);
    await page.locator('#page input[name="consent"]').check();
    await page.getByRole("button", { name: "Create account" }).click();
    await page.waitForURL(/\/account\/?$/);
    accountCreated = true;
    await expect(page.locator("body")).toContainText(name);

    await page.goto("/account/sessions");
    await page.getByRole("button", { name: "Send verification link" }).click();
    await expect(page).toHaveURL(/verification=sent/);
    const verification = await waitForMail(request, email, "Verify your WeCratfs email");
    await page.goto(emailActionURL(verification));
    await page.getByRole("button", { name: "Verify email" }).click();
    await expect(page.locator('[data-backend-form-message]')).toContainText("Your email has been verified");

    await page.goto("/account/sessions");
    await page.getByRole("button", { name: "Sign out all sessions" }).click();
    await page.waitForURL(url => url.pathname === "/", { timeout: 15_000 });

    await page.goto("/forgot-password");
    await page.locator('#page input[name="email"]').fill(email);
    await page.getByRole("button", { name: "Send recovery link" }).click();
    await expect(page.locator('[data-backend-form-message]')).toContainText("recovery instructions have been sent");
    const reset = await waitForMail(request, email, "Reset your WeCratfs password");
    await page.goto(emailActionURL(reset));
    await page.locator('#page input[name="password"]').fill(recoveredPassword);
    await page.locator('#page input[name="confirm_password"]').fill(recoveredPassword);
    await page.getByRole("button", { name: "Update password" }).click();
    await expect(page.locator('[data-backend-form-message]')).toContainText("Password updated");

    await page.goto("/login");
    await page.locator('#page input[name="email"]').fill(email);
    await page.locator('#page input[name="password"]').fill(recoveredPassword);
    await page.getByRole("button", { name: /sign in/i }).click();
    await page.waitForURL(/\/account\/?$/);
    await expect(page.locator("body")).toContainText(name);
  } finally {
    if (accountCreated && admin) {
      await admin.goto("/super-admin/security");
      const card = admin.locator(".role-assignment-card").filter({ hasText: email });
      if (await card.count()) await deleteRoleUser(admin, card, "Remove local E2E auth-flow account");
    }
    if (adminContext) await adminContext.close();
  }
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

  const unknownAccountResponse = await page.request.get("/account/not-a-real-setting");
  expect(unknownAccountResponse.status()).toBe(404);
});

test("customer notification choices persist and preserve consent state", async ({ page }) => {
  test.skip(!localMutationTarget, "Account preference mutations are restricted to the local test deployment.");
  await signIn(page, "demo.customer03@weevcrafts.local", "/account");
  await page.goto("/account/notifications");
  await expect(page.locator(".newsletter form")).toHaveCount(0);
  await expect(page.getByRole("link", { name: /Manage notification preferences/ })).toHaveAttribute("href", "/account/notifications");

  const fields = {
    order_updates: page.locator('input[name="order_updates"]'),
    maker_messages: page.locator('input[name="maker_messages"]'),
    offers: page.locator('input[name="offers"]'),
    maker_stories: page.locator('input[name="maker_stories"]'),
  };
  const original = {};
  for (const [name, field] of Object.entries(fields)) original[name] = await field.isChecked();
  const changed = { ...original, order_updates: !original.order_updates, maker_messages: !original.maker_messages, offers: !original.offers, maker_stories: !original.maker_stories };

  const save = async expected => {
    for (const [name, field] of Object.entries(fields)) await field.setChecked(expected[name]);
    const responsePromise = page.waitForResponse(response => response.url().includes("/account/notifications") && response.request().method() === "POST");
    await page.getByRole("button", { name: "Save preferences" }).click();
    expect((await responsePromise).status()).toBe(303);
    await page.goto("/account/notifications");
    for (const [name, field] of Object.entries(fields)) await expect(field).toBeChecked({ checked: expected[name] });
  };

  await save(changed);
  await save(original);
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
    await ownerContext.close();
    await staffContext.close();
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
    for (const width of [360, 390, 768, 1024, 1440]) {
      await owner.setViewportSize({ width, height: 1000 });
      const dimensions = await owner.evaluate(() => ({ document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
      expect(dimensions.document, `seller team document overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
      expect(dimensions.body, `seller team body overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
      const fieldset = permissionForm.locator("fieldset");
      const columns = await fieldset.locator(":scope > label").evaluateAll(labels =>
        new Set(labels.map(label => Math.round(label.getBoundingClientRect().left))).size
      );
      expect(columns, `seller permission grid columns at ${width}px`).toBe(width <= 480 ? 1 : width <= 820 ? 2 : 4);
      const checkboxBox = await permissionForm.locator('input[type="checkbox"]').first().boundingBox();
      expect(checkboxBox.width, `permission checkbox stretches at ${width}px`).toBeLessThanOrEqual(20);
    }
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
    await adminContext.close();
    await userContext.close();
    await ownerContext.close();
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

test("marketplace promotions support create, edit, activate, pause, archive and delete", async ({ page }) => {
  test.skip(!localMutationTarget, "Promotion mutations are restricted to the local test deployment.");
  const suffix = Date.now().toString();
  const name = `QA Campaign ${suffix}`;
  const editedName = `QA Campaign Edited ${suffix}`;
  await signIn(page, "demo.marketplace01@weevcrafts.local", "/marketplace-admin");
  await page.goto("/marketplace-admin/marketing");

  const create = page.locator('form[action="/marketplace-admin/marketing/create"]');
  await create.locator('input[name="name"]').fill(name);
  await create.locator('textarea[name="description"]').fill("Browser verification campaign.");
  await create.locator('input[name="code"]').fill(`CRUD${suffix}`);
  await create.locator('select[name="discount_type"]').selectOption("percentage");
  await create.locator('input[name="discount_value"]').fill("10");
  const createResponse = page.waitForResponse(response => response.url().includes("/marketing/create") && response.request().method() === "POST");
  await create.getByRole("button", { name: "Create draft" }).click();
  expect((await createResponse).status()).toBe(303);
  await page.goto("/marketplace-admin/marketing");

  let campaign = page.locator(".admin-campaign-item").filter({ hasText: name });
  await expect(campaign).toHaveCount(1);
  await campaign.locator("summary").click();
  const edit = campaign.locator('form[action="/marketplace-admin/marketing/update"]');
  await edit.locator('input[name="name"]').fill(editedName);
  await edit.locator('input[name="code"]').fill(`EDIT${suffix}`);
  const updateResponse = page.waitForResponse(response => response.url().includes("/marketing/update") && response.request().method() === "POST");
  await edit.getByRole("button", { name: "Save changes" }).click();
  expect((await updateResponse).status()).toBe(303);
  await page.goto("/marketplace-admin/marketing");

  campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
  await campaign.locator("summary").click();
  await expect(campaign).toContainText(`EDIT${suffix}`);
  for (const [index, status, button] of [[0, "active", "Activate"], [1, "draft", "Pause"], [2, "archived", "Archive"]]) {
    const statusForm = campaign.locator('form[action="/marketplace-admin/marketing/status"]').nth(index);
    const response = page.waitForResponse(item => item.url().includes("/marketing/status") && item.request().method() === "POST");
    await statusForm.getByRole("button", { name: button }).click();
    expect((await response).status()).toBe(303);
    await page.goto("/marketplace-admin/marketing");
    campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
    await campaign.locator("summary").click();
    await expect(campaign.locator("summary")).toContainText(status);
  }
  const deleteForm = campaign.locator('form[action="/marketplace-admin/marketing/delete"]');
  const deleteResponse = page.waitForResponse(response => response.url().includes("/marketing/delete") && response.request().method() === "POST");
  page.once("dialog", dialog => dialog.accept());
  await deleteForm.getByRole("button", { name: "Delete unused campaign" }).click();
  expect((await deleteResponse).status()).toBe(303);
  await page.goto("/marketplace-admin/marketing");
  await expect(page.locator(".admin-campaign-item").filter({ hasText: editedName })).toHaveCount(0);
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
    await seller.getByRole("spinbutton", { name: "Compare-at price in paise" }).fill("299900");
    await seller.getByLabel("Initial stock").fill("5");
    await seller.getByLabel("Delivery note").fill("Delivery to serviceable PIN codes in India");
    await seller.getByLabel("Description").fill("A locally created browser test listing used to verify the WeeVCrafts cross-role order flow.");
    await seller.getByRole("button", { name: "Save draft" }).click();

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const sellerProduct = seller.locator(".seller-product-row").filter({ hasText: productName });
    await expect(sellerProduct).toHaveCount(1);
    await sellerProduct.getByRole("link", { name: "Edit product" }).click();
    const editForm = seller.locator("form.seller-form-grid").first();
    await expect(editForm.locator('input[name="name"]')).toHaveValue(productName);
    await expect(editForm.locator('input[name="slug"]')).toHaveValue(slug);
    await expect(editForm.locator('input[name="brand_name"]')).toHaveValue(`QA Artisan ${suffix}`);
    await expect(editForm.locator('input[name="brand_slug"]')).toHaveValue(`qa-artisan-${suffix}`);
    await expect(editForm.locator('input[name="category_name"]')).toHaveValue("Crafts");
    await expect(editForm.locator('input[name="category_slug"]')).toHaveValue("crafts");
    await expect(editForm.locator('input[name="sku"]')).toHaveValue(sku);
    await expect(editForm.locator('input[name="price_cents"]')).toHaveValue("249900");
    await expect(editForm.locator('input[name="compare_at_cents"]')).toHaveValue("299900");
    await expect(editForm.locator('input[name="delivery_label"]')).toHaveValue("Delivery to serviceable PIN codes in India");
    await expect(editForm.locator('textarea[name="description"]')).toHaveValue("A locally created browser test listing used to verify the WeeVCrafts cross-role order flow.");
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

    await seller.goto(`/seller-admin/inventory?q=${encodeURIComponent(`QA-ALT-EDIT-${suffix}`)}`);
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

    await mediaPanel.locator("[data-media-file]").setInputFiles({ name: "not-an-image.png", mimeType: "image/png", buffer: Buffer.from("%PDF-1.7 not a PNG file") });
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("file contents do not match", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset")).toHaveCount(2);

    await mediaPanel.locator("[data-media-file]").setInputFiles({ name: "unsupported.pdf", mimeType: "application/pdf", buffer: Buffer.from("not supported") });
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Choose a JPEG, PNG, WebP, MP4 or WebM file.");

    await mediaPanel.locator("[data-media-file]").setInputFiles({ name: "oversized.png", mimeType: "image/png", buffer: Buffer.alloc(25 * 1024 * 1024 + 1) });
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("25 MB or smaller");

    await mediaPanel.locator("[data-media-file]").setInputFiles(await canvasImageFile(seller, "image/png"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product image saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset")).toHaveCount(3);
    await expect(mediaPanel.locator(".product-media-asset p").filter({ hasText: "image/png" })).toHaveCount(1);

    await mediaPanel.locator("[data-media-file]").setInputFiles(await canvasImageFile(seller, "image/jpeg"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product image saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset")).toHaveCount(4);
    await expect(mediaPanel.locator(".product-media-asset p").filter({ hasText: "image/jpeg" })).toHaveCount(1);

    await mediaPanel.locator("[data-media-file]").setInputFiles(await canvasWebMFile(seller));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product video saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset video source[type='video/webm']")).toHaveCount(1);
    await expect(mediaPanel.locator(".product-media-asset")).toHaveCount(5);

    const savedMediaTypes = await mediaPanel.locator(".product-media-asset p").allTextContents();
    expect(savedMediaTypes.join(" ")).toContain("image/webp");
    expect(savedMediaTypes.join(" ")).toContain("image/png");
    expect(savedMediaTypes.join(" ")).toContain("image/jpeg");
    expect(savedMediaTypes.join(" ")).toContain("video/mp4");
    expect(savedMediaTypes.join(" ")).toContain("video/webm");
    const uploadedMedia = await mediaPanel.locator(".product-media-asset").evaluateAll(assets => assets.map(asset => {
      const media = asset.querySelector("img, source");
      return { url: media.src, contentType: media.tagName === "SOURCE" ? media.type : asset.querySelector("p").textContent.trim().split(/\s/)[0] };
    }));
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("button", { name: "Submit for review" }).click();

    await signIn(marketplace, "demo.marketplace01@weevcrafts.local", "/marketplace-admin");
    const couponCode = `QA${suffix}`;
    await marketplace.goto("/marketplace-admin/marketing");
    const campaignForm = marketplace.locator('form[action="/marketplace-admin/marketing/create"]');
    await campaignForm.locator('input[name="name"]').fill(`QA Checkout Campaign ${suffix}`);
    await campaignForm.locator('textarea[name="description"]').fill("End-to-end checkout coupon verification.");
    await campaignForm.locator('input[name="code"]').fill(couponCode);
    await campaignForm.locator('select[name="discount_type"]').selectOption("percentage");
    await campaignForm.locator('input[name="discount_value"]').fill("10");
    const campaignCreated = marketplace.waitForResponse(response => response.url().includes("/marketing/create") && response.request().method() === "POST");
    await campaignForm.getByRole("button", { name: "Create draft" }).click();
    expect((await campaignCreated).status()).toBe(303);
    await marketplace.goto("/marketplace-admin/marketing");
    const checkoutCampaign = marketplace.locator(".admin-campaign-item").filter({ hasText: couponCode });
    await checkoutCampaign.locator("summary").click();
    const activateCampaign = checkoutCampaign.locator('form[action="/marketplace-admin/marketing/status"]').nth(0);
    const activationResponse = marketplace.waitForResponse(response => response.url().includes("/marketing/status") && response.request().method() === "POST");
    await activateCampaign.getByRole("button", { name: "Activate" }).click();
    expect((await activationResponse).status()).toBe(303);

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
    for (const media of uploadedMedia) {
      const response = await customer.request.get(media.url);
      expect(response.status(), media.url).toBe(200);
      expect(response.headers()["content-type"]).toContain(media.contentType);
    }
    const videoButton = customer.getByRole("button", { name: "Show product media 2" });
    await expect(videoButton).toBeVisible();
    await videoButton.click();
    const productVideo = customer.locator(".gallery-main video").filter({ has: customer.locator("source[type='video/mp4']") });
    await expect(productVideo).toBeVisible();
    const videoSource = await productVideo.locator("source[type='video/mp4']").getAttribute("src");
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
    await customer.getByLabel("Coupon code").fill(couponCode);
    const checkoutRequest = customer.waitForRequest(request => request.url().includes("/api/v1/checkout") && request.method() === "POST");
    const checkoutResponse = customer.waitForResponse(response => response.url().includes("/api/v1/checkout") && response.request().method() === "POST");
    await customer.getByRole("button", { name: /Place order/ }).click();
    const checkoutPayload = JSON.parse((await checkoutRequest).postData());
    expect(checkoutPayload.address.phone).toBe("9876543210");
    expect(checkoutPayload.address.country_code).toBe("IN");
    expect(checkoutPayload.coupon_code).toBe(couponCode);
    const checkoutOrder = (await (await checkoutResponse).json()).data;
    expect(checkoutOrder.discount_cents).toBeGreaterThan(0);
    expect(checkoutOrder.total_cents).toBeLessThan(checkoutOrder.subtotal_cents + checkoutOrder.shipping_cents);
    const capture = customer.getByRole("button", { name: "Simulate successful payment" });
    await expect(capture).toBeVisible();
    const orderNumber = await capture.getAttribute("data-order-number");
    expect(orderNumber).toBeTruthy();
    await capture.click();
    await expect(customer).toHaveURL(new RegExp(`/orders/${orderNumber}$`));
    await expect(customer.locator("body")).toContainText("Confirmed");

    await seller.goto("/seller-admin/earnings");
    const saleLedgerRow = seller.locator(".ledger-row").filter({ hasText: orderNumber });
    await expect(saleLedgerRow).toHaveCount(1);
    await expect(saleLedgerRow).toContainText("open");

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

    const reviewTitle = `QA review ${suffix}`;
    const reviewBody = `The handmade item arrived as described and the finish looks lovely. ${suffix}`;
    await customer.goto("/account/reviews");
    const reviewForm = customer.locator("article[data-review-product]").filter({ hasText: productName }).locator('form[action="/account/reviews"]');
    await expect(reviewForm).toBeVisible();
    const reviewProductID = await reviewForm.locator('input[name="product_id"]').inputValue();
    const reviewOrderID = await reviewForm.locator('input[name="order_id"]').inputValue();
    const reviewCSRF = await reviewForm.locator('input[name="csrf_token"]').inputValue();
    const deniedModeration = await customer.request.post("/marketplace-admin/reviews/status", { form: {
      csrf_token: reviewCSRF, review_id: "1", decision: "published",
    } });
    expect(deniedModeration.status()).toBe(403);
    await reviewForm.locator('select[name="rating"]').selectOption("5");
    await reviewForm.locator('input[name="title"]').fill(reviewTitle);
    await reviewForm.locator('textarea[name="body"]').fill(reviewBody);
    const reviewResponsePromise = customer.waitForResponse(response => response.url().includes("/account/reviews") && response.request().method() === "POST");
    await reviewForm.getByRole("button", { name: "Submit verified review" }).click();
    expect((await reviewResponsePromise).status()).toBe(303);
    await expect(customer.locator("body")).toContainText("pending");
    const duplicateReview = await customer.request.post("/account/reviews", { form: {
      csrf_token: reviewCSRF, product_id: reviewProductID, order_id: reviewOrderID,
      rating: "5", title: reviewTitle, body: reviewBody,
    } });
    expect(duplicateReview.status()).toBe(409);

    await marketplace.goto("/marketplace-admin/reviews");
    const reviewCard = marketplace.locator("article.rounded-2xl").filter({ hasText: reviewTitle });
    await expect(reviewCard).toBeVisible();
    await expect(reviewCard).toContainText("Verified purchase");
    const approveReview = marketplace.waitForResponse(response => response.url().includes("/marketplace-admin/reviews/status") && response.request().method() === "POST");
    await reviewCard.getByRole("button", { name: "Approve and publish" }).click();
    const moderationResponse = await approveReview;
    expect(moderationResponse.status()).toBe(303);

    await customer.goto(`/products/${slug}`);
    await customer.getByRole("tab", { name: /Reviews/ }).click();
    await expect(customer.locator("#product-panel-reviews")).toContainText(reviewTitle);
    await expect(customer.locator("#product-panel-reviews")).toContainText(reviewBody);
    await expect(customer.locator("#product-panel-reviews")).toContainText("Verified purchase");
    await expect(customer.locator("#product-panel-reviews .review-highlight")).toContainText("1 reviews");

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
    for (const contentType of ["video/mp4", "video/webm"]) {
      const uploadedVideo = seller.locator(".product-media-asset").filter({ has: seller.locator(`video source[type='${contentType}']`) });
      await expect(uploadedVideo).toHaveCount(1);
      const uploadedVideoURL = await uploadedVideo.locator(`video source[type='${contentType}']`).getAttribute("src");
      seller.once("dialog", dialog => dialog.accept());
      await uploadedVideo.locator("[data-media-delete]").click();
      await expect(seller.locator(`.product-media-asset video source[type='${contentType}']`)).toHaveCount(0);
      expect((await customer.request.get(uploadedVideoURL)).status()).toBe(404);
    }

    while (await seller.locator(".product-media-asset img").count()) {
      const uploadedImage = seller.locator(".product-media-asset").filter({ has: seller.locator("img") }).first();
      const uploadedImageURL = await uploadedImage.locator("img").getAttribute("src");
      const imageCount = await seller.locator(".product-media-asset img").count();
      seller.once("dialog", dialog => dialog.accept());
      await uploadedImage.locator("[data-media-delete]").click();
      await expect(seller.locator(".product-media-asset img")).toHaveCount(imageCount - 1);
      expect((await customer.request.get(uploadedImageURL)).status()).toBe(404);
    }
    await customer.goto(`/products/${slug}`);
    await expect(customer.locator('.gallery-main img[src^="/media/products/"]')).toHaveCount(0);

    const archiveVariant = seller.locator(`form[action="/seller-admin/products/variants/archive"]:has(input[name="variant_id"][value="${addedVariantID}"])`);
    await archiveVariant.locator('input[name="reason"]').fill("Local variant archive verification after checkout");
    seller.once("dialog", dialog => dialog.accept());
    await archiveVariant.getByRole("button", { name: "Archive variant" }).click();
    await seller.goto(productEditURL);
    await expect(seller.locator(".variant-stock-summary").filter({ hasText: "archived" })).toHaveCount(1);
  } finally {
    await sellerContext.close();
    await marketplaceContext.close();
    await customerContext.close();
    await operationsContext.close();
  }
});

test("finance operator reconciles and simulates seller settlement payouts", async ({ page }) => {
  test.skip(!localMutationTarget, "Settlement mutations are restricted to the local test deployment.");
  await signIn(page, "demo.finance01@weevcrafts.local", "/finance-portal");
  await page.goto("/finance-portal/settlements");
  const panel = page.locator(".admin-panel").filter({ hasText: "Seller settlements" });
  const createResponse = page.waitForResponse(response => response.url().includes("/settlements/create") && response.request().method() === "POST");
  await panel.getByRole("button", { name: "Create settlement batches" }).click();
  expect((await createResponse).status()).toBe(303);
  await page.waitForURL(/\/finance-portal\/settlements/);

  let pendingRow = panel.locator(".table-row").filter({ hasText: "pending" }).first();
  await expect(pendingRow).toBeVisible();
  const reference = (await pendingRow.locator("span").nth(0).textContent()).trim();
  const approveResponse = page.waitForResponse(response => response.url().includes("/settlements/status") && response.request().method() === "POST");
  await pendingRow.getByRole("button", { name: "Approve" }).click();
  expect((await approveResponse).status()).toBe(303);
  await page.waitForURL(/\/finance-portal\/settlements/);

  let approvedRow = panel.locator(".table-row").filter({ hasText: reference });
  await expect(approvedRow).toContainText("approved");
  const payoutResponse = page.waitForResponse(response => response.url().includes("/settlements/status") && response.request().method() === "POST");
  page.once("dialog", dialog => dialog.accept());
  await approvedRow.getByRole("button", { name: "Record mock payout" }).click();
  expect((await payoutResponse).status()).toBe(303);
  await page.waitForURL(/\/finance-portal\/settlements/);
  await expect(panel.locator(".table-row").filter({ hasText: reference })).toContainText("paid");
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
    await customerContext.close();
    await supportContext.close();
  }
});
