const base = require('../../playwright.config');
const path = require('node:path');

module.exports = {
  ...base,
  testDir: __dirname,
  testMatch: ['customer-layout.spec.js', 'marketplace-recovery.spec.js'],
  projects: ['chromium', 'firefox', 'webkit'].map(browserName => ({ name: browserName, use: { browserName } })),
  webServer: {
    command: 'go run ./tests/e2e/fixtures/customer-layout',
    cwd: path.resolve(__dirname, '../..'),
    url: 'http://127.0.0.1:18086/cart',
    reuseExistingServer: false,
    timeout: 180000,
  },
};
