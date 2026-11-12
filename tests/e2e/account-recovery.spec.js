const { randomBytes } = require("node:crypto");
const { test, expect } = require("@playwright/test");

const origin = process.env.WEEVCRAFTS_AUTH_QA_URL;
const mailpit = process.env.WEEVCRAFTS_AUTH_QA_MAILPIT;
const firebaseOrigin = process.env.WEEVCRAFTS_AUTH_QA_FIREBASE_URL;
const emulator = process.env.WEEVCRAFTS_AUTH_QA_FIREBASE_EMULATOR;
const oldPassword = "Original test password 2026";
const newPassword = "Replacement test password 2026";

function isLoopback(value) {
  return value && new URL(value).hostname === "127.0.0.1";
}

test.skip(!isLoopback(origin) || !isLoopback(mailpit), "Requires an isolated local application and Mailpit.");

async function register(page, email, username) {
  await page.goto(origin + "/register?next=%2Fproducts");
  await page.getByLabel("Full name").fill("Shared Full Name");
  await page.getByRole("textbox", { name: "Username", exact: true }).fill(username);
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Phone number").fill("9876543210");
  await page.getByLabel("Password", { exact: true }).fill(oldPassword);
  await page.locator('input[name="consent"]').check();
  await page.locator('form[action="/register"]').getByRole("button", { name: "Create account" }).click();
}

async function resetEmail(request, email, previousID) {
  let message;
  await expect.poll(async () => {
    const response = await request.get(mailpit + "/api/v1/messages");
    const { messages } = await response.json();
    message = messages.find(item => item.ID !== previousID && item.To.some(address => address.Address === email) &&
      item.Subject === "Reset your Weevcrafts password");
    return Boolean(message);
  }).toBe(true);
  const response = await request.get(mailpit + "/api/v1/message/" + message.ID);
  return { id: message.ID, text: (await response.json()).Text };
}

async function firebaseAction(request, action, data) {
  return request.post(emulator + "/identitytoolkit.googleapis.com/v1/accounts:" + action + "?key=fake-api-key", { data });
}

