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
  await page.getByRole("button", { name: "Continue with email" }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, "") === (landing === "/account" ? "/products" : landing).replace(/\/$/, ""), { timeout: 30_000 });
  if (landing === "/account") await page.goto("/account");
}

async function setHiddenInputValue(locator, value) {
  await locator.evaluate((input, nextValue) => {
    input.value = nextValue;
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
  }, value);
}

async function createMockPaidOrder(page, email) {
  await signIn(page, email, "/account");
  await page.goto("/products/natural-soap");
  const addResponse = page.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
  await page.locator(".detail-actions").getByRole("button", { name: "Add to cart",exact:true }).click();
  expect((await addResponse).ok()).toBeTruthy();

  await page.goto("/checkout");
  const savedAddresses = page.locator("#checkout-saved-address");
  if (await savedAddresses.count() && await savedAddresses.locator("option").count() > 1) {
    await savedAddresses.selectOption({ index: 1 });
  }
  const addressFields = [
    ["recipient_name", "Finance QA Customer"], ["phone", "9876543210"],
    ["line1", "12 Finance QA Road"], ["line2", "Settlement workflow"],
    ["city", "Bengaluru"], ["region", "Karnataka"], ["postal_code", "560034"],
  ];
  for (const [name, value] of addressFields) {
    const field = page.locator(`#checkout-address-form input[name="${name}"]`);
    if (await field.inputValue() === "") await field.fill(value);
  }
  await page.getByRole("button", { name: /Continue to delivery/ }).click();
  await page.getByRole("button", { name: /Continue to payment/ }).click();
  await page.getByRole("button", { name: /Continue to review/ }).click();
  const checkoutResponse = page.waitForResponse(response => response.url().includes("/api/v1/checkout") && response.request().method() === "POST");
  await page.getByRole("button", { name: /Place order/ }).click();
  expect((await checkoutResponse).ok()).toBeTruthy();
  const capture = page.getByRole("button", { name: "Simulate successful payment" });
  await expect(capture).toBeVisible();
  const orderNumber = await capture.getAttribute("data-order-number");
  await capture.click();
  await expect(page).toHaveURL(new RegExp(`/orders/${orderNumber}$`));
}

async function submitRoleChange(card, role, action, reason, sellerID = "") {
  const form = card.locator("form.role-assignment-form");
  await form.locator('select[name="role"]').selectOption(role);
  // Role changes replace the assigned role; cleanup restores the customer role.
  if (action === "revoke") await form.locator('select[name="role"]').selectOption('customer');
  await form.locator('input[name="reason"]').fill(reason);
  await form.locator('input[name="confirm"]').check();
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
  await page.waitForURL(/\/super-admin\/security(?:\?|$)/, { timeout: 30_000 });
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

test("role replacement routes to one portal and removes switching", async ({ browser }) => {
  test.skip(!localMutationTarget, "Role mutations require the local test deployment.");
  const superContext = await browser.newContext();
  const targetContext = await browser.newContext();
  const admin = await superContext.newPage();
  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/users");
    const card = admin.locator(".role-assignment-card").filter({ hasText: "demo.customer08@weevcrafts.local" });
    await submitRoleChange(card, "admin", "grant", "Local role replacement verification");
    const customer = await targetContext.newPage();
    await signIn(customer, "demo.customer08@weevcrafts.local", "/admin");
    await expect(customer.locator('a[href="/workspaces"]')).toHaveCount(0);
    await customer.goto("/workspaces");
    await expect(customer).toHaveURL(/\/admin\/?$/);
  } finally {
    await admin.goto("/super-admin/users");
    const card = admin.locator(".role-assignment-card").filter({ hasText: "demo.customer08@weevcrafts.local" });
    await submitRoleChange(card, "customer", "grant", "Restore customer role after verification");
    await targetContext.close();
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

    await page.route("**/*.js", route => route.abort());
    await page.goto("/register");
    await expect(page.locator('#page input[name="csrf_token"]')).not.toHaveValue("");
    await page.locator('#page input[name="display_name"]').fill(name);
    await page.locator('#page input[name="email"]').fill(email);
    await page.locator('#page input[name="phone"]').fill("9876543210");
    await page.locator('#page input[name="password"]').fill(initialPassword);
    await page.locator('#page input[name="consent"]').check();
    const registrationResponse = page.waitForResponse(response => {
      const pathname = new URL(response.url()).pathname;
      return response.request().method() === "POST" && (pathname === "/register" || pathname === "/api/v1/auth/register");
    });
    await page.getByRole("button", { name: "Create account" }).click();
    expect([201, 303]).toContain((await registrationResponse).status());
    await page.waitForURL(/\/products\/?$/);
    await page.unrouteAll();
    accountCreated = true;
    await page.goto("/account");
    await expect(page.locator("body")).toContainText(name);

    await page.goto("/account/sessions");
    await page.getByRole("button", { name: "Send verification link" }).click();
    await expect(page).toHaveURL(/verification=sent/);
    const verification = await waitForMail(request, email, "Verify your Weevcrafts email");
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
    const reset = await waitForMail(request, email, "Reset your Weevcrafts password");
    await page.goto(emailActionURL(reset));
    await page.locator('#page input[name="password"]').fill(recoveredPassword);
    await page.locator('#page input[name="confirm_password"]').fill(recoveredPassword);
    await page.getByRole("button", { name: "Update password" }).click();
    await expect(page.locator('[data-backend-form-message]')).toContainText("Password updated");

    await page.route("**/*.js", route => route.abort());
    await page.goto("/login");
    await page.locator('#page input[name="email"]').fill(email);
    await page.locator('#page input[name="password"]').fill(recoveredPassword);
    await page.locator('#page input[name="remember"]').check();
    await page.getByRole("button", { name: "Continue with email" }).click();
    await page.waitForURL(/\/products\/?$/);
    const sessionCookie = (await page.context().cookies()).find(cookie => cookie.name === "wecratfs_session");
    expect(sessionCookie).toBeTruthy();
    expect(sessionCookie.expires * 1000 - Date.now()).toBeGreaterThan(29 * 24 * 60 * 60 * 1000);
    await page.unrouteAll();
    await page.goto("/account");
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
  for (const phone of ["123456", "1234567890123456"]) {
    const invalidPhone = await page.request.post("/account/addresses", { form: {
      csrf_token: csrfToken, recipient_name: recipient, phone, line1: "12 QA Road", city: "Bengaluru",
      state: "Karnataka", postal_code: "560034", country_code: "IN",
    } });
    expect(invalidPhone.status(), `phone ${phone}`).toBe(422);
    await expect(invalidPhone.text()).resolves.toContain("Phone number");
  }
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

  await page.goto("/cart");
  let addedCheckoutItem = false;
  if (await page.locator(".cart-line").count() === 0) {
    await page.goto("/products/natural-soap");
    const addResponse = page.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
    await page.locator(".detail-actions").getByRole("button", { name: "Add to cart",exact:true }).click();
    expect((await addResponse).ok()).toBeTruthy();
    addedCheckoutItem = true;
  }
  await page.goto("/checkout");
  await page.locator("#checkout-saved-address").selectOption(addressID);
  await expect(page.locator('#checkout-address-form input[name="recipient_name"]')).toHaveValue(recipient);
  await expect(page.locator('#checkout-address-form input[name="line2"]')).toHaveValue("Opposite QA Library");
  if (addedCheckoutItem) {
    await page.goto("/cart");
    const cartLine = page.locator(".cart-line").first();
    const removeResponse = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname.startsWith("/ui/cart/"));
    await cartLine.locator("button.text-button").filter({ hasText: "Remove" }).click();
    expect((await removeResponse).status()).toBe(200);
  }
  await page.goto("/account/addresses");
  addressCard = page.locator(`.saved-address[data-address-card="${addressID}"]`);

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

    const analyticsForm = page.locator('form[action="/privacy/consent"]').nth(1);
    const analyticsWasEnabled = /Optional analytics are currently enabled/i.test(await page.locator("body").innerText());
    await analyticsForm.getByRole("button").click();
    await expect(page.locator("body")).toContainText(analyticsWasEnabled ? "Optional analytics are currently disabled" : "Optional analytics are currently enabled");
    await page.locator('form[action="/privacy/consent"]').nth(1).getByRole("button").click();
    await expect(page.locator("body")).toContainText(analyticsWasEnabled ? "Optional analytics are currently enabled" : "Optional analytics are currently disabled");

    const requestForm = page.locator('form[action="/privacy/requests"]');
    await requestForm.locator('select[name="request_type"]').selectOption("access");
    await requestForm.locator('textarea[name="details"]').fill(`Local E2E privacy access request ${Date.now()}`);
    await requestForm.getByRole("button", { name: "Submit request" }).click();
    await expect(page.locator("body")).toContainText(/access.*open/);
  }

  const unknownAccountResponse = await page.request.get("/account/not-a-real-setting");
  expect(unknownAccountResponse.status()).toBe(404);
});

