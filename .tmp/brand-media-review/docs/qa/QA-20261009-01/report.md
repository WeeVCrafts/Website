# Application verification — QA-20261009-01

Executed on 9 October 2026 against freshly built local instances. **The full 754-row manual campaign is incomplete; this report is not a release approval.** The CSV records only complete, evidenced procedures as Passed. Related automation remains supporting coverage.

Tracker totals: **124 Passed, 1 Failed, 38 Blocked, 591 Not run**; 754 rows total. These counts refer to complete manual procedures, not the number of automated assertions or supporting workflows.

## Current application and build

- Fetched `origin/main`; HEAD and upstream have zero commits ahead/behind. Base revision: `e853611798973f9197efc007357f19b1bbc5e261`.
- The working tree additionally contains the payment-helper security fix described below. The user's existing `docs/role-workflows.md` edits were preserved.
- Reinstalled locked Node dependencies with `npm ci`, verified Go modules, regenerated templ output, rebuilt CSS, built the Windows executable, and rebuilt/recreated the Docker application. Dependency versions were preserved; “latest” here means the current fetched source plus the tested fix, not arbitrary dependency upgrades.
- This application serves the backend and embedded frontend from one Go binary. `bin/weevcrafts.exe` is the current standalone Windows build, using Go 1.27.1. The Docker build uses the repository's Go 1.26 image.
- Running image: `sha256:3d47fd8ddbe8946e2cbd109cf432818e8a6e6a1db7f40a2d41b529900bac7108`. Main and QA containers were checked against that exact image.
- Main application: <http://127.0.0.1:8080>. `/health/live` and `/health/ready` both returned 200. PostgreSQL and Redis probes were ready; database migration 37 is applied.
- Nothing was committed, pushed, or deployed to an external environment.

Evidence: [final runtime verification](final-verification.txt), [Docker build](docker-build-final.log), [Windows build identity](windows-build-identity.txt), [dependency install](npm-ci.log), [template generation](templ-generate.log), [CSS build](css-build.log).

## Executed checks

| Check | Observed result |
| --- | --- |
| Final backend suite, `go test -p 1 -count=1 -json ./...` | 287 top-level tests passed; 700 pass events including subtests; zero failures and zero skips. Actual PostgreSQL, Redis, MinIO, and projection database were configured. |
| `go vet ./...` and Windows build | Passed. Empty log files represent commands with successful exits and no diagnostics. |
| Initial full Linux Go 1.26 race run | 674 pass events, one fixture-setup timeout, three projection skips. It was not a clean full race run. No data-race diagnostic was emitted. |
| Focused Linux race retest | Seven pass events, zero failures: 100-way inventory contention and the new payment-route guard, using reduced concurrency. |
| Original Chromium campaign | 99 passed, eight failed, 107 tests. Seven failures came from an overly strict access-denial assertion; one exposed the missing-page defect. |
| Chromium procedure/recovery/suspension batch | 64 passed, two failed, 66 tests. All eight roles passed navigation, password recovery, password validation, session expiry, and suspension checks. The two failures were the missing-page defect and an expired seeded payment fixture. |
| Fresh local payment helper retest | Passed with a newly created owned pending order. This verifies the allowed local helper boundary, not inventory correctness or genuine provider capture. |
| Production configuration payment guard | Failed before the fix; passed after the fix. The disabled route now returns 404 without changing the persisted order. |
| Order and dispatch SMTP checks | Passed against actual Mailpit delivery from application workers. Local simulated payment was used, so the complete provider-dependent INT-014 procedure remains Blocked. |
| Firefox/WebKit | 27 passed and one Super Admin screenshot hit the original 60-second limit. The focused WebKit retest passed in 1.1 minutes with a 180-second budget and unchanged assertions. All 28 sampled cross-browser checks have passing evidence; the original timeout remains recorded. |
| Documented k6 smoke | Passed: 7,971 requests/checks, zero failures, 2,657 iterations over 120 seconds; P95 16.70 ms and P99 40.65 ms. Compared with the repository's <1% failures, <500 ms P95, and <1,000 ms P99 thresholds. Docker allocated four CPUs and 8,240,349,184 bytes memory; fixture data had 110 users, 16 products, 18 variants, 11 orders, and 10 order items. |

The first native suite passed 675 test events with three projection skips. After enabling the dedicated projection database, an overloaded broad run timed out while inserting the inventory contention fixture. The final serial run passed all tests, and the focused race retest completed the same contention procedure. The original failures and skipped runs remain in the evidence inventory.

Two recovery harness attempts were interrupted after discovering that the live reset form uses the JSON confirmation endpoint and that concentrated independent cases shared the Docker gateway's authentication quota. The completed run uses the actual endpoint and clears only that client's quota keys in the dedicated QA Redis between cases. No in-case limit reset or production-limit change was made. Token expiry and session expiry use controlled disposable database/Redis fixtures.

The initial NAV-002 assertion expected 403 from every other root. `/account` intentionally redirects non-customer users to their assigned workspace. The corrected check accepts only that exact safe redirect; other unassigned workspaces must still return 403. These harness failures are not product defects.

## Defects

**QA-PAY-001 — critical, fixed and retested.** With production environment, local mode disabled, and Razorpay selected, the development payment capture route was still registered. An authenticated customer changed a pending order to paid without a provider charge. `cmd/website/main.go` now registers development payment helpers only when the environment is not production, local mode is enabled, and the local payment provider is selected. Regression cases cover production/local-provider, production/Razorpay, local-mode-off, and real-provider development configurations. Actual before/after application checks verify the persisted order and HTTP status.