test("rejects a duplicate username and completes native recovery without JavaScript", async ({ browser, page, request }, testInfo) => {
  const suffix = randomBytes(6).toString("hex");
  const username = "maker_" + suffix;
  const email = username + "@example.invalid";
  await register(page, email, username);
  await expect(page).toHaveURL(/\/products$/);
  const duplicateEmail = await request.post(origin + "/api/v1/auth/register", {
    data: { email, username: "unique_" + suffix, display_name: "Shared Full Name", phone: "9876543210", password: oldPassword, accepted_terms: true }
  });
  expect(duplicateEmail.status()).toBe(409);
  expect((await duplicateEmail.json()).error.code).toBe("EMAIL_TAKEN");

  const other = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const plain = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
  try {
    const signup = await other.newPage();
    const otherEmail = "other_" + email;
    await register(signup, otherEmail, username.toUpperCase());
    const field = signup.getByRole("textbox", { name: "Username", exact: true });
    await expect(field).toBeFocused();
    await expect(field).toHaveAttribute("aria-invalid", "true");
    const conflict = signup.getByRole("alert");
    await expect(conflict).toContainText("This username is already registered. Choose another.");
    await expect(conflict).toBeInViewport();
    await expect(signup.getByLabel("Email address")).toHaveValue(otherEmail);
    expect(await signup.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await signup.screenshot({ path: testInfo.outputPath("username-conflict-mobile.png") });
    await field.fill("other_" + username);
    await expect(field).not.toHaveAttribute("aria-invalid", "true");
    await field.press("Enter");
    await expect(signup).toHaveURL(/\/products$/);

    const recovery = await plain.newPage();
    await recovery.goto(origin + "/forgot-password");
    await recovery.getByLabel("Email address").fill(email);
    await recovery.getByRole("button", { name: "Send recovery link" }).click();
    await expect(recovery.getByRole("status")).toContainText("we’ll send a reset link");
    const link = (await resetEmail(request, email)).text.match(/https?:\/\/\S+\/reset-password\?token=[\w-]+/)[0];
    expect(new URL(link).origin).toBe(origin);
    const landing = await recovery.goto(link);
    expect(landing.headers()["referrer-policy"]).toBe("no-referrer");
    expect(landing.headers()["cache-control"]).toContain("no-store");
    await recovery.getByLabel("New password", { exact: true }).fill(newPassword);
    await recovery.getByLabel("Confirm password", { exact: true }).fill("Different password 2026");
    await recovery.getByRole("button", { name: "Update password" }).click();
    await expect(recovery.getByRole("alert")).toContainText("The passwords do not match");
    await recovery.getByLabel("New password", { exact: true }).fill(newPassword);
    await recovery.getByLabel("Confirm password", { exact: true }).fill(newPassword);
    await recovery.getByRole("button", { name: "Update password" }).click();
    await expect(recovery.getByRole("heading", { name: "Password updated" })).toBeVisible();
    await expect(recovery.getByLabel("New password", { exact: true })).toHaveCount(0);
    await recovery.screenshot({ path: testInfo.outputPath("reset-success-mobile.png") });

    const oldSession = await page.request.get(origin + "/account", { maxRedirects: 0 });
    expect(oldSession.status()).toBe(303);
    expect(oldSession.headers().location).toMatch(/^\/login/);
    const login = password => request.post(origin + "/api/v1/auth/login", { data: { email: username, password } });
    expect((await login(oldPassword)).status()).toBe(401);
    expect((await login(newPassword)).status()).toBe(200);

    const reused = await recovery.goto(link);
    expect(reused.status()).toBe(422);
    await expect(recovery.getByRole("link", { name: "Request a new reset link" })).toBeVisible();
    await recovery.goto(origin + "/reset-password");
    await expect(recovery.getByRole("link", { name: "Request a new reset link" })).toBeVisible();
  } finally {
    await other.close();
    await plain.close();
  }
});

test("Google signup keeps verified proof while choosing another username", async ({ page, request }) => {
  test.skip(!isLoopback(firebaseOrigin) || !isLoopback(emulator), "Requires the local Firebase auth emulator.");
  const suffix = randomBytes(6).toString("hex");
  const taken = "google_" + suffix;
  const reserved = await request.post(firebaseOrigin + "/api/v1/auth/register", {
    data: { email: taken + "@example.invalid", username: taken, display_name: "Fixture", phone: "9876543210", password: oldPassword, accepted_terms: true }
  });
  expect(reserved.status()).toBe(201);
  const credential = await firebaseAction(request, "signInWithIdp", {
    requestUri: firebaseOrigin,
    postBody: new URLSearchParams({
      providerId: "google.com",
      id_token: JSON.stringify({ sub: "qa-" + suffix, email: "google-" + suffix + "@example.invalid", email_verified: true, name: "Google fixture" })
    }).toString(),
    returnSecureToken: true
  });
  expect(credential.status()).toBe(200);
  const { idToken } = await credential.json();
  const scriptHeaders = { "access-control-allow-origin": "*" };
  await page.route("**/firebase-app.js", route => route.fulfill({
    contentType: "text/javascript",
    headers: scriptHeaders,
    body: "export const initializeApp = config => config;"
  }));
  await page.route("**/firebase-auth.js", route => route.fulfill({
    contentType: "text/javascript",
    headers: scriptHeaders,
    body: `
      export const getAuth = () => ({});
      export const setPersistence = async () => {};
      export const inMemoryPersistence = {};
      export class GoogleAuthProvider {setCustomParameters() {}}
      export async function signInWithPopup() {
        window.fixturePopupCount = (window.fixturePopupCount || 0) + 1;
        return {user:{emailVerified:true,getIdToken:async()=>${JSON.stringify(idToken)}}};
      }
    `
  }));
  await page.goto(firebaseOrigin + "/register?next=%2Fproducts");
  const panel = page.locator("[data-firebase-google-panel]");
  await panel.locator("[data-google-terms]").check();
  await panel.getByRole("button", { name: "Continue with Google" }).click();
  const form = panel.locator("[data-google-username]");
  const username = form.getByLabel("Username");
  await expect(username).toBeFocused();
  await username.fill(taken.toUpperCase());
  await form.getByRole("button", { name: "Create account" }).click();
  await expect(form.getByRole("alert")).toContainText("This username is already registered.");
  await expect(username).toBeFocused();
  expect(await page.evaluate(() => window.fixturePopupCount)).toBe(1);
  await username.fill("new_" + taken);
  await username.press("Enter");
  await expect(page).toHaveURL(/\/products$/);

  await page.context().clearCookies();
  await page.goto(firebaseOrigin + "/login?next=%2Fproducts");
  await panel.locator("[data-google-terms]").check();
  await panel.getByRole("button", { name: "Continue with Google" }).click();
  await expect(page).toHaveURL(/\/products$/);
});

test("Firebase password recovery works before and after account registration", async ({ page, request }) => {
  test.skip(!isLoopback(firebaseOrigin) || !isLoopback(emulator), "Requires the local Firebase auth emulator.");
  const username = "firebase_" + randomBytes(6).toString("hex");
  const email = username + "@example.invalid";
  const signup = await firebaseAction(request, "signUp", { email, password: oldPassword, returnSecureToken: true });
  expect(signup.status()).toBe(200);

  const recover = async (password, previousID) => {
    const response = await request.post(firebaseOrigin + "/api/v1/auth/password-reset/request", { data: { email } });
    expect(response.status()).toBe(202);
    const { id, text } = await resetEmail(request, email, previousID);
    const link = new URL(text.match(/https?:\/\/\S+mode=resetPassword[^\s]*/)[0]);
    expect(link.origin).toBe(emulator);
    expect(link.searchParams.get("continueUrl")).toBe(firebaseOrigin + "/login");
    const oobCode = link.searchParams.get("oobCode");
    expect((await firebaseAction(request, "resetPassword", { oobCode, newPassword: password })).status()).toBe(200);
    expect((await firebaseAction(request, "resetPassword", { oobCode, newPassword: password })).status()).toBe(400);
    return id;
  };

  const firstMessage = await recover(newPassword);
  const credential = await firebaseAction(request, "signInWithPassword", { email, password: newPassword, returnSecureToken: true });
  expect(credential.status()).toBe(200);
  const { idToken } = await credential.json();
  await page.goto(firebaseOrigin + "/register");
  const csrf = await page.locator('form[action="/register"] [name="csrf_token"]').inputValue();
  const completed = await page.request.post(firebaseOrigin + "/api/v1/auth/firebase/session", {
    headers: { Origin: firebaseOrigin, "X-CSRF-Token": csrf },
    data: { id_token: idToken, username, accepted_terms: true, phone: "9876543210" }
  });
  expect(completed.status()).toBe(200);
  const login = password => request.post(firebaseOrigin + "/api/v1/auth/login", { data: { email: username.toUpperCase(), password } });
  expect((await login(oldPassword)).status()).toBe(401);
  expect((await login(newPassword)).status()).toBe(200);

  await recover(oldPassword, firstMessage);
  expect((await login(newPassword)).status()).toBe(401);
  expect((await login(oldPassword)).status()).toBe(200);
});
