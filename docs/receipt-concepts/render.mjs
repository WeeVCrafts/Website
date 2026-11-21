// Renders each A4 receipt concept sheet to PNG and JPG for design review.
// Usage: node docs/receipt-concepts/render.mjs
import { chromium } from "playwright";
import path from "node:path";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const source = path.join(here, "mockups.html");
const imagesDir = path.join(root, "docs/receipt-images");
const docs = [
  ["order-receipt", "01-order-receipt"],
  ["tax-invoice", "02-seller-tax-invoice"],
  ["credit-note", "03-credit-note"],
  ["packing-slip", "04-packing-slip"],
  ["return-authorization", "05-return-authorization"],
];

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 900, height: 1200 }, deviceScaleFactor: 2 });
await page.goto("file:///" + source.replace(/\\/g, "/"));
await page.waitForTimeout(400);
await page.evaluate(() => document.fonts.ready);

let overflow = false;
for (const [id, name] of docs) {
  const sheet = page.locator(`#${id}`);
  const box = await sheet.evaluate((el) => ({
    client: el.clientHeight,
    scroll: el.scrollHeight,
  }));
  const content = await sheet.evaluate(
    (el) => [...el.children].reduce((sum, child) => sum + child.getBoundingClientRect().height, 0),
  );
  if (content > box.client) {
    overflow = true;
    console.log(`OVERFLOW ${name}: content ${Math.round(content)}px > ${box.client}px`);
  }
  await sheet.screenshot({ path: path.join(here, `${name}.png`) });
  await sheet.screenshot({ path: path.join(imagesDir, `${name}.jpg`), quality: 95, type: "jpeg" });
}
await browser.close();
console.log(overflow ? "Rendered with overflow warnings." : "Rendered 5 sheets to PNG and JPG, no overflow.");
