const base = require('../../playwright.config');
const path = require('node:path');
module.exports = {
  ...base,
  testDir: __dirname,
  testMatch: 'finance-controls.spec.js',
  webServer: {
    command: 'go run ./tests/e2e/fixtures/finance-controls',
    cwd: path.resolve(__dirname, '../..'),
    url: 'http://127.0.0.1:18087/admin/finance',
    reuseExistingServer: false,
    timeout: 180000,
  },
};