test("customer can revoke one device session without ending the current session", async ({ browser }) => {
  test.skip(!localMutationTarget, "Session mutations are restricted to the local test deployment.");
  const currentContext = await browser.newContext();
  const otherContext = await browser.newContext();
  const current = await currentContext.newPage();
  const other = await otherContext.newPage();
  try {
    await signIn(current, "demo.customer03@weevcrafts.local", "/account");
    await signIn(other, "demo.customer03@weevcrafts.local", "/account");
    const otherCookie = (await otherContext.cookies(target.origin)).find(cookie => cookie.name === "wecratfs_session");
    expect(otherCookie).toBeTruthy();

    await current.goto("/account/sessions");
    const currentCard = current.locator(`[data-session-id="${(await currentContext.cookies(target.origin)).find(cookie => cookie.name === "wecratfs_session").value}"]`);
    await expect(currentCard).toContainText("This is your current session");
    const otherCard = current.locator(`[data-session-id="${otherCookie.value}"]`);
    await expect(otherCard).toHaveCount(1);
    const revokeResponse = current.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/account/sessions/revoke");
    await otherCard.getByRole("button", { name: "Sign out this session" }).click();
    const revokedResponse = await revokeResponse;
    expect([204, 303]).toContain(revokedResponse.status());
    if (revokedResponse.status() === 204) {
      expect(revokedResponse.headers()["hx-redirect"]).toMatch(/\/account\/sessions\?notice=Session\+signed\+out/);
    }
    await expect(current).toHaveURL(/\/account\/sessions\?notice=Session\+signed\+out/);
    await expect(current.locator(`[data-session-id="${otherCookie.value}"]`)).toHaveCount(0);
    await expect(current.locator(`[data-session-id="${(await currentContext.cookies(target.origin)).find(cookie => cookie.name === "wecratfs_session").value}"]`)).toContainText("This is your current session");

    await other.goto("/account");
    await expect(other).toHaveURL(/\/login/);
  } finally {
    if (await current.locator('form[action="/account/sessions"]').count()) {
      await current.locator('form[action="/account/sessions"]').getByRole("button", { name: "Sign out all sessions" }).click().catch(() => {});
    }
    await currentContext.close();
    await otherContext.close();
  }
});

test("customer profile name saves to PostgreSQL and restores cleanly", async ({ page }) => {
  test.skip(!localMutationTarget, "Profile mutations are restricted to the local test deployment.");
  await signIn(page, "demo.customer03@weevcrafts.local", "/account");
  await expect(page.getByRole("link", { name: "Reviews", exact: true })).toBeVisible();
  const reviewCount = page.locator(".account-stats > span").nth(2).locator("b");
  await expect(reviewCount).not.toHaveText("—");
  await page.goto("/account/profile");
  const form = page.locator('form[action="/account/profile"]');
  const originalName = await form.locator('input[name="name"]').inputValue();
  const updatedName = `${originalName} QA ${Date.now()}`;

  const saveName = async name => {
    await page.goto("/account/profile");
    const profileForm = page.locator('form[action="/account/profile"]');
    await profileForm.locator('input[name="name"]').fill(name);
    const response = page.waitForResponse(item => item.request().method() === "POST" && new URL(item.url()).pathname === "/account/profile");
    await profileForm.getByRole("button", { name: "Save changes" }).click();
    expect((await response).status()).toBe(303);
    await page.goto("/account/profile");
    await expect(page.locator('form[action="/account/profile"] input[name="name"]')).toHaveValue(name);
  };

  try {
    await saveName(updatedName);
    await page.goto("/account");
    await expect(page.locator("#profile")).toContainText(updatedName);
  } finally {
    await saveName(originalName);
  }
});

