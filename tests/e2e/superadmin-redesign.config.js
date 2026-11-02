const {defineConfig}=require('@playwright/test');
const base=require('../../playwright.config');
module.exports=defineConfig({...base,outputDir:require('node:path').join(require('node:os').tmpdir(),'weevcrafts-superadmin-redesign'),testDir:__dirname,testMatch:'superadmin-redesign.spec.js',timeout:300000,use:{...base.use,baseURL:'http://127.0.0.1:18080',reducedMotion:'reduce',trace:'retain-on-failure'}});
