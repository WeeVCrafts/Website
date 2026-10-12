# Weevcrafts

Weevcrafts is a handmade and organic marketplace. The production app is a single
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
`railway config plan`. Review the exact plan before explicitly applying it with
`railway config apply`.
The app service variables should resolve to:

```text
DATABASE_URL=${{Postgres.DATABASE_URL}}?sslmode=verify-full
DATABASE_CA_CERT=<public PEM certificate from the managed Postgres root CA>
REDIS_URL=${{Redis.REDIS_URL}}
S3_ADDR=${{weevcrafts-media.ENDPOINT}}
S3_ACCESS_KEY=${{weevcrafts-media.ACCESS_KEY_ID}}
S3_SECRET_KEY=${{weevcrafts-media.SECRET_ACCESS_KEY}}
S3_BUCKET=${{weevcrafts-media.BUCKET}}
S3_SECURE=true
WECRATFS_ENV=production
WECRATFS_SECURE_COOKIES=true
WECRATFS_LOCAL_MODE=false
WECRATFS_PAYMENT_MODE=local
WECRATFS_LOCAL_PAYMENT_SECRET=<unique secret stored in Railway>
WECRATFS_PUBLIC_URL=https://${{RAILWAY_PUBLIC_DOMAIN}}
```

The current Railway environment uses simulated payments. Set payment mode to
`razorpay` only after configuring and verifying the live provider. Keep secrets
in Railway variables, never in this repository.
The configured demo provider exposes a capture-only checkout action at
`/api/v1/payments/demo-capture/{order-number}` with session, CSRF and order
ownership checks. It charges no money; failure/pending development helpers
remain disabled in production.

For Railway's Postgres SSL template, export the public certificate at
`/var/lib/postgresql/data/certs/root.crt` from the Postgres service and set
`DATABASE_CA_CERT` through `railway variable set DATABASE_CA_CERT --stdin`.
Use the private database hostname covered by that certificate; production
requires `sslmode=verify-full`. The app trusts this CA without disabling
certificate or hostname verification. Refresh the variable if the CA rotates.
The Dockerfile cache IDs are scoped to the existing Website service; update the
IDs if moving the app to a different Railway service.

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

## Run locally (1-Click)

### On RHEL 9 / 10 / Linux (Podman or Docker):
On RHEL, Podman is the default container engine. Install requirements if needed:
```sh
sudo dnf install -y podman podman-compose
```
Then run the 1-click script:
```sh
chmod +x start.sh stop.sh
./start.sh
```

### On Windows:
Double-click `start.bat` (or run `./start.sh` inside WSL / Git Bash).

### Or manually with Docker / Podman:
```sh
docker compose up -d --build
docker compose --profile clean run --build --rm seed-clean
```

Open <http://localhost:8080>. The local stack includes PostgreSQL, Redis,
Meilisearch, Garage S3 storage, Mailpit, Adminer, and Uptime Kuma. Demo accounts and local payment scenarios are
documented in [`docs/demo-accounts.md`](docs/demo-accounts.md).

Local tools:

| Tool | URL | Use |
| --- | --- | --- |
| Mailpit | <http://localhost:8025> | Read captured verification, recovery, and order emails. |
| Adminer | <http://localhost:8082/?pgsql=postgres&username=wecratfs&db=wecratfs> | Inspect the local PostgreSQL database. Password: `wecratfs`. |
| Uptime Kuma | <http://localhost:3001> | View app and dependency availability and response times. |

All published development services bind to `127.0.0.1`. Adminer and Kuma are local operator tools. They have
separate authentication from marketplace roles. They are not deployed by the
production app Dockerfile. See [`docs/local-tools.md`](docs/local-tools.md)
for monitor setup, credentials, persistence, and troubleshooting.

Product covers and gallery uploads use the configured S3-compatible media
bucket (Garage locally), with stable `/media/products/{id}` references. Uploads
are verified before publication and survive application rebuilds/deployments.
The product editor accepts PNG, JPEG and WebP covers up to 25 MB and reports
storage failures instead of substituting a stock image. Back up the bucket
together with PostgreSQL. Before upgrading a legacy container, preserve any
remaining `/app/web/assets/images/uploads` covers and re-upload them through
the product editor; an ephemeral container filesystem cannot retain these files.
The local `product_uploads` volume still holds legacy uploads and maker portraits.

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

The [secure customer update](docs/secure-customer-update.md) describes wishlist,
private profile photos, role replacement, social settings and verification.
Review [deployment security requirements](docs/deployment-security.md) before
production; the local Compose stack uses development credentials.

The [Firebase customer integration](docs/firebase-integration.md) documents
Google/email sign-in, account linking, web push, optional performance diagnostics,
local credential provisioning and the current email-template restriction.
The [Resend integration](docs/resend-integration.md) documents application-owned
branded verification/reset emails, the verified sender domain and private keys.

PostgreSQL query sources live in `internal/adapters/postgres/queries/` and
generated sqlc files live in `internal/adapters/postgres/generated/`. After
changing SQL queries or their schema inputs:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
sqlc generate -f sqlc.yaml
bash scripts/check-sqlc-generated.sh
```

### Clean local accounts

Normal startup bootstraps an empty database with three accounts and no products. Existing accounts are preserved on later starts. See [the clean-start guide](docs/local-clean-start.md) for credentials and the explicit local reset. The larger `seed-demo` profile is opt-in and adds demo data.

See [the customer workflow update](docs/customer-workflow-update.md) for checkout corrections, account security, mock payments, delivery progress, notification setup and isolated regression checks.

## Current account and operations model

WeeVCrafts uses Customer, Seller, Admin and Super Admin, with one role per account. See [four-role operations](docs/four-role-workflow.md) for permissions, private seller confirmations, migration and rollout. Dated audit reports describe historical builds.
