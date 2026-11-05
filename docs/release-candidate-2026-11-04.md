# Production cleanup candidate evidence register

Work started 4 November 2026; final local verification: 5 November 2026, India time. Source review baseline:
`6b2c93689ce8e5164b05bc71ba9088281aec43b1`. Current implementation is an uncommitted
working-tree candidate identified by the source manifest and local image below. The original
[audit](production-readiness-audit-2026-11-04.md) assessed `d259b18`; its earlier CI,
browser failures and deployment observations are historical, not this candidate's
verification. No production readiness approval or deployment is asserted here.

## Candidate outcome

The implementation is locally verified and remains uncommitted on
`codex/production-cleanup`. It is **not approved for production launch**. The
owner's material, actual Hostinger/Coolify configuration, backup/restore evidence,
provider-account validation, training and live-money/pilot work remain open.
Production defaults to `WECRATFS_PUBLIC_CONTENT_APPROVED=false`: current/legacy
information pages and new checkout stay unavailable until approved material is
installed and reviewed. Existing orders, provider callbacks, login and recovery
remain available. A flag alone does not approve content.

Changed implementation groups:

- Repository/build inputs: removed 2,271 tracked `.tmp` files while preserving
  physical copies; excluded scratch state from Git/Docker; removed Railway SDK,
  configuration, environment fallback and provider-specific host/build settings.
- Production guards: explicit HTTPS URL, normalized environment, real Razorpay
  provider, live keys outside private staging, known development storage keys
  rejected, and simulated capture helpers restricted to explicit local development.
- Finance: additive migration 55; distinct preparer/approver, immutable recipient
  versions, protected PDF/image evidence, exact amount/date/reference, ledger
  binding, proof revalidation, atomic audit and flagged historical paid records.
  Refund creation now shares the finance lock; real S3 accepts only scoped private
  finance evidence. The app records external bank transfers; no button sends money.
- Truthful presentation/cache: neutral unavailable imagery replaces unrelated
  sample photographs in live projections; mutable aliases revalidate, dependency
  documents use content hashes, font preloads match CSS references and hot media
  derivatives avoid original-object opens after fresh database visibility checks.
- Query correctness: seller pages load workspace data on demand; the enabled
  PostgreSQL suite exposed and now covers a singleton finance count incorrectly
  scanned through a four-column helper.
- Build/security: Go 1.26.10 in module/container/CI; patched `x/net`/`x/crypto`
  dependencies and required transitive versions; generated-asset checks and
  dedicated projection/finance databases/browser checks wired into CI. No remote
  CI run, commit, push, deployment or live financial action is implied.

## Binding decisions and current instructions

- Hostinger VPS/Coolify is the production target; active Railway integration is
  removed from active code/configuration. Historical Railway reports remain dated evidence.
- Preserve Go, templ, PostgreSQL, Redis and accepted designs. Keep isolated test
  fixtures/developer tools, applied migrations and business records.
- Login uses username/email plus password with the role read from stored account
  data. No MFA, by owner decision. SEC-01/SEC-02 are intentional owner-deferred
  security exceptions, not satisfied MFA requirements.
- Payouts are external manual bank transfers. Distinct operators prepare and
  approve; actual protected transfer evidence is required before paid status.
- The owner supplies approved policies, contacts, payout/shipping/returns terms,
  merchant facts and permitted media. Missing material stays unpublished; samples
  cannot become approved claims by copying them into documentation.
- This work prepares a candidate. Publishing, live-money drills and a controlled
  pilot require separate authorization.

Use the [current operations handbook](operations-handbook.md),
[release/recovery checklist](release-checklist.md),
[four-role contract](four-role-workflow.md) and
[category instructions](catalog-management.md). The general Super Admin
Storefront publisher is retired; category saves publish immediately.

## Reconciliation method

The [CSV](production-readiness-backlog-2026-11-04.csv) retains all 181 task IDs,
priorities and original criteria. Each row now has one current disposition:
**confirmed**, **already satisfied**, **verification required**, **owner input
required**, **deferred** or **superseded**. Original work type is retained as
historical classification. Source references identify where implementation or test seams live. Only rows
with fresh command evidence claim execution; partial criteria remain open.

