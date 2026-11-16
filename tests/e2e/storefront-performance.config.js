const customer = require('./customer-layout.config');

module.exports = {
  ...customer,
  testMatch: 'storefront-performance.spec.js',
  grep: /hero controls|mobile homepage/,
  use: { ...customer.use, baseURL: 'http://127.0.0.1:18086' },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
};
