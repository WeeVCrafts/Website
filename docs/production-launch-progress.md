# Production launch implementation

## Phase 1 checkpoint — 31 October 2026

The approved launch keeps four roles, manual bank payouts, seller-booked shipping,
ordinary receipts, INR and India delivery. The owner subsequently deferred Railway
provisioning for a move to Hostinger VPS and authorized remaining application work.

Implemented locally:

- GitHub CI for isolated PostgreSQL/Redis integration tests, Go vet, template
  generation, CSS builds, sqlc checks, customer layout Playwright and security scans.
- Staging access middleware with noindex/no-store headers. Health probes and the
  existing signature-verified Razorpay webhook remain reachable without Basic Auth.
- Production startup safeguards apply to staging. Staging requires a dedicated
  access password of at least 24 characters and Razorpay test keys. Outbound email
  remains disabled until recipient isolation is implemented.
- Public production readiness returns status only. Production metrics require a
  dedicated Bearer token (`WECRATFS_METRICS_TOKEN`, at least 32 characters);
  absent or incorrect tokens receive 404. Tokens are supplied privately to monitors.
- An encrypted database export command is available in `scripts/backup-database.sh`.
  It requires pg_dump, age, a private DATABASE_URL, an age public recipient and an
  absolute BACKUP_DIRECTORY. It does not configure a schedule or back up objects.

Railway changes:

- Created empty staging environment `4f9a0d3b-6646-4a05-bc23-e438db39bbee`
  in project `8c79a499-19f3-404b-bb80-f739f8e026e5` without copying production secrets.
- PostgreSQL and Redis template requests returned workflow IDs, but subsequent
  environment inspection showed no services. Provisioning is not verified.
- Separate staging bucket creation failed because the project plan allows one
  bucket. Never use the production bucket as an isolation workaround.

## Remaining phase 1 gates

1. Resolve separate staging storage and verify actual PostgreSQL/Redis provisioning.
2. Configure staging-only credentials, private networking, PostgreSQL CA verification,
   bucket TLS, access credentials and a sandbox webhook. Review variables by name
   and environment identity without logging secrets.
3. Deploy the exact tested SHA to staging. Run CI remotely and verify access denial,
   noindex, checkout and signature-verified sandbox callbacks.
4. Schedule encrypted exports and object/legacy-upload backups. Maintain 30 daily
   recovery points. Keep the age private key in a separate recovery secret store.
5. Restore database plus matching objects to an isolated target. Verify media,
   roles, ownership, saved totals and deleted-account handling. Record backup hash,
   recovery point and elapsed time against RPO 24 hours / RTO 4 hours.
6. Rehearse compatible application rollback and migration recovery. Reconcile
   external payments since the recovery point before reopening checkout.

The full application is not production-ready at this checkpoint. Remaining financial approval workflows,
payout approvals, shipping operations, MFA, documents, communication tracking,
remaining UI work and final release drills are not implemented by this change.
Do not promote production or run live charges based on this checkpoint.

## Refund recovery implementation

Migration 52 adds submission state, durable submission claims and scheduled recovery.
Only a queued record can claim one provider POST. Timeout, process failure and
inconsistent provider responses cannot automatically issue another POST. Legacy
refunds without provider references migrate to uncertain for review.

The five-minute reconciliation job reads known provider references or searches
bounded payment-refund pages for the original receipt. It validates the payment,
amount and currency before using the existing transactional completion method.
Lookup failures back off to a maximum of 60 minutes; successful pending results
return to five-minute polling.
No match or multiple matches require investigation; absence never authorizes a POST.
Before deploying migration 52, drain and stop older application instances that can
submit refunds. Older refund code does not honor the new submission claim. During
rollback, block refund submission routes before starting an older application;
reconcile every submitting/uncertain/pending record before reopening those routes.
Do not roll back the refund schema or delete recovery records to enable retries.
The existing finance projection reports uncertain/submitting states and marks
pending records older than 24 hours as needs_review. Provider-confirmed failure
is terminal until reviewed. Signed refund events use the same transactional completion
method as reconciliation. Event delivery and financial completion are individually
replay-safe; retry recovers a failure between completion and delivery recording.

Railway provisioning is paused by owner request. No Hostinger deployment has been
attempted. These application controls are provider-independent and can run on VPS.

## Receipt corrections

Receipt tables paginate long orders, wrap names/options, display all sellers and
use actual payment state for paid/payable labels. Numeric columns shrink to fit.
Uploaded PNG/JPEG/WebP photographs resolve through internal storage only after
checking media membership in the already-authorized order. Images are bounded in
bytes and decoded dimensions, resized and converted to PNG before PDF embedding.
Unavailable images leave a placeholder; arbitrary external URLs are never fetched.

## Verification checkpoint

- Full Go suite passed before the final receipt thumbnail and metrics-token changes.
- Focused refund, provider and webhook tests passed.
- Isolated PostgreSQL migration and refund recovery/replay test passed.
- Receipt pagination and thumbnail tests passed after the receipt changes.
- sqlc 1.30.0 Docker generation completed; repeated output hashes matched.
- Backup script syntax, architecture boundary check and workflow YAML parsing passed.
- First remote CI run found incorrect Redis option quoting and vulnerable dependency
  versions. Redis quoting was corrected; CI now selects the latest Go 1.26 patch.
  pgx was upgraded to 5.9.2, go-redis to 9.7.3 and x/net to 0.60.0; x/crypto
  resolves to 0.57.0. The corrected CI run and dependency regression tests must pass.
- The updated local Go suite passed. Remote image scanning additionally identified
  three high-severity transitive findings. go-jose is updated to 4.1.6, compress to
  1.18.7 and gRPC to 1.83.2. These image-scan fixes require another remote scan.
- Template drift was caused by Windows CRLF characters embedded in generated
  literals. Template inputs now enforce LF through `.gitattributes`; generated
  embedded scripts/styles were regenerated without changing their behavior.
- Backup restore and hosted deployment remain unverified.
