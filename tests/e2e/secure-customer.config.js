const base = require('../../playwright.config');
module.exports = {
  ...base,
  testDir: __dirname,
  testMatch: ['secure-customer.spec.js'],
  timeout: 120000,
  // Authentication requests contain credentials/tokens; keep them out of evidence.
  use: { ...base.use, trace: 'off' },
  projects: ['chromium', 'firefox', 'webkit'].map(browserName => ({ name: browserName, use: { browserName } })),
};
