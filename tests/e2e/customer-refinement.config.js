const base = require('../../playwright.config');
const path = require('node:path');
const os = require('node:os');
module.exports = {
  ...base,
  testDir: __dirname,
  testMatch: 'customer-refinement.spec.js',
  outputDir: path.join(os.tmpdir(), 'weev-customer-refinement-results'),
  timeout: 300000,
  use: {...base.use, baseURL: process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18083', trace: 'off', actionTimeout: 15000, navigationTimeout: 45000},
  projects: ['chromium', 'firefox', 'webkit'].map(browserName => ({name: browserName, use: {browserName}})),
};
