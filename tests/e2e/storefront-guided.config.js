const base = require('../../playwright.config');
const path = require('node:path');
const os = require('node:os');
module.exports = {...base, testDir: __dirname, testMatch: 'storefront-guided.spec.js', timeout: 240000,
 outputDir: path.join(os.tmpdir(), 'weev-storefront-guided-results'),
 use: {...base.use, baseURL: process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18085', trace: 'off', navigationTimeout:45000, actionTimeout:15000},
 projects: ['chromium','firefox','webkit'].map(browserName=>({name:browserName,use:{browserName}}))};
