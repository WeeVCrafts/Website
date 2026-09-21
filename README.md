# WeeVCrafts

WeeVCrafts is a handmade and organic marketplace. The production app is a single
Go service: it serves the storefront and role portals, exposes the `/api/v1`
JSON API, and uses PostgreSQL for marketplace data and Redis for sessions and
rate limits. Both the browser and the server-side pages use this application as
the only backend.

## Deploy on Railway

The root `Dockerfile` builds and runs the Go application. Add PostgreSQL,
Redis, and a Railway Storage Bucket to the same production environment. The
current Railway resources are named `Postgres`, `Redis`, and
`weevcrafts-media`. The app applies its embedded PostgreSQL migrations at
startup.

`.railway/railway.ts` keeps the app service linked to this repository, sets the
`/health/ready` health check, and wires the resource variables. Install the
Railway SDK with `npm install`, then review configuration with
`railway config plan` and apply planned changes with `railway config apply`.
The app service variables should resolve to:

```text
DATABASE_URL=${{Postgres.DATABASE_URL}}
REDIS_URL=${{Redis.REDIS_URL}}
S3_ADDR=${{weevcrafts-media.ENDPOINT}}
S3_ACCESS_KEY=${{weevcrafts-media.ACCESS_KEY_ID}}
S3_SECRET_KEY=${{weevcrafts-media.SECRET_ACCESS_KEY}}
S3_BUCKET=${{weevcrafts-media.BUCKET}}
S3_SECURE=true
WECRATFS_ENV=production
WECRATFS_SECURE_COOKIES=true
WECRATFS_LOCAL_MODE=false
WECRATFS_PAYMENT_MODE=razorpay
WECRATFS_PUBLIC_URL=https://${{RAILWAY_PUBLIC_DOMAIN}}
```

Generate a public domain for the app service in Railway Networking. The app
binds to Railway's `PORT`, marks session cookies secure in production, and
uses the public domain for account recovery links. Set `WECRATFS_PUBLIC_URL`
to your canonical `https://` domain if you add a custom domain. Product media
uploads and public reads are proxied through the app, so the bucket endpoint can
remain private to the Railway environment.

PostgreSQL and Redis are required. Meilisearch and S3-compatible object storage
are optional: search falls back to PostgreSQL, while product media uploads need
storage configured. Set `MEILI_ADDR` and `MEILI_API_KEY` for Meilisearch; set
`S3_ADDR`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, and `S3_SECURE` for
your S3-compatible storage. Configure SMTP with `WECRATFS_SMTP_ADDR`,
`WECRATFS_SMTP_USERNAME`, `WECRATFS_SMTP_PASSWORD`, and `WECRATFS_SMTP_FROM` to
enable email verification and password recovery. Set `RAZORPAY_KEY_ID`,
`RAZORPAY_KEY_SECRET`, and `RAZORPAY_WEBHOOK_SECRET`, then configure the
provider webhook to `https://<your-domain>/webhooks/razorpay` before accepting
live payments.

## Run locally

Requirements: Go 1.26+, Docker, and Docker Compose.

```sh
docker compose up -d --build
docker compose --profile demo run --rm seed-demo
```

Open <http://localhost:8080>. The local stack includes PostgreSQL, Redis,
Meilisearch, MinIO, and Mailpit. Demo accounts and local payment scenarios are
documented in [`docs/demo-accounts.md`](docs/demo-accounts.md).

Useful endpoints:

```sh
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:8080/health/metrics
curl http://localhost:8080/api/v1/products
curl 'http://localhost:8080/api/v1/search?q=soap'
```

The server-rendered storefront and account, seller, and admin workflows use
the same PostgreSQL-backed application services. Authentication uses secure
server-side sessions and CSRF tokens. Checkout creates pending orders; a
verified Razorpay webhook is the path that marks an order paid. Payments are
not collected in local mode.

To rebuild the disposable Meilisearch index from PostgreSQL catalog data:

```sh
go run ./cmd/rebuild-search
```

## Development

Templates and checked-in CSS are build artifacts. When their sources change:

```sh
make templ-generate
npm install
make css-build
```

The Go process embeds the generated templates, CSS, JavaScript, and images; it
does not start Node.js in production. `docs/architecture.md` describes the
application boundaries, `docs/api/openapi.yaml` records the JSON API, and
`docs/phase-status.md` tracks implementation status.

PostgreSQL query sources live in `internal/adapters/postgres/queries/` and
generated sqlc files live in `internal/adapters/postgres/generated/`. After
changing SQL queries or their schema inputs:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
sqlc generate -f sqlc.yaml
bash scripts/check-sqlc-generated.sh
```
