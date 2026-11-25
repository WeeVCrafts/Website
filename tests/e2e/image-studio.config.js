const { defineConfig } = require("@playwright/test");
const base = require("../../playwright.config");

module.exports = defineConfig({
  ...base,
  testDir: __dirname,
  testMatch: "image-studio.spec.js",
  use: {
    ...base.use,
    baseURL: process.env.WEEVCRAFTS_BASE_URL || "http://127.0.0.1:18190",
    ...(process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {}),
  },
});
