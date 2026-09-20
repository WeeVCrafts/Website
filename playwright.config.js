const { defineConfig } = require("@playwright/test");
const os = require("node:os");
const path = require("node:path");

module.exports = defineConfig({
  testDir: "./tests/e2e",
  outputDir: path.join(os.tmpdir(), "weevcrafts-playwright"),
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list"]],
  use: {
    baseURL: process.env.WEEVCRAFTS_BASE_URL || "http://127.0.0.1:8080",
    browserName: "chromium",
    headless: true,
    viewport: { width: 1440, height: 1000 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