test("cart Remove deletes an entire line at quantity two", async ({ page }) => {
  test.skip(!localMutationTarget, "Cart mutations are restricted to the local test deployment.");
  await signIn(page, "demo.customer06@weevcrafts.local", "/account");
  await page.goto("/cart");
  await expect(page.locator("[data-backend-cart]")).toHaveAttribute("data-backend-ready", "true");
  let cartLine = page.locator(".cart-line").first();
  if (await cartLine.count() === 0) {
    await page.goto("/products/natural-soap");
    const addResponse = page.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
    await page.locator(".detail-actions").getByRole("button", { name: "Add to cart",exact:true }).click();
    expect((await addResponse).ok()).toBeTruthy();
    await page.goto("/cart");
    cartLine = page.locator(".cart-line").first();
  }
  const productName = await cartLine.locator("h2").innerText();
  const quantity = cartLine.locator("output");
  if (await quantity.innerText() === "1") {
    await cartLine.locator(".quantity-control button").nth(1).click();
    await expect(quantity).toHaveText("2");
  }
  const remove = cartLine.locator("button.text-button").filter({ hasText: "Remove" });
  const removeResponsePromise = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname.startsWith("/ui/cart/"));
  await remove.click();
  const removeResponse = await removeResponsePromise;
  expect(removeResponse.status()).toBe(200);
  await expect(page.locator(".cart-line").filter({ hasText: productName })).toHaveCount(0);
  if (await page.locator('.cart-line').count() === 0) {
    await expect(page.locator('.cart-summary')).toBeHidden();
    await page.goto('/checkout');
    await expect(page.getByRole('heading', { name: 'Your cart is empty', exact: true })).toBeVisible();
    await expect(page.locator('#checkout-address-form')).toHaveCount(0);
  }
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

test("seller profile edits persist while customers remain blocked", async ({ browser }) => {
  test.skip(!localMutationTarget, "Seller profile mutations are restricted to the local test deployment.");
  const suffix = Date.now().toString();
  const ownerContext = await browser.newContext();
  const customerContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  const customer = await customerContext.newPage();
  let originalName = "";
  let restored = false;

  const saveProfile = async (name, reason) => {
    await owner.goto("/seller-admin/settings");
    const form = owner.locator('form[action="/seller-admin/settings"]');
    await form.locator('input[name="display_name"]').fill(name);
    await form.locator('input[name="reason"]').fill(reason);
    const response = owner.waitForResponse(item => item.request().method() === "POST" && new URL(item.url()).pathname === "/seller-admin/settings");
    const navigation = owner.waitForEvent("framenavigated", { predicate: frame => frame === owner.mainFrame() });
    await form.getByRole("button", { name: "Save maker story" }).click();
    expect((await response).status()).toBe(303);
    await navigation;
    await owner.waitForLoadState("networkidle");
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

    await signIn(customer, "demo.customer03@weevcrafts.local", "/account");
    const customerSettings = await customer.request.get("/seller-admin/settings", { maxRedirects: 0 });
    expect(customerSettings.status()).toBe(403);
    const customerMutation = await customer.request.post("/seller-admin/settings", { form: { display_name: "Unauthorized QA Edit", reason: "Should be rejected" } });
    expect(customerMutation.status()).toBe(403);

    await saveProfile(originalName, "Restore seller profile after local verification");
    restored = true;
  } finally {
    if (originalName && !restored) {
      try {
        await saveProfile(originalName, "Restore seller profile after interrupted local verification");
      } catch {}
    }
    await ownerContext.close();
    await customerContext.close();
  }
});

test("Super Admin role selectors contain four roles and Team endpoints are retired", async ({ page }) => {
  test.skip(!localMutationTarget, "An isolated demo fixture is required.");
  await signIn(page, "demo.super01@weevcrafts.local", "/super-admin");
  await page.goto("/super-admin/users");
  const values = await page.locator('.role-user-create-form select[name="role"] option').evaluateAll(items => items.map(item => item.value));
  expect(values).toEqual(['customer', 'seller', 'admin', 'super_admin']);
  for (const route of ['/seller-admin/team', '/seller-admin/team/add', '/seller-admin/team/remove']) {
    expect((await page.request.get(route)).status()).toBe(404);
    expect((await page.request.post(route, { form: {} })).status()).toBe(404);
  }
});

test("pending seller accounts see an honest onboarding state", async ({ browser }) => {
  test.skip(!localMutationTarget, "User management mutations are restricted to the local test deployment.");
  const suffix = Date.now().toString();
  const email = `qa.pending-owner.${suffix}@example.test`;
  const userPassword = "QA-Pending-Owner-2026!";
  const adminContext = await browser.newContext();
  const ownerContext = await browser.newContext();
  const admin = await adminContext.newPage();
  const owner = await ownerContext.newPage();
  let created = false;

  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/users");
    const form = admin.locator("form.role-user-create-form");
    await form.locator('input[name="display_name"]').fill(`QA Pending Owner ${suffix}`);
    await form.locator('input[name="email"]').fill(email);
    await form.locator('input[name="password"]').fill(userPassword);
    await form.locator('select[name="role"]').selectOption("seller");
    await form.locator('input[name="reason"]').fill("Verify pending seller onboarding restrictions");
    await form.locator('input[name="confirm"]').check();
    await form.getByRole("button", { name: "Create account" }).click();
    created = true;
    await expect(admin).toHaveURL(/\/super-admin\/security/);
    await expect(admin.locator(".role-assignment-card").filter({ hasText: email })).toContainText("seller");

    await signIn(owner, email, "/seller-admin", userPassword);
    await owner.goto("/seller-admin/onboarding");
    await expect(owner.locator("body")).toContainText("awaiting marketplace review");
    await owner.goto("/seller-admin/products");
    await expect(owner.locator("body")).toContainText("Product management is locked");
    await expect(owner.getByRole("link", { name: "Create product draft" })).toHaveCount(0);
    const blockedEditor = await ownerContext.request.get("/seller-admin/products/new", { maxRedirects: 0 });
    expect(blockedEditor.status()).toBe(303);
    expect(new URL(blockedEditor.headers().location, target).pathname).toBe("/seller-admin/onboarding");
  } finally {
    if (created) {
      try {
        await admin.goto("/super-admin/users");
        const card = admin.locator(".role-assignment-card").filter({ hasText: email });
        if (await card.count()) await deleteRoleUser(admin, card, "Clean up pending seller owner E2E account");
      } catch {}
    }
    await adminContext.close();
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
    const mediaPanel = admin.locator("[data-product-media]");
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
    await form.locator('input[name="price_rupees"]').fill("2499.00");
    await form.locator('input[name="compare_at_rupees"]').fill("2999.00");
    await form.locator('input[name="initial_stock"]').fill("3");
    await form.locator('input[name="delivery_label"]').fill("Delivery to serviceable PIN codes in India");
    await form.locator('textarea[name="description"]').fill("Local end-to-end test listing created by Super Admin.");
    const creationNavigation = admin.waitForEvent("framenavigated", { predicate: frame => frame === admin.mainFrame() });
    await form.getByRole("button", { name: "Save product" }).click();
    await creationNavigation;
    await admin.waitForLoadState("networkidle");
    productCreated = true;

    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    let productRow = admin.locator("tbody tr").filter({ hasText: productName });
    await expect(productRow.locator(".ui-status")).toHaveText("Draft");
    await productRow.getByText("Manage product", { exact: true }).click();
    await productRow.getByRole("link", { name: "Edit" }).click();
    await mediaPanel.locator("[data-media-file]").setInputFiles(await canvasImageFile(admin, "image/png"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product image saved.", { timeout: 30_000 });
    await admin.reload();
    const previewImageURL = await admin.locator("img.product-media-preview").getAttribute("src");
    expect(previewImageURL).toMatch(/^\/media\/products\/\d+\/preview$/);
    expect((await admin.request.get(previewImageURL)).status()).toBe(200);

    const editForm = admin.locator("form.admin-product-editor-form").first();
    await editForm.locator('input[name="price_rupees"]').fill("2599.00");
    await editForm.locator('textarea[name="description"]').fill("Updated description, price and catalogue details.");
    const editNavigation = admin.waitForEvent("framenavigated", { predicate: frame => frame === admin.mainFrame() });
    await editForm.getByRole("button", { name: "Save product" }).click();
    await editNavigation;
    await admin.waitForLoadState("networkidle");

    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = admin.locator("tbody tr").filter({ hasText: productName });
    await expect(productRow).toContainText("INR 2599.00");
    await productRow.getByText("Manage product", { exact: true }).click();
    await productRow.locator('form[action="/super-admin/products/archive"] input[name="reason"]').fill("Local product deletion workflow test");
    const archiveNavigation = admin.waitForEvent("framenavigated", { predicate: frame => frame === admin.mainFrame() });
    await productRow.getByRole("button", { name: "Archive" }).click();
    await admin.getByRole("dialog").getByRole("button", { name: "Archive", exact: true }).click();
    await archiveNavigation;
    await admin.waitForLoadState("networkidle");
    productArchived = true;
    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = admin.locator("tbody tr").filter({ hasText: productName });
    await expect(productRow.locator(".ui-status")).toHaveText("Archived");
    await productRow.getByText("Manage product", { exact: true }).click();
    const restoreForm = productRow.locator('form[action="/super-admin/products/restore"]');
    await restoreForm.locator('input[name="reason"]').fill("Local product restore workflow test");
    const restoreNavigation = admin.waitForEvent("framenavigated", { predicate: frame => frame === admin.mainFrame() });
    await restoreForm.getByRole("button", { name: "Restore draft" }).click();
    await admin.getByRole("dialog").getByRole("button", { name: "Restore draft", exact: true }).click();
    await restoreNavigation;
    await admin.waitForLoadState("networkidle");
    productArchived = false;
    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = admin.locator("tbody tr").filter({ hasText: productName });
    await expect(productRow.locator(".ui-status")).toHaveText("Draft");
    await productRow.getByText("Manage product", { exact: true }).click();
    const archiveForm = productRow.locator('form[action="/super-admin/products/archive"]');
    await archiveForm.locator('input[name="reason"]').fill("Local E2E cleanup after restore");
    const cleanupNavigation = admin.waitForEvent("framenavigated", { predicate: frame => frame === admin.mainFrame() });
    await archiveForm.getByRole("button", { name: "Archive" }).click();
    await admin.getByRole("dialog").getByRole("button", { name: "Archive", exact: true }).click();
    await cleanupNavigation;
    await admin.waitForLoadState("networkidle");
    productArchived = true;
  } finally {
    if (productCreated && !productArchived) {
      await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
      const row = admin.locator("tbody tr").filter({ hasText: productName });
      if (await row.count()) {
        const reason = row.locator('form[action="/super-admin/products/archive"] input[name="reason"]');
        if (await reason.count()) {
          await row.getByText("Manage product", { exact: true }).click();
          await reason.fill("Local E2E cleanup");
                await row.getByRole("button", { name: "Archive" }).click();
          await admin.getByRole("dialog").getByRole("button", { name: "Archive", exact: true }).click();
          await admin.waitForLoadState("networkidle");
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
  await signIn(page, "demo.marketplace01@weevcrafts.local", "/admin");
  await page.goto("/admin/marketing");

  const create = page.locator('form[action="/admin/marketing/create"]');
  await create.locator('input[name="name"]').fill(name);
  await create.locator('textarea[name="description"]').fill("Browser verification campaign.");
  await create.locator('input[name="code"]').fill(`CRUD${suffix}`);
  await create.locator('select[name="discount_type"]').selectOption("percentage");
  await create.locator('input[name="discount_value"]').fill("10");
  const createResponse = page.waitForResponse(response => response.url().includes("/marketing/create") && response.request().method() === "POST");
  await create.getByRole("button", { name: "Create draft" }).click();
  expect((await createResponse).status()).toBe(303);
  await page.goto("/admin/marketing");

  let campaign = page.locator(".admin-campaign-item").filter({ hasText: name });
  await expect(campaign).toHaveCount(1);
  await campaign.locator("summary").click();
  const edit = campaign.locator('form[action="/admin/marketing/update"]');
  await edit.locator('input[name="name"]').fill(editedName);
  await edit.locator('input[name="code"]').fill(`EDIT${suffix}`);
  await edit.locator('select[name="discount_type"]').selectOption("fixed");
  await expect(edit.locator('input[name="discount_value"]')).toHaveAttribute("step", "0.01");
  await edit.locator('input[name="discount_value"]').fill("12.50");
  const updateResponse = page.waitForResponse(response => response.url().includes("/marketing/update") && response.request().method() === "POST");
  await edit.getByRole("button", { name: "Save changes" }).click();
  expect((await updateResponse).status()).toBe(303);
  await page.goto("/admin/marketing");

  campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
  await campaign.locator("summary").click();
  await expect(campaign).toContainText(`EDIT${suffix}`);
  let editForm = campaign.locator('form[action="/admin/marketing/update"]');
  await editForm.locator('select[name="discount_type"]').selectOption("free_shipping");
  await expect(editForm.locator('input[name="discount_value"]')).toBeDisabled();
  const freeShippingResponse = page.waitForResponse(response => response.url().includes("/marketing/update") && response.request().method() === "POST");
  await editForm.getByRole("button", { name: "Save changes" }).click();
  expect((await freeShippingResponse).status()).toBe(303);
  await page.goto("/admin/marketing");
  campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
  await campaign.locator("summary").click();
  editForm = campaign.locator('form[action="/admin/marketing/update"]');
  await editForm.locator('select[name="discount_type"]').selectOption("percentage");
  await expect(editForm.locator('input[name="discount_value"]')).toHaveAttribute("step", "1");
  await editForm.locator('input[name="discount_value"]').fill("15");
  const percentageResponse = page.waitForResponse(response => response.url().includes("/marketing/update") && response.request().method() === "POST");
  await editForm.getByRole("button", { name: "Save changes" }).click();
  expect((await percentageResponse).status()).toBe(303);
  await page.goto("/admin/marketing");
  campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
  await campaign.locator("summary").click();
  for (const [index, status, button] of [[0, "active", "Activate"], [1, "draft", "Pause"], [2, "archived", "Archive"]]) {
    const statusForm = campaign.locator('form[action="/admin/marketing/status"]').nth(index);
    const response = page.waitForResponse(item => item.url().includes("/marketing/status") && item.request().method() === "POST");
    await statusForm.getByRole("button", { name: button }).click();
    expect((await response).status()).toBe(303);
    await page.goto("/admin/marketing");
    campaign = page.locator(".admin-campaign-item").filter({ hasText: editedName });
    await campaign.locator("summary").click();
    await expect(campaign.locator("summary")).toContainText(status);
  }
  const deleteForm = campaign.locator('form[action="/admin/marketing/delete"]');
  const deleteResponse = page.waitForResponse(response => response.url().includes("/marketing/delete") && response.request().method() === "POST");
  page.once("dialog", dialog => dialog.accept());
  await deleteForm.getByRole("button", { name: "Delete unused campaign" }).click();
  expect((await deleteResponse).status()).toBe(303);
  await page.goto("/admin/marketing");
  await expect(page.locator(".admin-campaign-item").filter({ hasText: editedName })).toHaveCount(0);
});

test("seller listing approval, mock-paid order, fulfilment and return handling work across roles", async ({ browser }) => {
  test.skip(!localMutationTarget, "Cross-role workflow mutations are restricted to the local test deployment.");
  test.setTimeout(420_000);

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
    const sellerProductForm = seller.locator("form.seller-form-grid").first();
    await sellerProductForm.locator('input[name="name"]').fill(productName);
    await setHiddenInputValue(sellerProductForm.locator('input[name="slug"]'), slug);
    await setHiddenInputValue(sellerProductForm.locator('input[name="brand_name"]'), `QA Artisan ${suffix}`);
    await setHiddenInputValue(sellerProductForm.locator('input[name="brand_slug"]'), `qa-artisan-${suffix}`);
    await sellerProductForm.locator('select[name="category_slug"]').selectOption("crafts");
    await sellerProductForm.locator('input[name="sku"]').fill(sku);
    await sellerProductForm.locator('input[name="price_rupees"]').fill("2499.00");
    await sellerProductForm.locator('input[name="compare_at_rupees"]').fill("2999.00");
    await sellerProductForm.locator('input[name="initial_stock"]').fill("5");
    await sellerProductForm.locator('input[name="delivery_label"]').fill("Delivery to serviceable PIN codes in India");
    await sellerProductForm.locator('textarea[name="description"]').fill("A locally created browser test listing used to verify the Weevcrafts cross-role order flow.");
    await sellerProductForm.getByRole("button", { name: "Save product draft" }).click();

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const sellerProduct = seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: productName });
    await expect(sellerProduct).toHaveCount(1);
    await sellerProduct.getByRole("link", { name: "Edit product" }).click();
    const editForm = seller.locator("form.seller-form-grid").first();
    await expect(editForm.locator('input[name="name"]')).toHaveValue(productName);
    await expect(editForm.locator('input[name="slug"]')).toHaveValue(slug);
    await expect(editForm.locator('input[name="brand_name"]')).toHaveValue(`QA Artisan ${suffix}`);
    await expect(editForm.locator('input[name="brand_slug"]')).toHaveValue(`qa-artisan-${suffix}`);
    await expect(editForm.locator('input[name="category_name"]')).toHaveValue("Crafts");
    await expect(editForm.locator('select[name="category_slug"]')).toHaveValue("crafts");
    await expect(editForm.locator('input[name="sku"]')).toHaveValue(sku);
    await expect(editForm.locator('input[name="price_rupees"]')).toHaveValue("2499.00");
    await expect(editForm.locator('input[name="compare_at_rupees"]')).toHaveValue("2999.00");
    await expect(editForm.locator('input[name="delivery_label"]')).toHaveValue("Delivery to serviceable PIN codes in India");
    await expect(editForm.locator('textarea[name="description"]')).toHaveValue("A locally created browser test listing used to verify the Weevcrafts cross-role order flow.");
    await editForm.locator('input[name="price_rupees"]').fill("2599.00");
    await editForm.locator('textarea[name="description"]').fill("Updated local browser test product description.");
    await editForm.getByRole("button", { name: "Save product draft" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products/);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    const updatedSellerProduct = seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: productName });
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("link", { name: "Edit product" }).click();
    const variantForm = seller.locator('form[action="/seller-admin/products/variants/create"]');
    await variantForm.locator('input[name="name"]').fill("Indigo edition");
    await variantForm.locator('input[name="sku"]').fill(`QA-ALT-${suffix}`);
    await variantForm.locator('input[name="price_rupees"]').fill("2799.00");
    await variantForm.locator('input[name="compare_at_rupees"]').fill("2999.00");
    await variantForm.locator('input[name="initial_stock"]').fill("5");
    await variantForm.getByRole("button", { name: "Add variant" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products\/\d+\/edit/);
    const productEditURL = seller.url();
    const addedVariantForm = seller.locator("form.product-variant-edit").nth(1);
    const addedVariantID = await addedVariantForm.locator('input[name="variant_id"]').inputValue();
    await addedVariantForm.locator('input[name="name"]').fill("Indigo edition updated");
    await addedVariantForm.locator('input[name="sku"]').fill(`QA-ALT-EDIT-${suffix}`);
    await addedVariantForm.locator('input[name="price_rupees"]').fill("2849.00");
    await addedVariantForm.locator('input[name="compare_at_rupees"]').fill("3099.00");
    await addedVariantForm.getByRole("button", { name: "Save variant" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products\/\d+\/edit/);

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(`QA-ALT-EDIT-${suffix}`)}`);
    const alternateSKUSearchRow = seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: productName });
    await expect(alternateSKUSearchRow).toHaveCount(1);
    await expect(alternateSKUSearchRow).toContainText("2599");
    await alternateSKUSearchRow.getByRole("link", { name: "Edit product" }).click();
    const mainProductForm = seller.locator("form.seller-form-grid").first();
    await expect(mainProductForm.locator('input[name="price_rupees"]')).toHaveValue("2599.00");
    await expect(mainProductForm.locator('input[name="compare_at_rupees"]')).toHaveValue("2999.00");
    await mainProductForm.getByRole("button", { name: "Save product draft" }).click();
    await expect(seller).toHaveURL(/\/seller-admin\/products/);
    await seller.goto(productEditURL);
    const preservedAlternateVariant = seller.locator("form.product-variant-edit").nth(1);
    await expect(preservedAlternateVariant.locator('input[name="price_rupees"]')).toHaveValue("2849.00");
    await expect(preservedAlternateVariant.locator('input[name="compare_at_rupees"]')).toHaveValue("3099.00");

    await seller.goto(`/seller-admin/inventory?q=${encodeURIComponent(`QA-ALT-EDIT-${suffix}`)}`);
    const inventoryRow = seller.locator(".portal-inventory-row").filter({ hasText: `QA-ALT-EDIT-${suffix}` });
    await expect(inventoryRow).toHaveCount(1);
    await expect(inventoryRow.locator('[data-label="Available"]')).toHaveText("5");
    const inventoryForm = inventoryRow.locator('form[action="/seller-admin/inventory/adjust"]');
    await inventoryForm.locator('input[name="delta"]').fill("2");
    await inventoryForm.locator('input[name="reason"]').fill("Local E2E inventory adjustment persistence check");
    const inventoryResponse = seller.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/seller-admin/inventory/adjust");
    await inventoryForm.getByRole("button", { name: "Save" }).click();
    expect((await inventoryResponse).status()).toBe(303);
    await expect(seller).toHaveURL(/\/seller-admin\/inventory/);
    await expect(inventoryRow.locator('[data-label="Available"]')).toHaveText("7");

    await seller.goto(productEditURL);
    const mediaPanel = seller.locator("[data-product-media]");
    await mediaPanel.locator("[data-media-file]").setInputFiles(path.resolve(__dirname, "fixtures/flower.mp4"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product video saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset video source[type='video/mp4']")).toHaveCount(1);
    await expect(mediaPanel.locator(".product-media-asset img")).toHaveCount(0);
    await mediaPanel.locator("[data-media-file]").setInputFiles(path.resolve(__dirname, "../../web/assets/images/customer/maker-mithila.webp"));
    await mediaPanel.locator("[data-media-upload]").click();
    await expect(mediaPanel.locator("[data-media-status]")).toContainText("Product image saved.", { timeout: 30_000 });
    await expect(mediaPanel.locator(".product-media-asset img")).toHaveCount(1);

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
      const previewURL = new URL(media.src);
      return { url: previewURL.pathname.replace(/\/preview$/, ""), preview: previewURL.pathname, contentType: media.tagName === "SOURCE" ? media.type : asset.querySelector("p").textContent.trim().split(/\s/)[0] };
    }));
    expect((await seller.request.get(uploadedMedia[0].preview)).status()).toBe(200);
    expect((await seller.request.get(uploadedMedia[0].url)).status()).toBe(404);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    await expect(updatedSellerProduct).toContainText("2599");
    await updatedSellerProduct.getByRole("button", { name: "Submit for review" }).click();

    await signIn(marketplace, "demo.marketplace01@weevcrafts.local", "/admin");
    const couponCode = `QA${suffix}`;
    await marketplace.goto("/admin/marketing");
    const campaignForm = marketplace.locator('form[action="/admin/marketing/create"]');
    await campaignForm.locator('input[name="name"]').fill(`QA Checkout Campaign ${suffix}`);
    await campaignForm.locator('textarea[name="description"]').fill("End-to-end checkout coupon verification.");
    await campaignForm.locator('input[name="code"]').fill(couponCode);
    await campaignForm.locator('select[name="discount_type"]').selectOption("percentage");
    await campaignForm.locator('input[name="discount_value"]').fill("10");
    const campaignCreated = marketplace.waitForResponse(response => response.url().includes("/marketing/create") && response.request().method() === "POST");
    await campaignForm.getByRole("button", { name: "Create draft" }).click();
    expect((await campaignCreated).status()).toBe(303);
    await marketplace.goto("/admin/marketing");
    const checkoutCampaign = marketplace.locator(".admin-campaign-item").filter({ hasText: couponCode });
    await checkoutCampaign.locator("summary").click();
    const activateCampaign = checkoutCampaign.locator('form[action="/admin/marketing/status"]').nth(0);
    const activationResponse = marketplace.waitForResponse(response => response.url().includes("/marketing/status") && response.request().method() === "POST");
    await activateCampaign.getByRole("button", { name: "Activate" }).click();
    expect((await activationResponse).status()).toBe(303);

    await marketplace.goto(`/admin/products?q=${encodeURIComponent(sku)}`);
    const reviewRow = marketplace.locator(".table-row").filter({ hasText: productName });
    await expect(reviewRow).toHaveCount(1);
    await reviewRow.getByRole("button", { name: "Approve" }).click();

    const latestUploadedImage = uploadedMedia.filter(media => media.contentType.startsWith("image/")).at(-1);
    await customer.goto(`/products/${slug}`);
    await expect(customer.locator(".gallery-main img:visible")).toHaveAttribute("src", latestUploadedImage.url);

    await customer.goto(`/products?category=crafts&category=arts&brand=qa-artisan-${suffix}&price=2500-5000&availability=in-stock&sort=price-desc`);
    await expect(customer.locator(`article[data-product="${slug}"]`)).toHaveCount(1);
    await expect(customer.locator("[data-backend-result-count]")).toHaveText("1 results");
    await customer.goto(`/products?category=home-living&brand=qa-artisan-${suffix}`);
    await expect(customer.locator(`article[data-product="${slug}"]`)).toHaveCount(0);
    await customer.goto(`/products?category=crafts&brand=qa-artisan-${suffix}&price=under-1000`);
    await expect(customer.locator(`article[data-product="${slug}"]`)).toHaveCount(0);
    await customer.goto(`/deals?brand=qa-artisan-${suffix}`);
    await expect(customer.locator(`article[data-product="${slug}"]`)).toHaveCount(1);

    await signIn(customer, "demo.customer07@weevcrafts.local", "/account");
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
    await expect(customer.locator("[data-backend-cart]")).toHaveAttribute("data-backend-ready", "true");
    const existingCartLines = customer.locator(".cart-line");
    while (await existingCartLines.count()) {
      const currentCartLineCount = await existingCartLines.count();
      await existingCartLines.first().getByRole("button", { name: "Remove", exact: true }).click();
      await expect(existingCartLines).toHaveCount(currentCartLineCount - 1);
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
      expect((await customer.request.get(media.preview)).status()).toBe(404);
    }
    const videoMedia = uploadedMedia.find(media => media.contentType === "video/mp4");
    const rangeResponse = await customer.request.get(videoMedia.url, { headers: { Range: "bytes=0-99" } });
    expect(rangeResponse.status()).toBe(206);
    expect(rangeResponse.headers()["content-range"]).toMatch(/^bytes 0-99\//);
    expect((await rangeResponse.body()).byteLength).toBe(100);
    const videoButton = customer.locator(".gallery .thumbs button:has(.gallery-video-thumb)").first();
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
    await customer.getByRole("button", { name: "Add to cart", exact: true }).click();
    expect((await cartResponse).ok()).toBeTruthy();
    await customer.goto("/cart");
    await expect(customer.locator("body")).toContainText(productName);
    await expect(customer.locator("body")).toContainText("Indigo edition");
    const cartRow = customer.locator(".cart-seller").filter({ hasText: productName });
    await expect(cartRow.locator("output")).toHaveText("1");
    const quantityButtons = cartRow.locator(".quantity-control button");
    await expect(quantityButtons).toHaveCount(2);
    await quantityButtons.nth(1).click({ timeout: 10_000 });
    await expect(cartRow.locator("output")).toHaveText("2");
    await quantityButtons.nth(0).click({ timeout: 10_000 });
    await expect(cartRow.locator("output")).toHaveText("1");
    await quantityButtons.nth(1).click({ timeout: 10_000 });
    await expect(cartRow.locator("output")).toHaveText("2");
    await cartRow.locator("button.text-button").filter({ hasText: "Remove" }).click();
    await expect(cartRow).toHaveCount(0);
    await customer.goto(`/products/${slug}`);
    await customer.locator("#detail-variant").selectOption(variantValue);
    const restoreCartResponse = customer.waitForResponse(response => response.url().includes("/api/v1/cart/add") && response.request().method() === "POST");
    await customer.getByRole("button", { name: "Add to cart", exact: true }).click();
    expect((await restoreCartResponse).ok()).toBeTruthy();
    await customer.goto("/cart");
    await expect(cartRow.locator("output")).toHaveText("1");
    await expect(cartRow).toContainText("Indigo edition updated");
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
    const quoteResponse = customer.waitForResponse(response => response.url().includes("/api/v1/checkout/quote") && response.request().method() === "GET");
    await customer.getByRole("button", { name: "Apply coupon" }).click();
    expect((await quoteResponse).ok()).toBeTruthy();
    await expect(customer.locator("[data-quote-discount]")).not.toHaveText("INR 0.00");
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
    await expect(customer.getByRole("button", { name: "Cancel order" })).toHaveCount(0);
    await customer.getByRole("link", { name: "View/print receipt" }).click();
    await expect(customer.getByRole("heading", { name: "Order receipt" })).toBeVisible();
    await expect(customer.locator("body")).toContainText(orderNumber);
    await customer.goto(`/orders/${orderNumber}`);

    await customer.goto(`/products/${slug}`);
    const saveProduct = customer.getByRole("button", { name: `Save ${productName}` });
    const saveProductResponse = customer.waitForResponse(response => new URL(response.url()).pathname === "/api/v1/wishlist/add" && response.request().method() === "POST");
    await saveProduct.click();
    expect((await saveProductResponse).ok()).toBeTruthy();
    await customer.goto("/wishlist");
    const savedCard = customer.locator(".saved-card").filter({ hasText: productName });
    await expect(savedCard).toHaveCount(1);
    const removeSavedProduct = customer.waitForResponse(response => new URL(response.url()).pathname.startsWith("/api/v1/wishlist/") && response.request().method() === "DELETE");
    await savedCard.locator("button.text-button").filter({ hasText: "Remove" }).click();
    expect((await removeSavedProduct).ok()).toBeTruthy();
    await expect(customer.locator(".saved-card").filter({ hasText: productName })).toHaveCount(0);

    await seller.goto("/seller-admin/earnings");
    const saleLedgerRow = seller.locator(".ledger-row").filter({ hasText: orderNumber });
    await expect(saleLedgerRow).toHaveCount(1);
    await expect(saleLedgerRow).toContainText("open");

    const sellerOrdersPath = `/seller-admin/orders?q=${encodeURIComponent(orderNumber)}`;
    for (const status of ["processing", "shipped", "delivered"]) {
      await seller.goto(sellerOrdersPath);
      const sellerOrder = seller.locator(".seller-order-card").filter({ hasText: orderNumber });
      await expect(sellerOrder).toHaveCount(1);
      await sellerOrder.getByRole("link", { name: "Open order" }).click();
      const fulfilmentForm = seller.locator('form[action="/seller-admin/orders/fulfill"]');
      await fulfilmentForm.locator('select[name="status"]').selectOption(status);
      await fulfilmentForm.locator('select[name="carrier"]').selectOption("Other");
      await fulfilmentForm.locator('input[name="custom_carrier"]').fill("Browser QA Carrier");
      await fulfilmentForm.locator('input[name="tracking_number"]').fill(`QA${suffix}`);
      const fulfillmentResponse = seller.waitForResponse(response => response.url().includes("/seller-admin/orders/fulfill") && response.request().method() === "POST");
      await fulfilmentForm.getByRole("button", { name: "Save fulfilment" }).click();
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
    const deniedModeration = await customer.request.post("/admin/reviews/status", { form: {
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

    await marketplace.goto("/admin/reviews");
    const reviewCard = marketplace.getByRole("article").filter({ hasText: reviewTitle });
    await expect(reviewCard).toBeVisible();
    await expect(reviewCard).toContainText("Verified purchase");
    const approveReview = marketplace.waitForResponse(response => response.url().includes("/admin/reviews/status") && response.request().method() === "POST");
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

    await signIn(operations, "demo.operations01@weevcrafts.local", "/admin");
    await operations.goto(`/admin/returns?q=${encodeURIComponent(orderNumber)}`);
    const returnRow = operations.locator(".portal-return-item").filter({ hasText: orderNumber });
    await expect(returnRow).toHaveCount(1);
    await returnRow.locator('select[name="status"]').selectOption("approved");
    await returnRow.getByRole("button", { name: "Save decision" }).click();
    await operations.goto(`/admin/returns?q=${encodeURIComponent(orderNumber)}`);
    const approvedReturn = operations.locator(".portal-return-item").filter({ hasText: orderNumber });
    await expect(approvedReturn).toContainText("approved");
    await customer.goto("/returns");
    const customerReturn = customer.locator("article.return-card").filter({ hasText: orderNumber });
    await customerReturn.getByRole("link", { name: "Print return authorization" }).click();
    await expect(customer.getByRole("heading", { name: "Return authorization slip" })).toBeVisible();
    await expect(customer.locator(".return-slip")).toContainText(orderNumber);
    await expect(customer.locator(".return-slip")).toContainText(shippingRecipient);
    await approvedReturn.getByRole("button", { name: "Mark returned order received" }).click();

    await customer.goto(`/orders/${orderNumber}`);
    await expect(customer.getByRole("heading", { name: "Request your refund" })).toBeVisible();
    await customer.getByLabel("Refund reason").fill("The returned item was inspected and the full refund is requested.");
    const refundResponse = customer.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/orders/${orderNumber}/refund`);
    await customer.getByRole("button", { name: "Request full refund" }).click();
    expect((await refundResponse).status()).toBe(303);
    await expect(customer).toHaveURL(new RegExp(`/orders/${orderNumber}$`));
    await expect(customer.locator("body")).toContainText("processed");
    await expect(customer.locator("body")).toContainText("Refunded");

    await customer.goto("/returns?tab=refunds");
    const refundedReturn = customer.locator("article.return-card").filter({ hasText: orderNumber });
    await expect(refundedReturn).toContainText("Refunded");

    await customer.goto("/account/addresses");
    const savedShippingAddresses = customer.locator(".saved-address").filter({ hasText: shippingRecipient });
    expect(await savedShippingAddresses.count()).toBeGreaterThan(0);
    const savedShippingAddress = savedShippingAddresses.first();
    await expect(savedShippingAddress.locator('input[name="phone"]')).toHaveValue("9876543210");
    const matchingSavedAddresses = customer.locator(".saved-address").filter({ hasText: shippingRecipient });
    while (await matchingSavedAddresses.count()) {
      const currentAddressCount = await matchingSavedAddresses.count();
      customer.once("dialog", dialog => dialog.accept());
      await matchingSavedAddresses.first().locator("form[data-address-delete]").getByRole("button", { name: "Delete address" }).click();
      await expect(matchingSavedAddresses).toHaveCount(currentAddressCount - 1);
    }

    await seller.goto(productEditURL);
    for (const contentType of ["video/mp4", "video/webm"]) {
      const uploadedVideo = seller.locator(".product-media-asset").filter({ has: seller.locator(`video source[type='${contentType}']`) });
      await expect(uploadedVideo).toHaveCount(1);
      const uploadedVideoURL = await uploadedVideo.locator(`video source[type='${contentType}']`).getAttribute("src");
      const publicVideoURL = uploadedVideoURL.replace(/\/preview$/, "");
      seller.once("dialog", dialog => dialog.accept());
      await uploadedVideo.locator("[data-media-delete]").click();
      await expect(seller.locator(`.product-media-asset video source[type='${contentType}']`)).toHaveCount(0);
      expect((await customer.request.get(publicVideoURL)).status()).toBe(404);
    }

    let primaryImageFallbackVerified = false;
    while (await seller.locator(".product-media-asset img").count()) {
      const imageAssets = seller.locator(".product-media-asset").filter({ has: seller.locator("img") });
      const uploadedImage = imageAssets.last();
      const uploadedImageURL = await uploadedImage.locator("img").getAttribute("src");
      const imageCount = await imageAssets.count();
      const expectedFallbackURL = imageCount > 1 ? (await imageAssets.nth(imageCount - 2).locator("img").getAttribute("src")).replace(/\/preview$/, "") : "";
      seller.once("dialog", dialog => dialog.accept());
      await uploadedImage.locator("[data-media-delete]").click();
      await expect(seller.locator(".product-media-asset img")).toHaveCount(imageCount - 1);
      expect((await customer.request.get(uploadedImageURL)).status()).toBe(404);
      if (!primaryImageFallbackVerified && expectedFallbackURL) {
        await customer.goto(`/products/${slug}`);
        await expect(customer.locator(".gallery-main img:visible")).toHaveAttribute("src", expectedFallbackURL);
        primaryImageFallbackVerified = true;
      }
    }
    await customer.goto(`/products/${slug}`);
    await expect(customer.locator('.gallery-main img[src^="/media/products/"]')).toHaveCount(0);

    const archiveVariant = seller.locator(`form[action="/seller-admin/products/variants/archive"]:has(input[name="variant_id"][value="${addedVariantID}"])`);
    await archiveVariant.locator('input[name="reason"]').fill("Local variant archive verification after checkout");
    seller.once("dialog", dialog => dialog.accept());
    await archiveVariant.getByRole("button", { name: "Archive variant" }).click();
    await seller.goto(productEditURL);
    await expect(seller.locator(".variant-stock-summary").filter({ hasText: "archived" })).toHaveCount(1);

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    let productRow = seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: sku });
    await expect(productRow).toHaveCount(1);
    await productRow.locator('form[action="/seller-admin/products/archive"] input[name="reason"]').fill("Local seller archive workflow verification");
    const archiveResponse = seller.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/seller-admin/products/archive");
    seller.once("dialog", dialog => dialog.accept());
    await productRow.locator('form[action="/seller-admin/products/archive"] button').click();
    expect((await archiveResponse).status()).toBe(303);

    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    productRow = seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: sku });
    await expect(productRow).toContainText("archived");
    await productRow.locator('form[action="/seller-admin/products/restore"] input[name="reason"]').fill("Local seller restore workflow verification");
    const restoreResponse = seller.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/seller-admin/products/restore");
    seller.once("dialog", dialog => dialog.accept());
    await productRow.locator('form[action="/seller-admin/products/restore"] button').click();
    expect((await restoreResponse).status()).toBe(303);
    await seller.goto(`/seller-admin/products?q=${encodeURIComponent(sku)}`);
    await expect(seller.locator(".portal-product-row, .seller-product-row").filter({ hasText: sku })).toContainText("draft");
  } finally {
    await sellerContext.close();
    await marketplaceContext.close();
    await customerContext.close();
    await operationsContext.close();
  }
});

