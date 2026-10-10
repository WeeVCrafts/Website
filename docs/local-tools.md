# Local database and uptime tools

Adminer and Uptime Kuma start with the normal `docker compose up -d` or
one-click scripts. Images are pinned to Adminer 6.1.1 and Uptime Kuma 2.5.5-slim.
The official slim image supports these HTTP/TCP checks without bundling a browser.
Both web interfaces bind only to `127.0.0.1`. The production Go image does
not include or deploy these tools.

## Adminer

Open <http://localhost:8082/?pgsql=postgres&username=wecratfs&db=wecratfs>.
The query prefills the PostgreSQL server and username; enter the password.

| Login field | Value |
| --- | --- |
| System | PostgreSQL |
| Server | `postgres` |
| Username | `wecratfs` |
| Password | `wecratfs` |
| Database | `wecratfs` |

Inspect `users`, product, order, and seller tables to verify saved records.
The connection uses the existing local PostgreSQL account, which can also
write data. Changes made in Adminer bypass application validation; use the
application UI for ordinary customer and seller workflows.

## Uptime Kuma

Open <http://localhost:3001>. The one-shot `uptime-kuma-setup` service
creates an administrator on a fresh installation and adds these monitors:

| Monitor | Target inside Docker | What it establishes |
| --- | --- | --- |
| WeeVCrafts storefront | `http://app:8080/` | Home page responds with HTTP 2xx. |
| WeeVCrafts readiness | `http://app:8080/health/ready` | App responds ready, including PostgreSQL/Redis probes. |
| WeeVCrafts liveness | `http://app:8080/health/live` | App process responds with HTTP 2xx. |
| PostgreSQL connection | `postgres:5432` | TCP connections are accepted. |
| Redis connection | `redis:6379` | TCP connections are accepted. |
| Meilisearch health | `http://meilisearch:7700/health` | Search health responds with HTTP 2xx. |
| MinIO readiness | `http://minio:9000/minio/health/ready` | Object storage readiness responds with HTTP 2xx. |
| Mailpit inbox | `http://mailpit:8025/` | Local email inbox web interface responds. |
| Adminer login | `http://adminer:8080/` | Database tool web interface responds. |

Checks run every 60 seconds, use a 10-second timeout, and retry once before
declaring failure. The TCP monitors do not prove query correctness; the
app readiness probe provides the stronger database/session checks.
Monitors use Docker service names because `localhost` inside Kuma refers
to Kuma itself. No Docker socket is mounted.

### Credentials

With the default empty `UPTIME_KUMA_PASSWORD`, setup generates a unique
password and stores it in `/app/data/weevcrafts-local-access.json` with
owner-only permissions in the persistent `uptime_kuma_data` volume.
The default username is `weevcrafts-local`. To retrieve the credential:

```sh
docker compose exec -T uptime-kuma cat /app/data/weevcrafts-local-access.json
```

To export it to a Git-ignored local file without printing it:

```sh
docker compose cp uptime-kuma:/app/data/weevcrafts-local-access.json .env.kuma.local
```

Alternatively set `UPTIME_KUMA_USERNAME` and `UPTIME_KUMA_PASSWORD` in `.env`
before the first startup. For an installation already owned by another account,
provide that account's credentials. Setup never resets existing accounts.
If you later change the password in Kuma, update `.env` with the new credential
before rerunning setup. Protect exported credentials as local secrets.

### Re-run and persistence

Setup uses Kuma's authenticated Socket.IO handlers from the pinned image,
not direct database writes. It adds only missing named monitors and retains
existing monitor edits, selected notification channels, and history:

```sh
docker compose run --rm uptime-kuma-setup
docker compose logs uptime-kuma-setup
```

Kuma uses its own SQLite database for local monitoring; it does not add tables
to the marketplace database. Accounts, monitors, and heartbeat history live in `uptime_kuma_data`.
Normal `docker compose down` preserves this volume. Include it in backups.
Adminer is stateless and uses the existing PostgreSQL volume.

### Notifications and monitoring limits

No external notification channel is configured automatically. Add a channel
in Kuma and select it for the relevant monitors when alerts are desired.
In-app dashboard status and heartbeat history are available immediately.
Monitoring on the same Docker host cannot detect that host's complete outage
from outside; deployment-level monitoring should run on a separate host.

### Troubleshooting

```sh
docker compose ps -a
docker compose logs --tail=60 adminer uptime-kuma uptime-kuma-setup
docker compose up -d adminer uptime-kuma uptime-kuma-setup
```

If ports 8082 or 3001 are occupied, adjust only their host-side mappings in
`compose.yaml`. Keep Adminer's server as `postgres`, not `localhost`.
If provisioning fails, examine the setup logs and supply the current Kuma
credentials; keep the data volume rather than deleting it to reset login.

Upstream references: [Adminer image](https://hub.docker.com/_/adminer),
[Uptime Kuma installation](https://github.com/louislam/uptime-kuma), and
[pinned Socket.IO handlers](https://github.com/louislam/uptime-kuma/blob/2.5.5/server/server.js).
