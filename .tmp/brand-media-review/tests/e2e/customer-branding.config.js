const base = require('./customer-media.config');
module.exports = {...base, testMatch: 'customer-branding.spec.js', outputDir: require('node:path').join(require('node:os').tmpdir(), 'weev-customer-branding')};
