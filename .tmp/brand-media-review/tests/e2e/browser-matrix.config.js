const base = require('../../playwright.config');

module.exports = {
  ...base,
  // Five viewport visits and screenshots of growing QA audit tables can take
  // longer than the ordinary single-page 60-second test budget in WebKit.
  timeout: 180000,
  testDir: __dirname,
  testMatch: ['role-portals.spec.js', 'seller-support-live.spec.js', 'integration-gaps.spec.js'],
  projects: [
    { name: 'Firefox', use: { ...base.use, browserName: 'firefox' } },
    { name: 'WebKit', use: { ...base.use, browserName: 'webkit' } },
  ],
};
