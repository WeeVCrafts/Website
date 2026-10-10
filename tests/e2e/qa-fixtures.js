const { execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');

// Independent cases share a Docker gateway IP. Reset only that QA client's
// attempt counters at the fixture boundary, leaving production limits and
// all session/business records intact. Rate-limit scenarios must not call
// this helper between attempts within a case.
function resetAuthenticationQuota() {
  const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
  if (process.env.WEEVCRAFTS_QA_DATABASE !== 'weev_qa_20261009_browser' || target.hostname !== '127.0.0.1' || target.port !== '18080') throw new Error('Authentication quota fixtures require the isolated QA deployment.');
  const gateway = execFileSync('docker', ['network', 'inspect', 'weevcrafts_default', '--format', '{{(index .IPAM.Config 0).Gateway}}'], { encoding: 'utf8' }).trim();
  for (const bucket of ['login', 'password-recovery']) {
    const digest = createHash('sha256').update(`auth:${gateway}:${bucket}`).digest('hex');
    const prefix = `wecratfs:rate:${digest}:`;
    const keys = execFileSync('docker', ['exec', 'weev-qa-20261009-redis', 'redis-cli', '--scan', '--pattern', prefix + '*'], { encoding: 'utf8' }).trim().split(/\r?\n/).filter(Boolean);
    if (keys.some(key => !key.startsWith(prefix))) throw new Error('Unexpected Redis quota key');
    if (keys.length) execFileSync('docker', ['exec', 'weev-qa-20261009-redis', 'redis-cli', 'DEL', ...keys], { stdio: 'ignore' });
  }
}
module.exports = { resetAuthenticationQuota };
