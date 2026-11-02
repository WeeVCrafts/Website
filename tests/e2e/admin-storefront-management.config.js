const base=require('../../playwright.config');
const path=require('node:path');
const os=require('node:os');
module.exports={...base,testDir:__dirname,testMatch:'admin-storefront-management.spec.js',timeout:120000,
 outputDir:path.join(os.tmpdir(),'weev-admin-storefront-results'),
 use:{...base.use,baseURL:process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18085',trace:'off'},
 projects:['chromium','firefox','webkit'].map(browserName=>({name:browserName,use:{browserName}}))};
