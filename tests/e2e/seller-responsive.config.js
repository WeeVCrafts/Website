const { defineConfig } = require('@playwright/test');
const base = require('../../playwright.config');

module.exports = defineConfig({
  ...base,
  testDir: __dirname,
  testMatch: 'seller-responsive.spec.js',
  timeout: 180000,
  expect: { timeout: 30000 },
  use: { ...base.use, actionTimeout: 15000, navigationTimeout: 20000, baseURL: process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18080' },
  projects: ['chromium', 'firefox', 'webkit'].map(browserName => ({ name: browserName, use: { browserName } })),
});
