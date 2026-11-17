const base = require('./customer-layout.config');

module.exports = {
  ...base,
  testMatch: ['product-delivery.spec.js'],
  use: { ...base.use, baseURL: 'http://127.0.0.1:18086' },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
};
