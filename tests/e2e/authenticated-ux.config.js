const { defineConfig } = require('@playwright/test');
const base = require('../../playwright.config');
module.exports = defineConfig({
  ...base, testDir: __dirname, testMatch: 'authenticated-ux.spec.js', timeout: 180000,
  use: { ...base.use, baseURL: 'http://127.0.0.1:18080', reducedMotion: 'reduce', navigationTimeout: 30000, actionTimeout: 15000, trace: 'off' },
  projects: ['chromium', 'firefox', 'webkit'].map(browserName => ({ name: browserName, use: { browserName } })),
});
