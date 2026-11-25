const customer = require('./customer-layout.config');

module.exports = {
  ...customer,
  testMatch: ['storefront-performance.spec.js', 'storefront-responsive.spec.js'],
  grep: /hero controls|mobile homepage|responsive storefront/,
  use: { ...customer.use, baseURL: 'http://127.0.0.1:18086' },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
    { name: 'webkit', use: { browserName: 'webkit' }, grep: /responsive storefront/ },
  ],
};
