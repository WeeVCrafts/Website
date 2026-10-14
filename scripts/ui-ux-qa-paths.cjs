const path = require('node:path');
const os = require('node:os');
module.exports = { output: process.env.WEEVCRAFTS_UI_ARTIFACTS || path.join(os.tmpdir(),'weevcrafts-ui-ux-20261014') };
