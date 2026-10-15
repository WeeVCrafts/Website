// Air invokes this before restarting the embedded-asset Go server.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const {execFileSync} = require('node:child_process');
process.chdir(path.resolve(__dirname, '..'));
fs.mkdirSync('tmp/air', {recursive:true});
function run(command, args) { execFileSync(command, args, {stdio:'inherit', windowsHide:true}); }
function imageHash() {
  const hash=crypto.createHash('sha256');
  for (const name of fs.readdirSync('web/assets/images/customer').filter(name=>name.endsWith('.webp')).sort()) hash.update(name).update(fs.readFileSync(path.join('web/assets/images/customer',name)));
  return hash.digest('hex');
}
const stateFile='tmp/air/images.sha256';
const current=imageHash();
if (!fs.existsSync(stateFile) || fs.readFileSync(stateFile,'utf8')!==current) {
  run(process.execPath,['scripts/build-image-variants.cjs']);
  fs.writeFileSync(stateFile,current);
}
run('templ',['generate','-path','.','-w','2']);
run(process.execPath,['node_modules/tailwindcss/lib/cli.js','-i','web/assets/css/source.css','-o','internal/transport/web/static/app.css','--minify']);
run(process.execPath,['scripts/minify-preview-css.js']);
run(process.execPath,['scripts/minify-ui-js.cjs']);
run('go',['build','-o',process.platform==='win32'?'tmp/air/website.exe':'tmp/air/website','./cmd/website']);