Documentation-only completion is kept separate from training, deployed controls
and successful drills. A written SOP does not prove named staffing, approvals,
monitoring, delivery, recovery or production configuration. Measurements and
optional expansions stay open until justified; no task becomes done because it
looks plausible in source.

Final candidate dispositions: **14 already satisfied**, **124 verification required**,
**27 owner input required** and **16 deferred**; no remaining baseline G row is
labelled an uncorrected implementation gap. These are not readiness percentages.
FIN-01/02/03 now have implementation and isolated execution evidence. FE-03's
code/local browser evidence passes, while its actual post-promotion Hostinger
verification stays open. The CSV preserves every original criterion and names
these limits. SEC-01/02 remain owner-deferred, not implemented MFA.

## Source coverage inventory (initial review snapshot)

The initial review snapshot below counts tracked and non-ignored new files in first-party
source roots, after temporary-copy untracking. It excludes `.tmp`, documentation
artifacts, user-owned `.taskplane` work and dependency caches such as node_modules. Counts describe inventory,
not executed test coverage or a claim that every file/line was reviewed.

| Subsystem | Source/config files | Generated/vendor subset | Other assets | Review/evidence status |
| --- | ---: | ---: | ---: | --- |
| Composition and configuration | 26 | 0 | 0 | Startup/provider/URL guards and role routing inspected; final regression evidence recorded below. |
| Domain and ports | 27 | 0 | 0 | Contracts inventoried; targeted boundary review only, not every line reviewed. |
| Application services | 82 | 0 | 0 | Auth, payment, inventory, support and media seams targeted; fresh regression evidence below. |
| PostgreSQL adapter and migrations | 157 | 6 | 0 | Authoritative data/migrations and relevant finance/recovery seams targeted; real-storage evidence below. |
| Other adapters | 35 | 0 | 0 | Provider/storage/session seams inventoried and targeted; external account/deployment behavior unverified. |
| HTTP transports | 56 | 1 | 0 | Route/media/cache/auth seams targeted; fresh API/browser evidence below. |
| Live handlers and view models | 73 | 0 | 0 | Portal ownership, current navigation and finance/projection seams targeted; fresh candidate behavior below. |
| Templates and asset sources | 223 | 92 | 168 | Current forms/role surfaces and embedded assets inventoried; generated/vendor files counted separately. |
| Test suites and fixtures | 60 | 0 | 3 | Fixture isolation and existing test seams inventoried; fresh final suite results below. |
| Tooling, CI and deployment | 39 | 0 | 0 | Commands, backup helper, CI and manifest inspected; actual host/schedule/restore unverified. |

Root manifests/build inputs are also reviewed separately: Dockerfile, package
manifests/lockfile, Makefile, Compose, ignore rules and tool configuration. The
graph reports partial SQL/template coverage and excluded deployment/temporary
subtrees. Exact-path coverage is best effort; source reads supplied fallback for
the cited active contracts. Unvisited partial ranges remain unverified. Neither
graph absence nor low degree authorizes deletion or proves whole-codebase safety.

## Requirement-to-evidence register

