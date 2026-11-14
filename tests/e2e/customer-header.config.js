const base=require('../../playwright.config');

module.exports={
  ...base,
  testDir:__dirname,
  testMatch:'customer-header.spec.js',
  use:{...base.use,baseURL:process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18080'},
  projects:['chromium','firefox','webkit'].map(browserName=>({name:browserName,use:{browserName}})),
};
