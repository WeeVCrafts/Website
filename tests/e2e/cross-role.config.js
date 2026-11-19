const base=require('../../playwright.config');
module.exports={...base,testDir:__dirname,testMatch:['cross-role.spec.js','marketplace-workflows.spec.js'],timeout:90000,expect:{timeout:15000}};
