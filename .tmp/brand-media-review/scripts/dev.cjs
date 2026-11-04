// Local development only. Starts Air; never seeds or resets a database.
const fs=require('node:fs');
const path=require('node:path');
const {spawn,execFileSync}=require('node:child_process');
process.chdir(path.resolve(__dirname,'..'));
const dockerPreview=process.argv.includes('--docker');
if (fs.existsSync('.env')) process.loadEnvFile('.env');
// Supply documented local defaults only for variables not already configured.
process.loadEnvFile('.env.example');
if (!dockerPreview) process.env.WECRATFS_ADDR='127.0.0.1:8080';
if (!process.env.WECRATFS_PUBLIC_URL || process.env.WECRATFS_PUBLIC_URL==='http://localhost:8080') process.env.WECRATFS_PUBLIC_URL='http://localhost:8090';
const goPath=execFileSync('go',['env','GOPATH'],{encoding:'utf8',windowsHide:true}).trim().split(path.delimiter)[0];
const air=path.join(goPath,'bin',process.platform==='win32'?'air.exe':'air');
if (!fs.existsSync(air)) { console.error('Install Air first: npm run dev:setup'); process.exit(1); }
if (dockerPreview) {
  // Validate required volumes/config before starting a long-lived watch process.
  try { execFileSync('docker',['compose','config','--quiet'],{stdio:'inherit',windowsHide:true}); }
  catch { console.error('Docker preview configuration is not ready.'); process.exit(1); }
}
console.log('Live reload: http://localhost:8090 (Go app: 8080). Existing catalog is preserved.');
const child=spawn(air,['-c',dockerPreview?'.air.docker.toml':'.air.toml'],{stdio:'inherit',windowsHide:true,env:process.env});
child.on('error',error=>{console.error(error.message);process.exitCode=1;});
child.on('exit',code=>{process.exitCode=code??0;});
for(const signal of ['SIGINT','SIGTERM']) process.on(signal,()=>child.kill(signal));
