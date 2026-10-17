const base = require('../../playwright.config');
module.exports = {
  ...base,
  // This read-only matrix visits nine seller routes at two viewport sizes.
  timeout: 180000,
  testDir: __dirname,
  testMatch: 'seller-workflows.spec.js',
  projects: [
    {name: 'Chromium', use: {...base.use, browserName: 'chromium'}},
    {name: 'Firefox', use: {...base.use, browserName: 'firefox'}},
    {name: 'WebKit', use: {...base.use, browserName: 'webkit'}},
  ],
};
