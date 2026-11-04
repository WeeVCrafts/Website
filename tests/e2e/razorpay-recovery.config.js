const base=require('./customer-media.config');
module.exports={...base,testMatch:'razorpay-testmode.spec.js',timeout:240000,outputDir:require('node:path').join(require('node:os').tmpdir(),'weev-razorpay-recovery'),use:{...base.use,baseURL:'http://127.0.0.1:18089',trace:'off'},projects:[{name:'chromium',use:{browserName:'chromium'}}]};
