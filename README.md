# WeevCrafts

## Local development with Air

Run `npm install`, ensure Go and `templ` are on PATH, then run `npm run dev:setup`
once to install pinned Air v1.67.4. Start the existing local PostgreSQL/Redis
services and run `npm run dev`. Open **http://localhost:8090** for automatic
browser refresh. Port 8080 serves the underlying application directly.
This command does not seed or reset data.

The launcher reads an optional `.env`, retains existing environment settings,
and supplies missing local defaults from `.env.example`. It sets the local app
address to port 8080 and defaults its public URL to the Air proxy on 8090.
Keep a custom public URL in `.env` if needed. Stop with Ctrl+C.

Air watches Go, templ, styles, scripts and source images. Before each restart it
regenerates templ, builds Tailwind, and minifies CSS/JavaScript so embedded assets
match source. Responsive photographs rebuild when their sources change. Generated
outputs and temporary binaries are excluded to avoid restart loops.
See [.air.toml](.air.toml) and the [official Air documentation](https://github.com/air-verse/air).

WeevCrafts is a handmade and organic marketplace. The production app is a single
Go service: it serves the storefront and role portals, exposes the `/api/v1`
JSON API, and uses PostgreSQL for marketplace data and Redis for sessions and
rate limits. Both the browser and the server-side pages use this application as
the only backend.

## Deploy on Hostinger VPS with Coolify

The production target is Hostinger VPS managed through Coolify. Build the root
`Dockerfile` from the reviewed Git revision. One unprivileged Go service serves
the storefront, role portals, API and media; Node.js is used only during build.
Expose application port 8080 through Coolify's HTTPS proxy. Standard `PORT` and
`WECRATFS_ADDR` remain supported. Configure `/health/ready` as the health check.
The application applies embedded PostgreSQL migrations during startup.

Use separate production PostgreSQL, Redis and private S3-compatible media storage.
[`deploy/coolify-dependencies.yml`](deploy/coolify-dependencies.yml) supplies the
dependency definitions; review private networking, TLS certificates, credentials,
persistence and backups before applying it. Local Compose is development-only.
Set these variables privately in Coolify; never commit production values:

```text
WECRATFS_ENV=production
WECRATFS_PUBLIC_CONTENT_APPROVED=false
WECRATFS_CHECKOUT_ENABLED=false
WECRATFS_PUBLIC_URL=https://weevcrafts.com
WECRATFS_SECURE_COOKIES=true
WECRATFS_LOCAL_MODE=false
WECRATFS_NO_PAYMENT_DEMO=false
WECRATFS_PAYMENT_MODE=razorpay
DATABASE_URL=postgres://<user>:<password>@<private-host>/<database>?sslmode=verify-full
DATABASE_CA_CERT=<trusted public PEM CA if needed>
REDIS_URL=redis://:<password>@<private-host>:6379
S3_ADDR=<private TLS storage endpoint>
S3_ACCESS_KEY=<dedicated application key>
S3_SECRET_KEY=<dedicated application secret>
S3_BUCKET=<private media bucket>
S3_SECURE=true
S3_CA_CERT=<trusted public PEM CA if needed>
RAZORPAY_KEY_ID=<approved provider key>
RAZORPAY_KEY_SECRET=<private provider secret>
RAZORPAY_WEBHOOK_SECRET=<private webhook secret>
```

Use live Razorpay credentials for real-money trading and a verified webhook at
`https://weevcrafts.com/webhooks/razorpay`. Missing payment credentials leave
checkout unavailable; no simulated success is substituted. Isolated staging uses
separate data, storage, Redis, identity and Razorpay test credentials. Never send
staging mail or notifications to production users.

Configure SMTP or Resend for application-owned verification/reset and transaction
emails; see [mail integration](docs/resend-integration.md). Firebase sign-in and
Meilisearch are optional. PostgreSQL remains the catalog/search authority.
Media uploads require configured durable storage; back it up with PostgreSQL.

Before deploying, verify the exact image revision, migrations, production
configuration, restore evidence and release gates. See
[deployment security](docs/deployment-security.md) and the
[production readiness audit](docs/production-readiness-audit-2026-11-04.md).
A local build or green CI does not establish that the deployed environment is ready.

Production defaults to a publication hold. Information/policy pages and new
checkout return a truthful unavailable response while
`WECRATFS_PUBLIC_CONTENT_APPROVED=false`. Existing orders, provider callbacks,
login and account recovery remain available. Install the owner's approved
policies, contact details, support hours and permitted content, review the exact
release, and retain approval evidence before explicitly enabling this variable.
The variable records an operator decision; it does not approve content itself.
Approved information can be published independently of new checkout:
`WECRATFS_CHECKOUT_ENABLED` defaults to false in every environment. While false,
new checkout pages and APIs return HTTP 503 before order/reservation handlers run.
Existing orders, payment recovery and signed provider callbacks remain available.
Enabling checkout in production requires approved content and nonempty Razorpay
order and webhook credentials; live/test key safeguards continue to apply.

Production `/health/metrics` requires the dedicated bearer token configured as
`WECRATFS_METRICS_TOKEN` (at least 32 characters). It preserves existing search
counters and adds `http` and `database_pool` resource counters. HTTP groups use
a fixed vocabulary, not raw URLs, queries, customer IDs, headers or SQL. Histogram
buckets are cumulative milliseconds; counters reset on application restart and
exclude health probes/metrics requests. Pool counters include workers and probes;
pool acquisitions are not SQL-statement counts or committed transactions.
These are operational observations, not
visitor, MAU, billing or persistent audit records. Responses require no-store.

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
```

Open <http://localhost:8080>. The default stack includes the real application,
PostgreSQL, Redis, Meilisearch and Garage storage, using persistent named volumes.
It neither inserts demo records nor provisions local accounts. Existing accounts,
products and orders are retained. Optional operator tools use the `tools` profile:
`docker compose --profile tools up -d`. Mailpit is an optional developer inbox and
is not used as the application's transactional sender by default.

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

See [Razorpay test payments](docs/razorpay-integration.md) for sandbox settings,
webhook setup and the opt-in provider browser test. The
[28 October indexing audit](docs/seo-audit-2026-10-28.md) records Search Console
findings, implemented SEO fixes and the remaining deployment/recrawl steps.

To rebuild the disposable Meilisearch index from PostgreSQL catalog data:

```sh
go run ./cmd/rebuild-search
```

## Development

Use Go 1.26.10 or newer within the supported Go 1.26 series. The module, CI and
container build require this security patch level.

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

### Local accounts and isolated fixtures

Normal startup preserves existing data and accounts. It does not seed products,
create demo accounts or reset the database. See [local startup](docs/local-clean-start.md).
Demo seeding and account provisioning are explicit operator tools for isolated
local test environments. Never run them against production business data.

## Current account and operations model

See [homepage and category management](docs/catalog-management.md) for saved
category photographs/order, product cross-listing, the four homepage product
sections and the opt-in `cmd/import-catalog` workflow. Its default is read-only
validation; actual merchant inventory and photographs are required for listings.

WeevCrafts uses Customer, Seller, Admin and Super Admin, with one role per account. See [four-role operations](docs/four-role-workflow.md) for permissions, private seller confirmations, migration and rollout. Dated audit reports describe historical builds.

## Docker preview with Air

Run `npm run dev:docker` to keep the real Docker application running behind Air's
browser-refresh proxy at **http://localhost:8090**. Source changes rebuild the
application container and refresh the connected browser after Compose reports it
healthy. `npm run dev` remains available for a native Go development process; stop
the Compose app before using that alternative because both use port 8080.

The Dockerfile regenerates responsive images, Tailwind/minified assets and templ
outputs during the image build. It includes the supplied PNG logo. The final image
runs the compiled Go application as an unprivileged user without Node or Air.

Local HTTP viewing uses development transport settings with demo switches disabled.
An internet production deployment still requires HTTPS, secure cookies, PostgreSQL
TLS with dedicated credentials, TLS object storage and configured payment providers;
the server's production validation remains enabled. No remote deployment is implied
by a successful local container build.

## Category publication and storefront content

The general Storefront editor is retired. Super Admin manages categories through
the current Category Management workspace; category saves publish immediately.
Review content and permitted photographs before saving. See
[category and catalog management](docs/catalog-management.md) and
[current publication behavior](docs/storefront-editor.md).

Catalog discovery and product entry behavior: [workflow notes](docs/discovery-workflows.md).
