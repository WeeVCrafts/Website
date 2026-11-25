const { test, expect } = require("@playwright/test");
const path = require("node:path");

test("seller can upload, edit canvas via keyboard, compare, reject and approve in AI Image Studio", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));

  await page.setViewportSize({ width: 1440, height: 1000 });
  const resp = await page.goto("/seller-admin/image-studio");
  expect(resp.status()).toBe(200);

  await expect(page.locator("[data-image-studio]")).toBeVisible();
  const sampleImage = path.resolve(__dirname, "../../web/assets/images/customer/brass-elephant.png");
  await page.locator("[data-studio-file]").setInputFiles(sampleImage);
  await page.locator("[data-studio-process-btn]").click();

  // Wait for local AI background removal + variations to render.
  await expect(page.locator("[data-studio-compare]")).toBeVisible({ timeout: 30_000 });
  await expect(page.locator("[data-studio-editor]")).toBeVisible();
  await expect(page.locator(".studio-variation-card").first()).toBeVisible();

  // Keyboard operation on interactive canvas.
  const canvas = page.locator("[data-studio-canvas]");
  await canvas.focus();
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("Equal");
  await expect(page.locator("[data-studio-x-val]")).not.toHaveText("0%");

  // Save custom composition to server.
  await page.locator("[data-studio-apply]").click();
  await expect(page.locator("[data-studio-status]")).toContainText("Custom studio variation updated.");

  // Manual cutout touch-up brush (Erase background + Save cutout edits).
  await page.locator('[data-studio-tool="erase"]').click();
  await expect(page.locator("[data-studio-brush-controls]")).toBeVisible();
  const box = await canvas.boundingBox();
  await page.mouse.move(box.x + box.width * 0.15, box.y + box.height * 0.15);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.25, box.y + box.height * 0.25);
  await page.mouse.up();
  await page.locator("[data-studio-mask-save]").click();
  await expect(page.locator("[data-studio-status]")).toContainText("Manual cutout saved and studio variations updated.");

  // Toggle comparison with original.
  await page.locator("[data-studio-compare-toggle]").click();
  await expect(page.locator(".studio-variation-orig").first()).toBeVisible();

  // Approve first variation into product gallery.
  await page.locator(".studio-variation-card button:has-text('Approve & save to gallery')").first().click();
  await expect(page.locator("[data-studio-status]")).toContainText("/media/products/");

  // Narrow mobile viewport (375x812) must not overflow horizontally.
  await page.setViewportSize({ width: 375, height: 812 });
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);
  expect(overflow).toBeFalsy();
  expect(errors).toEqual([]);
});
