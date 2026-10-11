const { test, expect } = require("@playwright/test");

const password = "WeeVCrafts-Demo-2026!";

async function signIn(page, email, landing, passwordValue = password) {
  await page.goto("/login");
  await page.locator('#page input[name="email"]').fill(email);
  await page.locator('#page input[name="password"]').fill(passwordValue);
  await page.getByRole("button", { name: /continue with email/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, "") === (landing === "/account" ? "/products" : landing).replace(/\/$/, ""));
  if (landing === "/account") await page.goto("/account");
}

function tinyPNG() {
  return Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
    "base64",
  );
}

test("reported product image and seller dropdown workflows work live", async ({ browser }) => {
  test.setTimeout(120_000);
  const suffix = Date.now().toString();
  const productName = `QA Reported Image ${suffix}`;
  const sku = `QA-REPORTED-${suffix}`;
  const sellerName = "Aditi Rao"; // Approved demo seller; new sellers require onboarding.
  const context = await browser.newContext();
  const admin = await context.newPage();
  const browserErrors = [];
  admin.on("pageerror", error => browserErrors.push(error.message));
  admin.on("console", message => {
    if (message.type() === "error") browserErrors.push(message.text());
  });
  admin.on("response", response => {
    if (response.status() >= 400) browserErrors.push(`${response.status()} ${new URL(response.url()).pathname}`);
  });
  let productCreated = false;

  try {
    await signIn(admin, "demo.super01@weevcrafts.local", "/super-admin");

    await admin.goto("/super-admin/products/new");
    const productForm = admin.locator("form.admin-product-editor-form");
    const sellerSelect = productForm.locator('select[name="seller_id"]');
    await expect(sellerSelect.locator("option", { hasText: sellerName })).toHaveCount(1);
    const sellerOption = sellerSelect.locator("option", { hasText: sellerName });
    await sellerSelect.selectOption(await sellerOption.getAttribute("value"));
    await productForm.locator('input[name="name"]').fill(productName);
    await productForm.locator('input[name="slug"]').fill(`qa-reported-image-${suffix}`);
    await productForm.locator('input[name="brand_name"]').fill(`QA Reported Maker ${suffix}`);
    await productForm.locator('input[name="brand_slug"]').fill(`qa-reported-maker-${suffix}`);
    await productForm.locator('input[name="category_name"]').fill("QA Reported Crafts");
    await productForm.locator('input[name="category_slug"]').fill(`qa-reported-crafts-${suffix}`);
    await productForm.locator('input[name="sku"]').fill(sku);
    await productForm.locator('input[name="price_rupees"]').fill("1999.00");
    await productForm.locator('input[name="initial_stock"]').fill("2");
    await productForm.locator('input[name="delivery_label"]').fill("Delivery to serviceable PIN codes in India");
    await productForm.locator('textarea[name="description"]').fill("Live verification of the super admin uploaded product image.");
    await productForm.locator('input[name="image_file"]').setInputFiles({ name: "reported.png", mimeType: "image/png", buffer: tinyPNG() });
    const createResponse = admin.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/super-admin/products/new");
    await productForm.getByRole("button", { name: "Save product" }).click();
    expect((await createResponse).status()).toBe(303);
    productCreated = true;

    await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
    const row = admin.locator(".table-row").filter({ hasText: productName });
    await expect(row).toHaveCount(1);
    await row.getByRole("link", { name: "Edit" }).click();
    const preview = admin.locator("img.product-media-preview");
    await expect(preview).toHaveAttribute("src", /\/assets\/images\/uploads\//);
    const imageResponse = await admin.request.get(await preview.getAttribute("src"));
    expect(imageResponse.status()).toBe(200);
    expect(await imageResponse.body()).toEqual(tinyPNG());
    await admin.reload();
    expect(await (await admin.request.get(await admin.locator("img.product-media-preview").getAttribute("src"))).body()).toEqual(tinyPNG());

    await admin.goto("/super-admin/users");
    const roleOptions = await admin.locator('.role-user-create-form select[name="role"] option').evaluateAll(options => options.map(option => option.value));
    expect(roleOptions).toEqual(["customer", "seller", "admin", "super_admin"]);
    expect(browserErrors).toEqual([]);
  } finally {
    if (productCreated) {
      await admin.goto(`/super-admin/products?q=${encodeURIComponent(sku)}`);
      const row = admin.locator(".table-row").filter({ hasText: productName });
      if (await row.count()) {
        const reason = row.locator('form[action="/super-admin/products/archive"] input[name="reason"]');
        if (await reason.count()) {
          await reason.fill("Clean up live reported image verification product");
          admin.once("dialog", dialog => dialog.accept());
          await row.getByRole("button", { name: "Archive" }).click();
        }
      }
    }
    await context.close();
  }
});
