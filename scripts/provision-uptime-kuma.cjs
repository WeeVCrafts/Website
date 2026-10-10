// Run inside the pinned Uptime Kuma image, which supplies socket.io-client.
// Use Kuma's authenticated Socket.IO handlers; never edit its database directly.
const { io } = require('socket.io-client');
const { randomBytes } = require('node:crypto');
const fs = require('node:fs');

const credentialsPath = '/app/data/weevcrafts-local-access.json';
const desiredMonitors = [
  { name: 'WeeVCrafts storefront', type: 'http', url: 'http://app:8080/' },
  { name: 'WeeVCrafts readiness', type: 'http', url: 'http://app:8080/health/ready' },
  { name: 'WeeVCrafts liveness', type: 'http', url: 'http://app:8080/health/live' },
  { name: 'PostgreSQL connection', type: 'port', hostname: 'postgres', port: 5432 },
  { name: 'Redis connection', type: 'port', hostname: 'redis', port: 6379 },
  { name: 'Meilisearch health', type: 'http', url: 'http://meilisearch:7700/health' },
  { name: 'Garage readiness', type: 'http', url: 'http://garage:3903/health' },
  { name: 'Mailpit inbox', type: 'http', url: 'http://mailpit:8025/' },
  { name: 'Adminer login', type: 'http', url: 'http://adminer:8080/' },
];

const socket = io(process.env.UPTIME_KUMA_URL || 'http://uptime-kuma:3001', {
  transports: ['websocket'], reconnection: false, timeout: 15000,
});

function request(event, ...args) {
  return new Promise((resolve, reject) => {
    socket.timeout(20000).emit(event, ...args, (error, result) => {
      if (error) return reject(new Error(`${event} timed out`));
      if (result && result.ok === false) return reject(new Error(`${event}: ${result.msg}`));
      resolve(result);
    });
  });
}

async function main() {
  await new Promise((resolve, reject) => {
    socket.once('connect', resolve);
    socket.once('connect_error', reject);
  });
  const needsSetup = await request('needSetup');
  let credentials;
  if (process.env.UPTIME_KUMA_PASSWORD) {
    credentials = {
      username: process.env.UPTIME_KUMA_USERNAME || 'weevcrafts-local',
      password: process.env.UPTIME_KUMA_PASSWORD,
    };
  } else if (fs.existsSync(credentialsPath)) {
    credentials = JSON.parse(fs.readFileSync(credentialsPath, 'utf8'));
  } else if (needsSetup) {
    credentials = {
      username: process.env.UPTIME_KUMA_USERNAME || 'weevcrafts-local',
      password: `Weev!${randomBytes(24).toString('base64url')}9a`,
    };
    fs.writeFileSync(credentialsPath, JSON.stringify(credentials, null, 2) + '\n', {
      mode: 0o600, flag: 'wx',
    });
  } else {
    throw new Error('Existing Kuma account found. Set UPTIME_KUMA_USERNAME and UPTIME_KUMA_PASSWORD in .env; existing accounts are never reset.');
  }
  if (needsSetup) {
    await request('setup', credentials.username, credentials.password);
    console.log('Created local Kuma administrator. Generated credentials are kept in the persistent data volume; supplied .env credentials remain in .env.');
  }

  let monitorList = {};
  socket.on('monitorList', list => { monitorList = list; });
  await request('login', credentials);
  await request('getMonitorList');
  for (const desired of desiredMonitors) {
    // Retain any user edits, notification selections and existing history.
    if (Object.values(monitorList).some(monitor => monitor.name === desired.name)) {
      console.log(`Retained: ${desired.name}`);
      continue;
    }
    await request('add', {
      ...desired,
      interval: 60, retryInterval: 60, maxretries: 1, timeout: 10,
      method: 'GET', maxredirects: 5, accepted_statuscodes: ['200-299'],
      active: true, notificationIDList: {}, conditions: [],
      ignoreTls: false, upsideDown: false,
    });
    console.log(`Added: ${desired.name}`);
  }
  console.log('Local monitoring configured. Notification delivery channels can be selected in Kuma settings.');
}

main().catch(error => {
  console.error(error.message);
  process.exitCode = 1;
}).finally(() => socket.close());
