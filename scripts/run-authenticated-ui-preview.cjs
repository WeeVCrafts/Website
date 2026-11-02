// Source-built preview using only the existing, disposable role-workflow data.
// Never loads .env, seeds records, or connects to the main marketplace database.
const { execFileSync, spawn } = require('node:child_process');
const path = require('node:path');
const net = require('node:net');
process.chdir(path.resolve(__dirname, '..'));
const database = 'seller_workflows_20261017';
const container = 'weevcrafts-storefront-qa-db';
const binary = path.resolve('tmp/role-ux-preview.exe');

async function main() {
  await new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', () => reject(new Error('Port 18080 is occupied. Stop its existing audit preview before starting another.')));
    probe.listen(18080, '127.0.0.1', () => probe.close(resolve));
  });
  const details = JSON.parse(execFileSync('docker', ['inspect', container], { encoding: 'utf8', windowsHide: true }))[0];
  const value = key => details.Config.Env.find(item => item.startsWith(key + '='))?.slice(key.length + 1);
  if (value('POSTGRES_USER') !== 'storefrontqa' || details.HostConfig.PortBindings['5432/tcp']?.[0]?.HostPort !== '15432') throw new Error('Unexpected QA database configuration.');
  execFileSync('docker', ['start', container, 'weevcrafts-storefront-qa-redis'], { stdio: 'inherit', windowsHide: true });
  execFileSync('docker', ['exec', container, 'psql', '-U', 'storefrontqa', '-d', database, '-tAc', 'SELECT 1'], { stdio: 'ignore', windowsHide: true });
  execFileSync('go', ['build', '-o', binary, './cmd/website'], { stdio: 'inherit', windowsHide: true });
  const env = {
    ...process.env,
    DATABASE_URL: `postgres://storefrontqa:${encodeURIComponent(value('POSTGRES_PASSWORD'))}@127.0.0.1:15432/${database}?sslmode=disable`,
    DATABASE_CA_CERT: '', REDIS_URL: '', REDIS_ADDR: '127.0.0.1:16379',
    WECRATFS_ADDR: '127.0.0.1:18080', WECRATFS_PUBLIC_URL: 'http://127.0.0.1:18080',
    WECRATFS_ENV: 'development', WECRATFS_LOCAL_MODE: 'true', WECRATFS_PAYMENT_MODE: 'local',
    WECRATFS_SECURE_COOKIES: 'false', WECRATFS_STAGING: 'false',
    WECRATFS_SMTP_ADDR: '', WECRATFS_SMTP_FROM: '', WECRATFS_RESEND_API_KEY: '', WECRATFS_RESEND_API_KEY_FILE: '',
    RAZORPAY_KEY_ID: '', RAZORPAY_KEY_SECRET: '', RAZORPAY_WEBHOOK_SECRET: '', GOOGLE_APPLICATION_CREDENTIALS: '',
    MEILI_ADDR: '127.0.0.1:17770', S3_ADDR: '127.0.0.1:13900', S3_BUCKET: 'weevcrafts-ui-audit',
    S3_ACCESS_KEY: '', S3_SECRET_KEY: '', S3_SECURE: 'false',
  };
  const child = spawn(binary, [], { env, stdio: 'inherit', windowsHide: true });
  child.on('exit', code => { process.exitCode = code ?? 0; });
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
