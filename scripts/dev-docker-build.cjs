// Air can restart its Windows shell before an earlier Compose child exits.
// Keep one Compose operation active, including while the next build is queued.
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {setTimeout: delay} = require('node:timers/promises');
const {randomUUID} = require('node:crypto');

process.chdir(path.resolve(__dirname, '..'));
const lock = path.resolve('tmp/air-docker/compose.lock');
fs.mkdirSync(path.dirname(lock), {recursive: true});
const token = randomUUID();
const alive = pid => {
  if (!Number.isSafeInteger(pid) || pid <= 0) return false;
  try { process.kill(pid, 0); return true; }
  catch (error) { return error.code === 'EPERM'; }
};

async function acquire() {
  let announced = false;
  for (;;) {
    try {
      const fd = fs.openSync(lock, 'wx');
      try { fs.writeFileSync(fd, JSON.stringify({token, owner: process.pid})); }
      finally { fs.closeSync(fd); }
      return;
    } catch (error) {
      if (error.code !== 'EEXIST') throw error;
      try {
        const lease = JSON.parse(fs.readFileSync(lock, 'utf8'));
        if (!alive(lease.owner) && !alive(lease.child)) {
          fs.unlinkSync(lock);
          continue;
        }
      } catch (readError) {
        if (readError.code !== 'ENOENT' && !(readError instanceof SyntaxError)) throw readError;
      }
      if (!announced) { console.log('Waiting for the previous Docker rebuild to finish.'); announced = true; }
      await delay(500);
    }
  }
}

async function main() {
  await acquire();
  try {
    const child = spawn('docker', ['compose', 'up', '-d', '--build', '--wait', 'app'], {stdio: 'inherit', windowsHide: true});
    if (child.pid) fs.writeFileSync(lock, JSON.stringify({token, owner: process.pid, child: child.pid}));
    process.exitCode = await new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('exit', code => resolve(code ?? 1));
    });
  } finally {
    try {
      if (JSON.parse(fs.readFileSync(lock, 'utf8')).token === token) fs.unlinkSync(lock);
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
