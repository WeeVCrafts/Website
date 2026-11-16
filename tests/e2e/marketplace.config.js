const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: '.', testMatch: 'marketplace-workflows.spec.js', workers: 1,
  timeout: 120000, expect: { timeout: 15000 },
  outputDir: '../../.tmp/marketplace-playwright',
  use: { browserName: 'chromium', trace: 'retain-on-failure' },
  reporter: [['list']],
});
