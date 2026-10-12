const base = require('../../playwright.config');
module.exports = { ...base, testDir: __dirname, testMatch: 'cover-order-regressions.spec.js', projects: ['chromium','firefox','webkit'].map(browserName => ({ name: browserName, use: { ...base.use, browserName } })) };
