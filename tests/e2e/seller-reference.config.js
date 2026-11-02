const {defineConfig}=require('@playwright/test');
const base=require('./seller-responsive.config');
module.exports=defineConfig({...base,outputDir:require('node:path').join(require('node:os').tmpdir(),'weevcrafts-seller-reference'),testMatch:'seller-reference.spec.js',workers:1,use:{...base.use,baseURL:process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18086',reducedMotion:'reduce',navigationTimeout:45000}});