| Requirement/risk | Observable evidence needed | Current result |
| --- | --- | --- |
| Repository cleanup | Tracked temporary copies excluded; local copies retained; source/generated assets build. | Index/local-copy checks and generated/container builds pass. No Git-history rewrite. |
| Coolify and Railway removal | No active SDK/config/runtime fallback; explicit public URL; compatible image and host configuration. | Active integration removed; local guards/image checks pass. Actual host configuration remains unverified. |
| Truthful production paths | Approved/persisted data, honest empty/error states, no production simulation or invented financial success. | Targeted source/regression checks pass; publication hold defaults closed. Owner content and full deployed behavior remain open. |
| Password login and role boundaries | Username/email credential checks; four roles; failed cross-role access; revoked stale sessions. | Local suite/browser checks pass in the scope below. External Firebase checks skipped. MFA intentionally absent. |
| Manual payout controls | Separate actors; verified versioned recipient; actual transfer date/amount/reference/private proof; atomic replay/hold protections. | Candidate PostgreSQL/storage regressions and fixture browser checks pass. Legacy reconciliation and real-operator rehearsal remain open. |
| Asset cache correctness | Versioned content remains immutable; mutable aliases revalidate and updated dependency reaches returning client. | Local header, generated-URL and returning-browser checks pass. Actual Hostinger/CDN promotion remains unverified. |
| Payments/refunds/inventory | Isolated deterministic contention/replay/failure tests plus provider sandbox rehearsal on exact candidate. | Local Go suite passes with isolated dependencies. External provider sandbox and live-money rehearsals remain unverified. |
| Performance, SEO and accessibility | Representative route/media/query measurements, crawl, browser/keyboard and manual accessibility results. | No new field-performance, real-device or conformance certification. |
| Backups and environment isolation | Actual scheduled encrypted database/media backups, separate keys/retention/alerts and isolated matched restore. | External operational evidence required. Helper script alone is insufficient. |
| Owner content and operating policy | Approved wording, effective versions, real contacts/facts, media permissions, staffing and finance decisions. | Owner input required. |
| Handoff/training | Procedures match current routes; real operators complete staging scenarios and record findings. | Handbook/checklist authored; local role/finance browser checks pass. Real-operator training remains unverified. |

Additional source-confirmed baseline gap: production accepted the local payment
provider, and the published local S3 credentials were not all rejected. Candidate
source now requires Razorpay in production, an explicit HTTPS public URL,
canonicalized environment handling and development/local gating of capture
helpers, and rejects the known local storage credentials. Local configuration
regressions pass; actual deployment configuration remains unverified. REL-03 and
SEC-08 retain that distinction.

## Checks and boundaries

Final local checks on 5 November 2026 use Go 1.26.10 and the uncommitted candidate.
Raw logs/manifests are in the ignored local directory
`.tmp/production-cleanup-evidence/`; they are not deployed or committed evidence.
No private QA credential file should be published.

Every local result below applies to baseline `6b2c936` plus the candidate source
manifest `48c6e994bd32b674b8c6a0cfd7fc28c85f6ed8f910d4e51b410b7dfbd73efc78`,
not to the baseline alone. The final hash comparison rechecked all 978 files.
The isolated application and its three test dependency containers were stopped
after verification; evidence and preserved local temporary files remain on disk.
Unrelated local applications were not stopped.