test("Admin reconciles and simulates seller settlement payouts", async ({ browser, page }) => {
  test.skip(!localMutationTarget, "Settlement mutations are restricted to the local test deployment.");
  await signIn(page, "demo.finance01@weevcrafts.local", "/admin");
  await page.goto("/admin/settlements");
  const panel = page.locator(".admin-panel").filter({ hasText: "Seller settlements" });
  const createResponse = page.waitForResponse(response => response.url().includes("/settlements/create") && response.request().method() === "POST");
  await panel.getByRole("button", { name: "Create settlement batches" }).click();
  expect((await createResponse).status()).toBe(303);
  await page.waitForURL(/\/admin\/settlements/);

  let pendingRow = panel.locator(".table-row").filter({ hasText: "pending" }).first();
  if (await pendingRow.count() === 0) {
    const customerContext = await browser.newContext();
    try {
      await createMockPaidOrder(await customerContext.newPage(), "demo.customer05@weevcrafts.local");
    } finally {
      await customerContext.close();
    }
    const retryBatchResponse = page.waitForResponse(response => response.url().includes("/settlements/create") && response.request().method() === "POST");
    await panel.getByRole("button", { name: "Create settlement batches" }).click();
    expect((await retryBatchResponse).status()).toBe(303);
    await page.waitForURL(/\/admin\/settlements/);
  }
  await expect(pendingRow).toBeVisible();
  const reference = (await pendingRow.locator("span").nth(0).textContent()).trim();
  const settlementID = await pendingRow.locator('input[name="settlement_id"]').first().inputValue();
  const csrfToken = await pendingRow.locator('input[name="csrf_token"]').first().inputValue();
  const approveResponse = page.waitForResponse(response => response.url().includes("/settlements/status") && response.request().method() === "POST");
  await pendingRow.getByRole("button", { name: "Approve" }).click();
  expect((await approveResponse).status()).toBe(303);
  await page.waitForURL(/\/admin\/settlements/);

  let approvedRow = panel.locator(".table-row").filter({ hasText: reference });
  await expect(approvedRow).toContainText("approved");
  const payoutResponse = page.waitForResponse(response => response.url().includes("/settlements/status") && response.request().method() === "POST");
  page.once("dialog", dialog => dialog.accept());
  await approvedRow.getByRole("button", { name: "Record mock payout" }).click();
  expect((await payoutResponse).status()).toBe(303);
  await page.waitForURL(/\/admin\/settlements/);
  const paidRow = panel.locator(".table-row").filter({ hasText: reference });
  await expect(paidRow).toContainText("paid");

  const terminalMutation = await page.request.post("/admin/settlements/status", {
    maxRedirects: 0,
    form: { csrf_token: csrfToken, settlement_id: settlementID, status: "failed" },
  });
  expect(terminalMutation.status()).toBe(409);
  await page.reload();
  await expect(panel.locator(".table-row").filter({ hasText: reference })).toContainText("paid");
});