Evidence: [before failure](production-guard-before.json), [after verification](production-guard-after.json), [allowed local helper](local-helper-retest.json), [backend guard and race retest](go-race-retest.jsonl).

**QA-GST-001 — P1, open.** GST-020 fails: unknown `/category/missing-qa-*` and `/brands/missing-qa-*` URLs return HTTP 200 instead of 404. Unknown product and general URLs return 404. The failure reproduces in the final Chromium procedure batch. No fix was made for this defect.

Evidence: [final procedure results](playwright-chromium-retest.json), screenshots and traces in [chromium-complete](chromium-complete/).

## Coverage and remaining work

- Complete recorded procedures cover sign-in validation, safe redirects, recovery, password reset/replay/expiry, revocation, suspended accounts, request tokens, and single-role workspace boundaries across all eight roles. Unknown account creation was additionally checked in PostgreSQL: [zero unexpected accounts](unknown-account-evidence.txt).
- INT-007 proves exactly ten successful reservations from 100 customers competing for ten units. INT-008 proves twenty identical concurrent checkout retries create one order, reservation, payment attempt, and outbox record. INT-030 passes the documented local public smoke plus those checkout concurrency checks, with recorded resources and comparison to repository thresholds. This does not establish production capacity or provider latency.
- Existing application automation also passed local registration/email verification, settings, addresses and ownership, privacy/session revocation, product/media moderation, seller support, discounts, customer support, fulfillment, reviews, returns, local refunds, and local mock settlement/payout workflows. These tests do not automatically pass every related manual row.
- Responsive screenshots cover 360, 390, 768, 1024, and 1440 pixels. UI-001 remains Not run as a complete procedure because long-name/empty/paginated/error/media fixture combinations and all usability conditions were not fully exercised. UI-015 critical workflows across all browsers are also incomplete.
- Genuine payment sandbox callbacks, captures, reconciliation, refunds, payouts, shipping-provider workflows, HTTPS release-cookie behavior, screen-reader use, and release policy/scope evidence remain unavailable or unverified. Corresponding rows are explicitly Blocked.
- Other Not run rows still require their complete manual procedures. The full campaign must continue before claiming application-wide acceptance.
- `npm audit` found eight development-tool advisories: six high and two moderate. They were not resolved by force-upgrading the locked tooling. See [audit details](npm-audit.json). CSS also reports an old Browserslist dataset. Successful builds do not resolve those advisories.

## Environment and reproducibility

Tests used isolated PostgreSQL databases `weev_qa_20261009_unit`, `weev_qa_20261009_browser`, `weev_qa_20261009_guard`, and `weevcrafts_projections_20261009`, dedicated QA Redis/Meilisearch, and dedicated storage buckets. Existing shared development data was preserved. SMTP delivery used local Mailpit. The guarded production-configured instance used HTTP with secure cookies disabled solely for testing; it does not prove an HTTPS release configuration.

The local mode browser target was `http://127.0.0.1:18080`; the production guard target was `http://127.0.0.1:18081`. The four dedicated QA application/guard/search/Redis containers were stopped after testing to release resources. Their containers, dedicated databases, storage buckets, and upload volume are retained for reproduction. The main application on port 8080 remains running and healthy. See [resource disposition](resource-disposition.txt).

Browser fixture mutations require `WEEVCRAFTS_QA_DATABASE=weev_qa_20261009_browser` and the exact isolated target. Accounts are the documented local demo identities or disposable role copies, with identifiers annotated in the reports. Original workflow tests mutate fixtures; use a freshly seeded dedicated QA database for a complete repeat.

To inspect retained QA instances, start `weev-qa-20261009-redis` and `weev-qa-20261009-search` first, then `weev-qa-20261009-app` and `weev-qa-20261009-guard`. This does not reset the already mutated fixture data.

Representative completed commands:

```text
npm ci
go mod verify
templ generate
npm run css:build
go test -p 1 -count=1 -json ./...
go vet ./...
go build -o bin/weevcrafts.exe ./cmd/website
docker compose build app
docker compose up -d --force-recreate app
node node_modules/@playwright/test/cli.js test --reporter=list,json
node node_modules/@playwright/test/cli.js test --config=tests/e2e/browser-matrix.config.js --reporter=list,json
node scripts/record-qa-results.js docs/qa/QA-20261009-01/manifest.json
```

The k6 container ran `scripts/load-smoke.js` on network `weevcrafts_default` with `BASE_URL=http://weev-qa-20261009-app:8080` and image digest `grafana/k6@sha256:e66db15b860113878fa74670e31f5e274830b7b6e42c8bff28b2f2d86a257603` (k6 2.3.0). Results: [load JSON](load-smoke.json), [load log](load-smoke.log), [resource/data identity](load-environment.txt). Set `PLAYWRIGHT_JSON_OUTPUT_NAME` to an absolute evidence path when switching Playwright config directories.

Backend integration commands require `DATABASE_URL`, `PROJECTION_DATABASE_URL`, `REDIS_ADDR`, `S3_INTEGRATION=1`, `S3_ADDR`, and the dedicated `S3_BUCKET`. Linux race tests ran in a Go 1.26 container with `-race -p 1` and `GOMAXPROCS=2` for the focused retest. Playwright requires `WEEVCRAFTS_BASE_URL`, Chromium/Firefox/WebKit installations, and the isolated fixture environment described above.

The [manifest](manifest.json) declares every consumed run and exact tracker mapping. [Machine-readable summary](run-summary.json) contains all manual statuses, raw run totals, and supporting automated checks. [Execution tracker](../../test-execution-tracker.csv) retains all 754 execution IDs and 373 case IDs. Original and final reports, screenshots, traces, failed/interrupted harness logs, backend JSONL events, and build logs are preserved in this directory.
