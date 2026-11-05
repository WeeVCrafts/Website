# Release and recovery checklist

Target: Hostinger VPS/Coolify. Review baseline: `6b2c936`. This checklist describes
required evidence; it does not authorize deployment, live payments, destructive
restoration or a pilot. Current task produces a release candidate. Record the
candidate SHA plus dirty-diff/artifact identity until committed, image digest,
environment, command, result, timestamp and reviewer for every gate in the
[candidate register](release-candidate-2026-11-04.md).

## Identity, isolation and configuration

1. Assign release, finance, support and recovery owners and backups. Obtain approved
   policies/contact details and identify excluded categories/actions. Keep missing
   merchant facts, testimonials, certifications and unlicensed media unpublished.
2. Record the actual Hostinger host, Coolify application/environment, canonical
   domain, image digest/revision and PostgreSQL migration level without secrets.
   Do not identify a server by port alone; another project can use localhost:8080.
3. Verify staging has a separate PostgreSQL database, Redis instance/namespace,
   private bucket, credentials, identity settings and provider webhook secrets.
   Staging uses Razorpay test keys; no production recipient, push token or data
   may be used. Keep outbound mail disabled until recipient isolation is proven.
4. Verify production uses explicit HTTPS `WECRATFS_PUBLIC_URL`, production mode,
   secure cookies, dedicated credentials, PostgreSQL `sslmode=verify-full` and
   trusted CA, private Redis, TLS storage and a private bucket. Check variables by
   name and mode without printing their values. Local/demo flags and helper routes
   must not authorize simulated business actions in production.
   Keep `WECRATFS_PUBLIC_CONTENT_APPROVED=false` until approved policies,
   contacts, support hours and permitted content are installed and reviewed on
   this candidate. Test the production hold: policy pages and new checkout
   return 503 without publishing placeholder claims; existing orders, login,
   recovery and verified provider callbacks remain available. Record owner
   approval before enabling the variable. Adding items to a cart remains allowed.
5. Verify Coolify ingress/TLS renewal, trusted proxy behavior, host redirects,
   private dependency ports, access to the console, readiness/liveness, request
   draining and protected metrics. `deploy/coolify-dependencies.yml` is repository
   guidance, not proof that deployed credentials/TLS/schedules are correct.

## Candidate checks

Run from the repository root with supported Go/Node and pinned tools from CI.
Use only disposable integration services. The full Go suite creates fixtures;
never point it at existing local business or production databases. Supply its
private `DATABASE_URL` and Redis/storage settings through the isolated runner,
not copied credential strings in logs. Serialize checks sharing these services.

```sh
git rev-parse HEAD
git status --short
bash scripts/check-architecture.sh
go vet ./...
go test -p 1 ./...
make templ-check
bash scripts/check-sqlc-generated.sh
npm run assets:images
npm run css:build
make css-check
npm run test:assets
git diff --exit-code -- ':(glob)**/*_templ.go' ':(glob)**/image_variants_generated.go'
npx playwright test -c tests/e2e/customer-layout.config.js
npx playwright test -c tests/e2e/finance-controls.config.js
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
docker build -t weevcrafts:release-candidate .
```

Generation/build commands may change tracked artifacts. Inspect differences and
compare generated results with the intended candidate; do not erase valid work
to force a clean check. Run secret and HIGH/CRITICAL container vulnerability scans
using the configured CI jobs and retain their reports. A successful old CI run
does not cover this candidate, branch protection or all configured integrations.

Supplement the layout fixture with affected real-storage integration tests and
the existing role/purchase/recovery/coordination/category suites using their
documented isolated configurations. Verify guest, Customer, Seller, Admin and
Super Admin allowed/denied paths; username and email login; session revocation;
cross-seller object/media/attachment access; CSRF; validation and dependency errors.

Rehearse last-unit contention, duplicate retries, expired reservations, late and
replayed webhooks, capture mismatch, refund timeout/crash/reconciliation, held
fulfilment, payout preparation/independent approval, transfer evidence, duplicate
completion and legacy reconciliation. Compare saved amounts and provider truth;
mock protocol tests do not prove provider sandbox or bank operation.

Inspect rendered results at 320/390/768/1024/1440px, keyboard-only, zoom and
reduced motion. Run Chromium/Firefox/WebKit as available and record physical
device/screen-reader gaps explicitly. Measure routes with representative real-sized
catalog/media only after competing builds stop. Report field and lab measurements
separately; no speed score, field INP or accessibility conformance follows from a
layout/axe pass alone.

## Backup and isolated restoration

Before promotion, verify a monitored encrypted backup schedule, independent
storage, retention, access controls and separate recovery-key custody. The
`scripts/backup-database.sh` helper creates an encrypted PostgreSQL export only;
it neither schedules backups nor preserves objects. Supply `DATABASE_URL`,
`BACKUP_AGE_RECIPIENT` and an absolute `BACKUP_DIRECTORY` privately before running
it. Record successful output, size/hash and readable/decryptable format. Back up
matching private objects and remaining legacy uploads/maker portraits with a
manifest/version boundary. Do not put recovery secrets or bank evidence in Git.

The recovery operator must restore database plus matching media into an isolated
target and record recovery point and elapsed time. Reapprove the previously
documented RPO 24 hours, RTO 4 hours and 30 daily recovery points against actual
financial risk. Verify saved order/item totals, inventory/reservations, ownership,
roles, ledger/audits, media and prior deletion state. Rebuild disposable search
data and invalidate sessions; Redis is not authoritative business recovery data.

Before reopening affected operations, reconcile provider payments/refunds since
the recovery point, reconcile bank transfers, and reapply privacy deletion/session
cutoffs. Never overwrite post-backup orders or repeat a financial POST because
the restored database lacks its outcome. Retain drill evidence and unresolved
differences. A backup file or checksum without restoration is insufficient.

## Promotion and compatible rollback

Promotion requires a separate owner instruction. Confirm tested candidate/image,
successful required CI, approved content and closed blocking operational gates.
Record accepted exceptions, including owner-deferred MFA. Run a controlled pilot
only after separate authorization for its actual payment/refund process.

Before migration, capture current schema/version and verified recovery boundary;
stop/drain unsafe older writers. Existing migrations are applied at startup.
New schema must be additive and historical financial records remain intact.
Migration 52's durable refund claims require compatible writers: do not resume
older refund code that can bypass submission claims. The payout controls likewise
must not be bypassed by running an older status-only settlement writer.

Rollback triggers include failed readiness, authorization bypass, inconsistent
financial state, severe error/latency change or migration failure. The release
owner contains affected operations, preserves signed callbacks/evidence and
records the incident. Use the last tested application only after proving schema
and financial-writer compatibility. Otherwise forward-fix or perform an explicitly
authorized isolated-tested recovery; never drop new columns, delete claims or
restore an older database over newer orders to make an old binary start.

After promotion/rollback, record observation window and named operator, verify
readiness, browsing/login, relevant role paths, callbacks, queues, storage and
metrics, then reconcile financial events. Backup alerts, payment exceptions,
uncertain refunds, outbox age, disk/CPU/memory and certificates require actual
monitors and assigned recipients. The owner must choose support coverage,
monitoring thresholds and observation duration before launch.