test("customer and Admin share a persisted ticket conversation", async ({ browser }) => {
  test.skip(!localMutationTarget, "Support workflow mutations are restricted to the local test deployment.");

  const suffix = Date.now().toString();
  const subject = `QA support request ${suffix}`;
  const customerContext = await browser.newContext();
  const supportContext = await browser.newContext();
  const adminContext = await browser.newContext();
  const admin = await adminContext.newPage();
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

    await customer.reload();
    await expect(customer.locator(`#ticket-${ticketID}`)).toContainText(subject);
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");
    await admin.goto("/super-admin/support");
    await expect(admin.locator("body")).toContainText(subject);

    await signIn(support, "demo.support01@weevcrafts.local", "/admin");
    await support.goto(`/admin/support/cases?ticket_id=${ticketID}`);
    await expect(support.locator("body")).toContainText(subject);
    await expect(support.locator("body")).toContainText("Please help me check a question about this handmade order.");
    const agentReply = `We have received your request ${suffix} and will follow up here.`;
    const replyForm = support.locator('form[action="/admin/support/cases/reply"]');
    await replyForm.locator('textarea[name="message"]').fill(agentReply);
    await replyForm.getByRole("button", { name: "Send Reply" }).click();
    await expect(support).toHaveURL(new RegExp(`/admin/support/cases\\?ticket_id=${ticketID}`));

    await customer.goto(`/account/support?ticket_id=${ticketID}`);
    const customerTicket = customer.locator(`#ticket-${ticketID}`);
    await expect(customerTicket).toContainText(agentReply);
    const customerReply = `Additional details for support request ${suffix}.`;
    const customerReplyForm = customerTicket.locator('form[action="/support/customer-reply"]');
    await customerReplyForm.locator('textarea[name="message"]').fill(customerReply);
    await customerReplyForm.getByRole("button", { name: "Send reply" }).click();
    await expect(customer).toHaveURL(new RegExp(`/account/support\\?ticket_id=${ticketID}`));

    await support.goto(`/admin/support/cases?ticket_id=${ticketID}`);
    await expect(support.locator("body")).toContainText(customerReply);
    const updateForm = support.locator('.live-case-detail form[action="/admin/support/cases/status"]');
    await updateForm.locator('select[name="status"]').selectOption("resolved");
    await updateForm.locator('input[name="note"]').fill("Resolved after customer follow-up.");
    await updateForm.getByRole("button", { name: "Save status" }).click();
    await expect(support).toHaveURL(new RegExp(`/admin/support/cases\\?ticket_id=${ticketID}`));
    const closeForm = support.locator('.live-case-detail form[action="/admin/support/cases/status"]');
    await closeForm.locator('select[name="status"]').selectOption("closed");
    await closeForm.locator('input[name="note"]').fill("Resolved after customer follow-up.");
    await closeForm.getByRole("button", { name: "Save status" }).click();
    await expect(support).toHaveURL(new RegExp(`/admin/support/cases\\?ticket_id=${ticketID}`));
    await customer.goto(`/account/support?ticket_id=${ticketID}`);
    await expect(customer.locator(`#ticket-${ticketID}`)).toContainText("closed");
    await expect(customer.locator(`#ticket-${ticketID} form[action="/support/customer-reply"]`)).toHaveCount(0);
  } finally {
    await adminContext.close();
    await customerContext.close();
    await supportContext.close();
  }
});
