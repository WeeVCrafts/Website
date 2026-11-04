const base=require('./customer-media.config');
module.exports={...base,testMatch:'checkout-recovery.spec.js',timeout:360000,outputDir:require('node:path').join(require('node:os').tmpdir(),'weev-checkout-recovery')};