| Check | Candidate/environment | Fresh outcome |
| --- | --- | --- |
| `go test -p 1 -count=1 -json ./...` | Disposable PostgreSQL 17, Redis and S3-compatible RustFS; dedicated projection/finance databases | Exit 0: **928 tests/subtests passed**, 35 packages with tests passed; 21 packages have no tests. Four live Firebase/push checks skipped because those external prerequisites are absent. |
| Manual settlements/refund locking | Real PostgreSQL; private evidence storage port; deterministic concurrent liability test | Nine focused top-level finance cases passed; full-suite rerun passes. Covers distinct operators, aliases/CSRF, revisions, duplicate decisions, recipient/ledger changes, holds, missing/corrupt/unavailable proof and 54-to-55 legacy preservation. |
| Actual S3 evidence adapter | Separate local RustFS instance, synthetic documents | Put/Open/Delete roundtrip, finance-only PDF/type/key/size contracts, nil fail-closed and denied public product presigning pass. Actual production bucket ACL/TLS/retention remains unverified. |
| Node font/build regression | `npm run test:assets` | Exit 0: font byte changes alter URL and generated CSS using known independent SHA values; maintained CSS stays unchanged; repeat build deterministic. |
| Generated artifacts | Pinned templ check, sqlc generation plus clean generated diff, image/CSS/JS build and `make css-check` | All exit 0. Required generated source/assets retained. CI now verifies generated assets and creates the dedicated finance/projection databases. |
| Architecture and static analysis | `bash scripts/check-architecture.sh`; `go vet -p 1 ./...` | Exit 0. This verifies the configured boundaries and analyzer scope. |
| Customer layout Playwright | Isolated template/API fixture; Chromium | Exit 0: five checks pass at 320/390/720/1440px, including keyboard focus, totals, cart controls and checkout recovery. |
| Finance Playwright | Separate loopback render fixture; Chromium | Exit 0: two checks pass at 390/1440px with keyboard preparation/approval controls, required evidence and legacy warning. Rendered screenshots inspected; no redesign. |
| Real application browser probe | Compiled application at 127.0.0.1:18088; isolated real dependencies; all outbound providers disabled | Guest and four roles pass username/email password login, stored-role destination, permitted/denied workspaces, 390/1440px, keyboard entry and no page errors. Super Admin's existing shared Admin operations remain allowed. No sample products inserted. |
| Returning browser dependency migration | Real Chromium cache, old-alias fixture and current application's actual HTML/assets | Old immutable script alias fetched once and reused on second visit; current content-versioned dependency fetched once after local release switch. This is local cache migration evidence, not live CDN/deployment proof. |
| Go vulnerability scan | `govulncheck` on final Go 1.26.10/module set | Exit 0: **No vulnerabilities found**, including required-module findings after targeted dependency patches. |
| Current-source secret scan | 900 first-party text-source/doc files; default Gitleaks rules with reviewed exact/path-scoped fixture/client exceptions | Final scan has zero findings. Positive synthetic PAT canary is detected; exception file does not disable default rules. History, generated/minified copies and archived QA media are not certified by this scan; remote CI remains open. |
| Container build and scan | `weevcrafts:production-cleanup-20261105`, unprivileged `app` user | Build exits 0. Final Trivy HIGH/CRITICAL vulnerability and secret scan exits 0; exact image identity below. |
| Actual Hostinger/Coolify, backups and external provider drills | Deployed environment | **Unverified.** No deployment, live money, real mail/push, production load test or destructive restore performed. |

Independent finance review initially found three P1 defects: real storage rejected
finance keys/PDFs, decisions did not reopen proof bytes, and pending refunds could
race the settlement lock. All were corrected, covered by direct regressions and
independently rereviewed without remaining material findings in that scope.
Separate core review verified content withholding, provider/cache/CI fixes and
reviewed the final security/tooling/query changes. This is scoped independent
review, not a whole-codebase security certification.

Failed checks were investigated rather than silently relabelled:

- Initial Go 1.26.8 scan found 13 reachable standard-library vulnerabilities.
  Go 1.26.10 and targeted `x/net`/`x/crypto` patches resolve these and the three
  uncalled required-module findings. See [Go release history](https://go.dev/doc/devel/release)
  and [Go's certificate-security notice](https://go.dev/blog/emudhra-incident).
- The real PostgreSQL suite exposed a one-column financial count passed to a
  four-column scan helper. Existing whole-dataset regression fails before the
  fix and passes after it. Error propagation still withholds unavailable totals.
- Secret scan initially flags 19 deterministic local fixtures, an object ID and
  the existing public Firebase browser key. Exceptions preserve all default
  rules, restrict the client exception to the current value/path and require
  key changes to be reviewed. [Firebase documents client-key handling and required
  restrictions](https://firebase.google.com/docs/projects/api-keys); actual key
  restrictions, Security Rules and IAM still require owner/environment evidence.
- One ad hoc probe assumed Super Admin could not read Admin operations; existing
  company-role contracts permit this. Probe expectation was corrected; access
  controls were not weakened. A refund concurrency test's observer used a cached
  transaction snapshot; autocommit observation now proves the lock wait directly.
- Intermediate compilation during concurrent construction exposed incomplete
  test/view-model fields and one import-name collision. These were corrected;
  no final pass uses an intermediate source snapshot.
- Python Playwright was absent; browser checks use the installed repository Node
  Playwright. MinIO's proposed public image pull was denied, so isolated SDK
  integration uses a separate already-available RustFS image. Availability of the
  actual approved production storage image remains an infrastructure gate.
- A root-owned scanner cache temporarily blocked Docker context traversal.
  Ownership was repaired only inside this task's cache and the build rerun.

Known bounded residual work includes the source's 5,000-per-class sitemap cap,
uncached original-image HEAD buffering, admin query profiling, realistic catalog
load/performance budgets, physical-device/screen-reader verification, training,
and all deployed-environment/business gates. No current catalog omission, field
performance score or accessibility conformance is claimed.

Environment identification: read-only container metadata on 4 November shows
localhost:8080 is `backend-api-1` (`localhost/saas-school-backend:local`), an
unrelated project. Its homepage 404 is not evidence of a WeevCrafts defect. Keep
that service untouched; use a separately identified loopback port for candidate QA.

Do not point mutations, load tests, seed/reset commands or fixture-creating Go
tests at business data. Retain individual failed checks with their actual cause;
do not silently retry until green or treat missing dependencies as a pass.

## Owner handoff and release gates

Before launch the owner must supply the legal/business identity, actual support
and grievance contacts, permitted categories/claims, merchant inventory/media,
shipping serviceability/prices/handling estimates, cancellation/return/refund
terms, seller agreement, payout eligibility/cadence/fees/reserves/negative-balance
policy, tax/accounting treatment, retention/deletion rules, operational staffing
and recovery targets. No legal/accounting conclusion is supplied by this code
review. Review dated applicable obligations with the responsible adviser.

The release engineer must obtain isolated staging and actual infrastructure
evidence. Finance must reconcile existing settlements without fabricating bank
references and rehearse uncertainty/recovery before live actions. Operators must
rehearse the handbook. Close blocking implementation defects and record external
gates/accepted exceptions before describing a build as publishable.

## Historical evidence and repository artifacts

Keep dated reports with their original revision and environment. Large screenshot
trees may contain exact duplicates, but a matching image hash does not prove that
an adjacent report/log is redundant. The cleanup removes temporary repository
copies from tracking/build inputs while preserving local copies. Unique QA,
design and merchant evidence remains retained; no broad screenshot deletion or
Git-history rewrite is authorized. A separate duplicate inventory can guide later
scoped archival without erasing evidence.

The [exact-duplicate inventory](artifact-duplicate-inventory-2026-11-04.csv) hashes
924 tracked QA/screenshot/report artifacts (234,274,953 bytes). It identifies
exact byte matches only. Group counts and duplicate bytes are recorded by the
inventory check below; no artifacts were deleted. Review each report's references
before any later archival, and never replace unique evidence with an assumed
representative screenshot.

Inventory result: 64 duplicate groups, 140 extra copies and 8,073,244 duplicate bytes beyond one copy. Hash equality is not an authorization to delete.

## Candidate identity and primary changed files

- Baseline: `6b2c93689ce8e5164b05bc71ba9088281aec43b1`; branch `codex/production-cleanup`; uncommitted working tree.
- In-scope code/test/build manifest: 978 files, SHA-256 `48c6e994bd32b674b8c6a0cfd7fc28c85f6ed8f910d4e51b410b7dfbd73efc78`.
- Local image `weevcrafts:production-cleanup-20261105`: `sha256:79b263b543541b9bf7fafc74374ae62e2e523867f8141c9afc6b6a0cb05aa715`; runtime user `app`.

Primary implementation: [settlements](../internal/web/handlers/settlements.go),
[migration 55](../internal/adapters/postgres/migrations/0055_manual_settlement_controls.sql),
[S3 adapter](../internal/adapters/storage/s3.go),
[refund lock](../internal/adapters/postgres/refund_repository.go),
[production guards](../internal/config/security.go),
[publication hold](../cmd/website/release_access.go),
[cache headers](../internal/transport/web/asset_cache.go),
[media delivery](../internal/transport/web/media.go),
[portal projections](../internal/web/handlers/live_portal.go),
[finance insights](../internal/web/handlers/superadmin_insights.go),
[build image](../Dockerfile), [dependencies](../go.mod) and
[CI](../.github/workflows/launch-checks.yml). Generated templates, dependency
references, focused regressions and the current operations/release docs accompany
these changes. Pre-existing AGENTS/index changes and user scratch work remain
separate and preserved.
