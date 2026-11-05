# Weevcrafts production readiness audit and task register

> **Historical assessment; superseded decisions:** This audit assessed `d259b18`. The cleanup review starts at `6b2c936`; use the [candidate register](release-candidate-2026-11-04.md) and reconciled CSV for current disposition. On 4 November the owner selected Hostinger VPS/Coolify, retained username/email plus password with server-derived role, declined MFA, required separate payout operators, and undertook to supply approved content. SEC-01/SEC-02 are owner-deferred exceptions, not completed controls. The original findings and priorities below remain historical evidence; implementation and fresh verification must be recorded separately.

Audit date: 4 November 2026, India time. Source baseline: `d259b18ac55c7905fb0669ef20cee7946527a6d1`, plus the inspected working tree. This is an assessment and backlog, not a production release approval. No application code, business records, infrastructure, payments or existing designs were changed by this audit.

## Assessment

The application has a substantial implementation, including real payment integration, transactional stock reservations, payment exception holds, refund recovery, seller earnings snapshots, responsive uploaded photographs, role controls and automated release checks. Its Go/templ/PostgreSQL/Redis stack should be preserved. A framework rewrite, microservice split, automated bank payouts or integrated courier booking is not required to finish the currently agreed marketplace scope.

The remaining work is principally financial operating controls, privileged authentication, recovery evidence, clear operational instructions, environment verification and measured performance work. A successful deployment or CI run alone does not establish that staff can operate the marketplace safely or that its real users receive good loading performance.

Current scope recorded in project documents: India delivery, INR, four roles, seller-booked shipping, manual bank payouts and ordinary receipts. Any expansion needs a separate product/accounting decision. Seller Admin is the Seller workspace, not a fifth role; retired staff/team roles should not be reintroduced through documentation.

### What was checked

- README, architecture, current role/UX contracts, release/hosting notes, payment/media/SEO documentation, CI workflow, package scripts and selected current authentication, settlement, seller projection and media-related source.
- Three independent reviews used **Luna only**, covering commerce/security, performance/SEO, and operational UX. Material findings were reconciled against current source. A proposed seller false-empty-state defect was rejected because request-level projection errors already suppress empty/zero presentation.
- Codebase-memory supplied structural orientation. Its fast index is dated 16 October and has stale/missing coverage for newer files, scripts, assets, migrations and docs. Current source and repository search supplied evidence; graph absence was not used as proof of a missing feature.
- GitHub [Launch checks run 41769369283](https://github.com/WeeVCrafts/Website/actions/runs/41769369283) completed successfully for `d259b18`; both `application` and `security` jobs passed. This proves that committed revision's configured checks, not the unrelated uncommitted template changes or every production workflow.
- Impeccable context and the bounded detector ran. The detector returned `[]` for the selected four template files. This is a narrow static result, not accessibility certification or a visual-quality score.
- A fresh Chromium/axe read-only browser probe could not navigate localhost:8080: the connection closed without an HTTP response. The existing isolated customer-layout Playwright command timed out waiting 90 seconds for its fixture server. No browser assertions completed in this run.
- Read-only public requests to `/health/ready` and `/robots.txt` returned 403 from this environment. Their cause is undetermined; this is not evidence that the site is down for customers or Googlebot. No bot challenge, access restriction or production setting was bypassed.
- No production load test, fresh Lighthouse/CrUX measurement, authenticated production operation, physical-device test, new security certification or restore rehearsal was completed.

Latest November QA notes record deployment on **Coolify**, including responsive uploaded media. Older Railway-only and missing-image-derivative statements are historical. The exact currently deployed SHA and live controls still require environment verification. Do not use older completion percentages as a current readiness estimate.

## Evidence that changes the task list

| Finding | Current evidence | Consequence |
|---|---|---|
| Privileged password login has no second-factor challenge in the inspected native path | `internal/application/auth/service.go:68`; `internal/transport/api/handler.go:190` | Implement and verify Admin/Super Admin MFA, including recovery and all alternate login paths. |
| Settlement state workflow exists, but the same authorized Admin can approve and record paid | `internal/web/handlers/live_portal.go:1905`, `:2003`; `web/admin/portal_views.templ:518` | Complete maker/checker controls rather than rebuilding the settlement state machine. |
| Recording paid changes settlement/ledger state without collecting bank transaction evidence | `internal/web/handlers/live_portal.go:1927`, `:2030`; `0021_local_demo_platform.sql:134` | Add transfer reference, verified recipient, amount/date/evidence and bank reconciliation. Generated batch references are not bank UTRs. |
| Finance still labels the action mock/simulated | `web/admin/portal_views.templ:309`, `:530`; seller terms `web/selleradmin/portal_views.templ:79` | Resolve production/manual-payment semantics and promises before using payouts operationally. Changing a label alone is insufficient. |
| Encrypted database export helper exists | `scripts/backup-database.sh:5`; `docs/production-launch-progress.md` recovery gates | Verify an actual monitored schedule, media backups, key custody and full restore. Repository support does not prove a production scheduler is absent or configured. |
| Refund submission claims/recovery and late-payment review holds exist | `internal/adapters/postgres/refund_recovery.go`; `payment_capture.go`; migration 54; `cmd/website/main.go:242` | Rehearse failure/replay paths on the exact release instead of listing reconciliation as wholly unimplemented. |
| Uploaded image resizing now exists | `docs/qa/customer-media-branding-2026-11-03.md`; current media code | Verify cache behavior, quality, resource limits and real-catalog results. An older October note saying uploads have no derivatives is superseded. |
| Seller page builds several datasets on each route | `internal/web/handlers/live_portal.go:2292` | Profile and reduce route-unrelated query work while preserving full-dataset summaries and permission boundaries. |
| One-year immutable caching also applies to unversioned embedded URLs | `internal/transport/web/handler.go:216`; raw HTMX/Alpine URLs in `web/pages/pages.templ:65`; raw font URLs in `web/assets/css/fonts.css` | Fingerprint all mutable dependencies or revalidate their unversioned aliases. Otherwise returning clients can retain an old dependency after deployment. This is source evidence, not an observed stale production browser. |
| A product derivative cache hit still opens the original object first | `internal/transport/web/media.go:26`, `:85`; `internal/application/commerce/media.go:140`; `internal/adapters/storage/s3.go:89` | Separate visibility authorization from object-byte acquisition so a hot derivative can avoid redundant origin I/O without bypassing deletion/visibility checks. |
| Sitemap source queries cap each content class at 5,000 | `cmd/website/seo.go:114` | Introduce complete pagination/sitemap-index before catalog growth reaches this limit. No current omission is established by this audit. |
| Failed seller queries already become unavailable states | Request projection wrappers; `internal/web/handlers/live_portal.go:2368` | Keep this safeguard and test it; do not introduce fake zero statistics during optimization. |
| Super Admin general Storefront editor is retired | `docs/storefront-editor.md:3`; `docs/category-publication-verification-2026-11-02.md` | Do not train operators to publish through a retired screen. Categories currently have a separate immediate-publication workflow. |
| Seller return list is read-only; shipping printout is address/tracking information | `web/selleradmin/portal_views.templ:521`, `:440`, `:463` | Explain company return decisions/private seller confirmations; distinguish parcel labels from purchased courier postage. |
| Active-looking docs still contain old role and hosting descriptions | Compare current four-role/November QA notes with `docs/role-workflows.md`, `docs/manual-test-cases.md`, older integration notes | Publish one current operations index and label historical evidence by date/revision. |

## How to use the register

Priority: **P0** blocks unrestricted real-money operations or the affected financial action. **P1** is needed before a broad public rollout; a limited pilot needs an explicitly assigned owner and mitigation for an open item. **P2** is a later improvement, justified by demand or measurements.

Work type: **G** = source-confirmed implementation/control gap; **V** = verify existing implementation or deployed configuration; **D** = document/training/copy improvement; **O** = measure and optimize; **B** = business/legal/accounting decision or approval. A V row does not mean the feature is absent. These are proposed task owners, not assumed staffing assignments.

Completion requires evidence against the stated criterion, an owner, the tested release/environment, and a review date. Keep personal data, credentials, recovery keys and bank documents out of public evidence. Financial controls may be implemented with the existing four roles and separate people; no new role taxonomy is implied.

The register contains **181 tasks across 19 areas**: 34 P0 gates, 133 P1 tasks and 14 P2 improvements. By work type: 6 implementation/control tasks, 101 verification tasks, 29 documentation/training tasks, 23 measurement/optimization tasks and 22 business/legal/accounting decisions. These counts describe proposed work, not 181 proven defects. The companion `production-readiness-backlog-2026-11-04.csv` includes assignment, status, target-date, evidence and dependency columns for tracking.

## 1. Release identity, scope and configuration

Owner: release engineer with the marketplace owner.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| REL-01 | P0 | V | Identify the actual Coolify service, origin, domain, running image digest/SHA and database migration level. Match them to the tested commit; record drift without exposing secrets. |
| REL-02 | P0 | V | Provision or verify isolated staging: separate database, Redis namespace/instance, bucket, auth settings, payment keys and webhook secrets. Demonstrate that QA cannot write or send messages to production users. |
| REL-03 | P0 | V | Verify production mode, canonical HTTPS URL, secure cookies, demo/local flags disabled, correct payment key mode and required provider configuration. A missing provider must fail honestly rather than simulate success. |
| REL-04 | P0 | V | Rehearse migrations and a compatible rollback from the current production schema. Include the refund-claim, earnings-snapshot and payment-recovery compatibility constraints; never resume older unsafe writers after a rollback. |
| REL-05 | P1 | V | Require successful checks on the exact release SHA and record deployment evidence. Ensure required GitHub checks actually prevent an unverified release; branch protection is an account configuration check. |
| REL-06 | P1 | B | Freeze release scope: currencies, serviceable geography, product categories, payment methods, refund limits, manual shipping/payout model and support hours. Publish the same scope in UI and manuals. |
| REL-07 | P1 | V | Test maintenance/checkout-disable behavior that preserves browsing, customer recovery and verified provider callbacks. Document who invokes it and how payment events are reconciled before reopening. |
| REL-08 | P1 | D | Create a release checklist with exact commands, environment identity checks, rollback triggers, monitoring window, responsible person and durable sign-off evidence. |

## 2. Privileged identity, permissions and security

Owner: backend/security engineer; Super Admin owns operational access reviews.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| SEC-01 | P0 | G | Add MFA for Admin and Super Admin. A password-only login cannot obtain privileged access through HTML, API, recovery, legacy aliases or an existing session that has not satisfied the new policy. |
| SEC-02 | P0 | G | Add audited enrollment/recovery, one-use recovery codes, safe factor replacement and session revocation. Rehearse losing the last Super Admin factor without weakening last-admin safeguards. |
| SEC-03 | P1 | V | Require recent authentication for role changes, bank-detail changes, payout decisions and sensitive exports. Verify session expiry, logout/all-session revocation and role-change revocation across transports. |
| SEC-04 | P0 | V | Test cross-account/cross-seller object access for orders, media, receipts, returns, attachments, exports and payment reviews. Altering IDs or legacy routes must never change ownership rules. |
| SEC-05 | P1 | V | Test CSRF/origin checks, stored/reflected XSS, SQL injection boundaries, safe redirects, CSV formula protection and error redaction across current forms/API endpoints. Preserve server-side validation. |
| SEC-06 | P1 | V | Verify production CSP, HSTS, framing restrictions, MIME sniffing and referrer policy with actual auth/payment integrations. Test permitted scripts and reject unintended origins; stage restrictive changes first. |
| SEC-07 | P1 | V | Exercise login/reset/verification/search/upload rate limits and lockout behind the real proxy. Verify trusted-client-IP handling so forwarded headers cannot bypass limits or lock everyone into one bucket. |
| SEC-08 | P1 | V | Verify managed secrets, least-privilege service credentials, rotation and log/screenshot redaction. Review Firebase, mail, object storage, database, Redis, payment and Coolify access independently. |
| SEC-09 | P1 | V | Threat-model media processing and uploads: decoded pixel limits, actual type checking, traversal, executable content, decompression bombs, attachment malware handling, timeouts and tenant authorization. |
| SEC-10 | P1 | V | Complete a bounded independent application security review against an appropriate OWASP ASVS baseline; close material findings and retain reproducible negative-test evidence. |
| SEC-11 | P1 | D | Define individual operator accounts, least-privilege assignment, offboarding, periodic access reviews and emergency access. Do not share a Seller login as a substitute for the retired staff feature. |

## 3. Checkout, stock and payment correctness

Owner: commerce/backend engineer with QA and the payments operator.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| PAY-01 | P0 | V | Execute a complete sandbox order on the release candidate: quote, address review, reservation, provider payment, verified capture, receipt, seller visibility and customer history. Match totals and references at every boundary. |
| PAY-02 | P0 | V | Run duplicate-click, refresh, browser-back and concurrent retry scenarios. Same idempotency key plus same request returns the same operation; changed request is rejected without double orders or charges. |
| PAY-03 | P0 | V | Test simultaneous purchase of the last unit, cart changes, reservation expiry and stock adjustment. Stock and reserved quantities stay valid with zero overselling or lost inventory. |
| PAY-04 | P0 | V | Exercise delayed, duplicate and out-of-order signed webhooks, invalid signatures, provider timeout, browser close and callback loss. Reconciliation converges without trusting browser-reported success. |
| PAY-05 | P0 | V | Verify capture after cancellation/reservation expiry and mismatched provider amount/currency/order. Exception review must hold fulfilment and payout until an authorized resolution is durable. |
| PAY-06 | P1 | V | Verify quote expiry, changed catalog price, promotions, unavailable seller, shipping policy revision and invalid PIN code between review and payment. Show an actionable changed-total/unavailable response. |
| PAY-07 | P0 | V | Reconcile multi-seller coupons, per-item rounding, commissions and shipping discounts against saved snapshots. Refunds reverse the original entries even after catalog/rule changes. |
| PAY-08 | P1 | V | Verify abandoned/failed/uncertain payment recovery UI, retry permission and customer wording. A pending payment must not invite an unsafe second charge or falsely claim success. |
| PAY-09 | P1 | V | Verify real provider-account activation, settlement destination, enabled payment methods, webhook alert recipient and test/live separation. Plan an explicitly authorized small live payment/refund drill before accepting general orders. |
| PAY-10 | P1 | D | Publish a payment mismatch investigation SOP: app order, provider order/payment, capture, bank settlement, refund and ledger references; identify when to hold, escalate and contact the customer. |

## 4. Refunds, seller earnings and manual payouts

Owner: finance operator with backend support and a separate approver.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| FIN-01 | P0 | G | Enforce distinct payout preparation/approval identities. The same account cannot approve its own batch or bypass the rule through an alternate Admin/Super Admin route. Preserve existing transaction/advisory locks. |
| FIN-02 | P0 | G | Record a verified payee/bank-details version, transfer amount/date, bank UTR/reference and protected evidence before a manual payout becomes paid. Settlement batch ID alone is not transfer evidence. |
| FIN-03 | P0 | G | Replace mock-payout semantics with a real manual-bank-transfer recording workflow and matching seller wording. Do not imply that a status button sends money; bind completion to evidence and approval. |
| FIN-04 | P0 | V | Test settlement batch creation/approval concurrency, duplicate decisions, refund/review changes after approval and payout failure. No ledger entry can be paid twice or released from a hold through a race. |
| FIN-05 | P1 | B | Approve payout eligibility, hold period, cadence, minimum amount, refund/chargeback reserve, negative-balance policy and shipping allocation. Keep unsupported charges out of seller promises. |
| FIN-06 | P0 | V | Exercise refund submit timeout, process crash, pending response, known/unknown reference and reconciliation failure. Preserve submission claims: uncertain outcomes cannot trigger another automatic provider POST. |
| FIN-07 | P1 | V | Test refunds after paid settlement, chargebacks, negative seller balances and legacy unallocated orders. Review historical amounts without recomputing them using current commission rules. |
| FIN-08 | P1 | D | Create daily payment/refund/bank/ledger reconciliation and month-end close procedures. Record unmatched items, aging, owner, resolution evidence and seller statement totals. |
| FIN-09 | P1 | V | Produce readable order/settlement exports and statements with saved discounts, commission, refunds, shipping, paid references and balances. Reconcile them to the accounting system. |
| FIN-10 | P1 | B | Confirm whether partial/item refunds and split-order cancellations are required for launch. If unsupported, state the limitation and define the multi-seller/customer escalation policy before trading. |

## 5. Shipping, fulfilment, returns and support

Owner: marketplace operations and Seller, with company decision authority documented.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| OPS-01 | P0 | B | Approve actual serviceable PIN codes, handling times, delivery estimates, shipping prices, remote-area exclusions and customer contact channel. Test the approved policy in checkout and seller instructions. |
| OPS-02 | P1 | D | Write seller dispatch SOP: verify paid/held state, pick correct variants, inspect, pack, book externally, enter valid carrier/tracking at shipment, and retain handover evidence. |
| OPS-03 | P1 | D | Clarify that the current printed shipping label contains parcel/address details and does not purchase postage or book a courier. Verify address legibility and print sizes. |
| OPS-04 | P1 | V | Test permitted fulfilment transitions, revision conflicts, wrong seller, missing tracking, invalid tracking URL, cancelled order and active payment hold. Seller updates remain explicitly seller-recorded. |
| OPS-05 | P0 | B | Approve returns/cancellation eligibility, windows, damaged/wrong item evidence, reverse-shipping responsibility, return address, inspection and refund authority. Match published policy to implementation. |
| OPS-06 | P1 | D | Explain the read-only Seller returns view and the private seller-confirmation response channel. Provide order/case context and identify that company staff make final customer-facing decisions. |
| OPS-07 | P1 | V | Verify customer case, private seller request, seller reply, company follow-up and customer reply end to end. Prevent exposure of internal notes or another seller's items/attachments. |
| OPS-08 | P1 | D | Define delivery failures, lost parcels, RTO, damaged parcels, non-responsive seller, chargeback and escalation handling. No manual support closure may manufacture a payment/courier event. |
| OPS-09 | P1 | V | Measure and expose operational queues: unprocessed paid orders, overdue dispatch, aged returns, open confirmations and stalled cases. Define owners and alert thresholds. |
| OPS-10 | P2 | B | Evaluate courier APIs/labels/tracking automation after manual operations are reliable and volume justifies it. Do not block the approved seller-booked model on this optional integration. |

## 6. Database, media backups and disaster recovery

Owner: infrastructure engineer with finance/privacy participation in restore drills.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| REC-01 | P0 | V | Verify a scheduled encrypted database backup actually runs, reaches independent storage and alerts on failure. Check restoration, not only file creation or successful upload. |
| REC-02 | P0 | V | Back up object media and any remaining local legacy uploads/maker portraits. Capture a manifest/version boundary that can be reconciled with database references. |
| REC-03 | P0 | V | Keep recovery keys separately, restrict access, define retention and test decryption with the designated recovery operator. Record who can restore when the normal administrator is unavailable. |
| REC-04 | P0 | V | Restore database and matching media into an isolated environment. Verify order totals, stock, ownership, roles, image availability, audit records and deletion state. Record measured recovery point and elapsed time. |
| REC-05 | P0 | B | Reapprove documented RPO 24 hours/RTO 4 hours against financial risk. If losing a day of orders is unacceptable, add more frequent/PITR recovery before taking that volume of trade. |
| REC-06 | P0 | V | Rehearse payment/refund reconciliation after restore. Preserve external provider truth and reapply privacy deletions/session cutoffs before checkout or email delivery resumes. |
| REC-07 | P1 | V | Verify offsite retention, at least the documented 30 daily recovery points, backup integrity, object versioning/deletion protection as appropriate and storage-cost alerts. |
| REC-08 | P1 | D | Write and rehearse VPS loss, storage failure, database corruption, Redis loss and key-loss procedures. Assign a drill schedule and record evidence after material schema/infrastructure changes. |

## 7. VPS/Coolify, network and runtime reliability

Owner: infrastructure engineer.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| INF-01 | P0 | V | Verify current VPS/Coolify configuration, security updates, SSH access, admin-console protection, firewall and domain ownership. Expose only required public services; keep database/Redis management private. |
| INF-02 | P0 | V | Verify HTTPS, certificate renewal, canonical www/http redirects, origin TLS and origin reachability controls. Test real browsers and legitimate crawlers; investigate this audit's 403 without weakening authentication. |
| INF-03 | P1 | V | Verify production PostgreSQL TLS hostname/CA validation, Redis authentication/private transport, storage TLS and least-privilege credentials. Local Compose credentials and tools must not become production defaults. |
| INF-04 | P1 | V | Set CPU/memory/disk limits and headroom for Go, image processing, PostgreSQL, Redis and backups. Alert on disk exhaustion, OOM, restart loops and connection pressure. |
| INF-05 | P1 | V | Test readiness/liveness, deployment grace period, in-flight request draining, worker shutdown and dependency startup failures. An unready instance must not receive checkout traffic. |
| INF-06 | P1 | V | Coordinate migrations and multiple app replicas/workers. Prove reservation cleanup, reconciliation, media deletion and outbox work stay replay-safe during deployments and restarts. |
| INF-07 | P1 | V | Verify production metrics-token protection, access logs, log rotation/retention and protected database/admin tools. Health responses must not leak provider diagnostics or credentials. |
| INF-08 | P1 | V | Test controlled dependency failure in staging: PostgreSQL, Redis, storage, mail, identity and Razorpay. Record truthful degraded behavior, retry policy and recovery. |
| INF-09 | P2 | O | Right-size server region and capacity using Indian customer latency, traffic, data location and cost measurements. Scale vertically or add replicas only after identifying the bottleneck. |

## 8. Performance baseline and release budgets

Owner: performance/frontend/backend engineers with QA.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| PERF-01 | P1 | O | Establish current cold/warm-cache baselines for home, catalog, search, product, cart, checkout, order history and both portals using representative real-sized data/media. Save traces and request waterfalls. |
| PERF-02 | P1 | O | Measure LCP, INP and CLS in real-user data where available, segmented by route/device/network; supplement low traffic with repeatable lab tests. Do not report sampled event timing as field INP. |
| PERF-03 | P1 | B | Adopt initial budgets: mobile/desktop field p75 LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1; agree route/API latency and availability targets for the actual expected traffic. |
| PERF-04 | P1 | O | Break slow LCP into server response, resource discovery, download and render delay. Optimize the largest observed component and remeasure rather than adding preloads indiscriminately. |
| PERF-05 | P1 | O | Test mid-range Android, iPhone Safari and constrained mobile networks; capture low-memory/CPU behavior, not only wide desktop emulation. Run benchmarks without concurrent builds/load tests. |
| PERF-06 | P1 | V | Add regression budgets for critical asset bytes, failed requests, LCP/CLS lab proxies, API p95 and image dimensions to suitable CI/staging checks. Preserve raw evidence and investigate variance. |
| PERF-07 | P1 | O | Define and run an isolated browse/search plus authenticated checkout workload, including bursts, long duration and contention. Existing 10→50-user public smoke is only a starting point. |
| PERF-08 | P1 | V | Repair/reproduce the local fixture startup timeout and application empty-response condition before interpreting local browser failures. Record actual successful browser results on the release candidate. |

Google's performance thresholds are field targets evaluated at the 75th percentile; lab scores are diagnostics, and small samples are not proof of field performance. See [Google Core Web Vitals](https://developers.google.com/search/docs/appearance/core-web-vitals) and [LCP diagnosis](https://web.dev/articles/optimize-lcp).

## 9. Product photographs, video and storage delivery

Owner: media/backend and frontend engineers, with sellers supplying truthful assets.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| IMG-01 | P1 | V | Verify current derivative generation for seller products, storefront/category uploads and maker images. A large uploaded original must deliver an appropriately small listing image without overwriting the zoom original. |
| IMG-02 | P1 | O | Sample actual catalog assets at card, gallery, hero and high-DPR sizes. Tune encoding quality/resolution based on visible craft detail, download bytes and processing cost. Static assets include WebP; current product derivatives use JPEG. Add newer encoding only when quality/bytes/CPU evidence supports it. |
| IMG-03 | P1 | V | Verify `srcset`/`sizes` against actual rendered widths and mobile art direction. Mobile must not download both mobile and desktop hero photographs or oversized originals for tiny thumbnails. |
| IMG-04 | P1 | V | Keep the first meaningful hero/product image eagerly discoverable with suitable priority; lazy-load below-fold media. Preserve aspect ratios/intrinsic dimensions so delayed media does not shift controls. |
| IMG-05 | P1 | V | Test derivative cache hit/miss, concurrent cold requests, eviction/restart and multi-replica behavior. Limit bytes, entries, CPU and decoded pixels; deduplicate or prewarm expensive work if measurements justify it. |
| IMG-06 | P1 | V | Verify public media cache keys, immutable/versioned URLs or correct invalidation, ETag/304 and HEAD behavior. A missing/unavailable object must not become a long-cached broken image. |
| IMG-07 | P1 | V | Verify public product visibility and private avatar/attachment authorization before transformation or cache lookup. CDN/media caches must not expose hidden/private objects across roles. |
| IMG-08 | P1 | V | Test image replacement, original zoom, orientation, transparency, invalid bytes, oversized dimensions and storage outage. Show usable fallback/retry while retaining saved records and uploads. |
| IMG-09 | P1 | V | Verify video poster/fallback, no unnecessary autoplay download, range/HEAD responses, unsupported codecs, mobile playback and keyboard controls. Serve a real photograph if playback is unsupported. |
| IMG-10 | P2 | O | Evaluate CDN/object offload and background derivative creation after measuring origin/media CPU, bandwidth, misses and user latency. Keep originals backed up and derivatives disposable. |
| IMG-11 | P2 | O | Remove redundant S3 object opens on a valid hot derivative, retaining fresh visibility/deletion authorization. Instrument storage calls and prove repeated cached reads avoid opening unused original bytes. |

## 10. CSS, JavaScript, fonts and interaction cost

Owner: frontend engineer; preserve accepted tokens/components and server rendering.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| FE-01 | P1 | O | Inventory CSS/JS/font bytes by route and remove unused or duplicate payloads after coverage analysis. Admin/seller decoration should not burden the public purchase path. |
| FE-02 | P1 | V | Verify generated/minified assets match sources on Linux CI and release images. Add checks for relevant generated CSS/JS/image manifests, not only templ/sqlc output. |
| FE-03 | P1 | G | Fix one-year immutable caching of unversioned HTMX/Alpine, font and other mutable aliases. Fingerprint every dependency or serve aliases with revalidation; prove a dependency change reaches returning browsers after deployment. |
| FE-04 | P1 | V | Verify Brotli/gzip negotiation at the actual edge/proxy for HTML/CSS/JS/SVG/JSON, correct `Vary`, and no double compression. Already-compressed photographs/video should not consume needless CPU. |
| FE-05 | P1 | O | Review critical render-blocking CSS/fonts and preloads. Serve only necessary font families/weights/subsets, use usable fallbacks and avoid duplicate icon-font downloads. |
| FE-06 | P1 | V | Load Razorpay and Firebase code only where needed. Verify script failure/offline behavior and third-party performance impact while preserving actual sign-in/payment functionality. |
| FE-07 | P1 | O | Profile search suggestions, filters, carousels, shared event listeners and DOM updates. Cancel obsolete requests, prevent duplicate listeners after HTMX swaps and eliminate observed long tasks/layout thrashing. |
| FE-08 | P1 | V | Verify normal links/forms, no-JavaScript fallbacks, browser history, focus restoration and retriable enhancement failures. The server remains authoritative for shopping state. |
| FE-09 | P2 | O | Consolidate repeated style overrides and obsolete assets after route/ownership verification. Preserve accepted designs; do not perform a cosmetic rewrite as a performance assumption. |

## 11. PostgreSQL, caching, workers and scale

Owner: backend/database engineer.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| DB-01 | P1 | O | Profile slow/high-frequency SQL with realistic catalog/orders/seller counts and safe query statistics. Capture EXPLAIN ANALYZE in staging for the highest-impact reads. |
| DB-02 | P1 | O | Reduce route-unrelated seller/admin projection queries. Load the active workspace and required summary data rather than every table, preserving error propagation and full-scope counts. |
| DB-03 | P1 | O | Audit joins, substring search, filters, counts, sorts and ownership predicates for suitable indexes. Add indexes only from demonstrated query plans; measure write/storage overhead. |
| DB-04 | P1 | V | Verify bounded pagination, stable tie ordering and owner filters before pagination. Benchmark deep OFFSET; use cursor pagination where large histories justify it. |
| DB-05 | P1 | V | Bound database connections, query deadlines, lock timeouts and transactions against instance size. Measure pool wait/lock contention and avoid exhausting connections with background jobs. |
| DB-06 | P1 | V | Test cache keys/invalidation for price, availability, promotions, category visibility and seller suspension. Never cache one customer's cart/account/checkout output for another visitor. |
| DB-07 | P1 | V | Verify search cache misses, stampede protection, stale-index hydration and PostgreSQL fallback. Clarify whether Meilisearch is actually used by the current customer path before operating an unnecessary service. |
| DB-08 | P1 | V | Monitor outbox age, failed/poison jobs, retries, reconciliation lag, reservation cleanup and orphan-media cleanup. Each job needs bounded batches, replay safety and an operator recovery action. |
| DB-09 | P1 | V | Verify PostgreSQL autovacuum, growth, bloat, WAL/backup disk use and Redis memory/eviction policy. Session eviction must not unpredictably compete with disposable cache growth. |
| DB-10 | P2 | O | Evaluate aggregate caches/read models, PITR tuning, replicas or search-engine expansion only after traffic and query evidence. Preserve PostgreSQL business authority. |
| DB-11 | P2 | O | Measure featured-popularity refresh at the 15-minute cache boundary. Its advisory lock serializes refresh callers; move aggregation to a background job/read the prior snapshot only if actual wait time or burst load warrants it. |

## 12. Technical SEO and discoverability

Owner: SEO/content owner with backend/frontend support.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| SEO-01 | P1 | V | Crawl the current canonical public site and sitemap. Verify 200/indexable pages, real 404/410 removals, 301 equivalents, canonical host and expected redirect chains. Resolve access/WAF failures for legitimate crawlers. |
| SEO-02 | P1 | V | Retest robots/noindex rules: public catalog/content stays discoverable; accounts, cart, checkout, admin, seller workspaces, previews and staging remain excluded. Robots is never a substitute for authorization. |
| SEO-03 | P1 | V | Verify sitemap completeness against approved products, active sellers, visible categories and published pages, accurate `lastmod` and failure behavior. Exclude hidden/deleted/private pages; paginate sitemap files when size requires it. |
| SEO-04 | P1 | V | Verify unique useful titles/descriptions, one clear main heading, canonical URLs, breadcrumbs and social cards for product/category/maker/brand/content templates. Check escaping and image availability. |
| SEO-05 | P1 | V | Validate Product/Offer, Organization/WebSite and Breadcrumb structured data against rendered facts. Price, currency, stock and seller details must match actual approved data; do not invent reviews or ratings. |
| SEO-06 | P1 | B | Define variant URL/canonical policy and indexable category/editorial collection pages. Add product-variant markup only when the catalog and URLs support it. |
| SEO-07 | P1 | O | Review filter/sort/search/pagination URL combinations for crawl traps. Preserve useful server-rendered discovery and page canonicals; selectively control low-value combinations based on crawl evidence. |
| SEO-08 | P1 | D | Verify Search Console ownership/sitemap, review current indexing/rich-result/CWV reports and assign follow-up. Historical accepted sitemap/indexing evidence is not a guarantee of current ranking. |
| SEO-09 | P1 | B | Replace QA-labelled maker/content and thin descriptions with accurate real seller/product copy. Verify materials, dimensions, care, provenance and any handmade/organic/sustainability claims. |
| SEO-10 | P1 | V | Verify crawlable internal links from home→category→product and maker pages, mobile navigation and non-JavaScript discovery. Avoid important products accessible only through search forms. |
| SEO-11 | P2 | B | Evaluate Merchant Center/feed eligibility, shipping/return policy data and local brand profiles using real business information. Add feeds only with a process to keep price/stock synchronized. |
| SEO-12 | P2 | B | Plan useful craft guides and category content from actual search demand. Measure impressions, qualified traffic and conversion; do not buy spam links or promise first-place rankings. |
| SEO-13 | P2 | O | Remove the silent 5,000-row limit per sitemap content class with complete pagination or a sitemap index before approaching that scale. Consider a server-side revision cache/validator; retain retryable 503 on failed rebuild rather than publishing partial data. |

Google documents ecommerce navigation/markup and the crawl cost of faceted URLs: [ecommerce guidance](https://developers.google.com/search/docs/specialty/ecommerce), [product structured data](https://developers.google.com/search/docs/appearance/structured-data/product), [faceted navigation](https://developers.google.com/crawling/docs/faceted-navigation). Apply crawl restrictions carefully: blocking a URL can prevent Google from reading its noindex directive.

## 13. Storefront design, shopping UX and responsiveness

Owner: frontend/UX engineer with QA and marketplace content owner.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| UX-01 | P1 | V | Review every critical customer route at 320/390/768/1024/1440+ widths, portrait/landscape and long content. No clipped controls, full-page horizontal overflow, hidden totals or missing purchase actions. |
| UX-02 | P1 | V | Verify product price/variant/availability, seller identity, delivery estimate, returns, materials/dimensions and primary action before purchase. Surface charges and relevant restrictions before payment. |
| UX-03 | P1 | V | Verify search, category, filters, sort, pagination, selected chips, no-results recovery and Back/Forward state. Mobile filtering must preserve focus, chosen filters and surrounding navigation. |
| UX-04 | P1 | V | Exercise cart/wishlist quantity, stock changes, move/remove, merged carts and duplicate actions. Visible state commits only after server success and stays synchronized across controls. |
| UX-05 | P0 | V | Verify checkout address entry, autofill, saved details, validation, review/edit, totals and payment uncertainty at narrow widths. Retain inputs after recoverable failures and prevent duplicate submission. |
| UX-06 | P1 | V | Verify loading/empty/unavailable/error/disabled/success states consistently across commerce and portals. Distinguish no data from a failed read, and give an actionable retry. |
| UX-07 | P1 | V | Test real touch and on-screen keyboards: menus, swipe, galleries, filters, file pickers and sticky controls. Responsive screenshots alone do not verify touch gestures. |
| UX-08 | P1 | V | Verify cookies/permission prompts, banners, sticky headers and mobile bottom controls never obscure focus, form errors or the pay action. Avoid unsolicited push permission on first visit. |
| UX-09 | P1 | V | Review privacy/security/account controls, login return destinations, reset/verification links and guest/session-expired behavior. Auth should return customers to the intended safe task. |
| UX-10 | P2 | O | Reduce unnecessary animation, decorative weight and duplicated components only where observed friction or loading cost justifies it. Preserve accepted imagery, brand and operational typography. |

## 14. Accessibility and inclusive operation

Owner: frontend/accessibility reviewer with manual QA.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| A11Y-01 | P1 | V | Audit WCAG 2.2 AA across representative templates, states and dialogs using automated and manual checks. Record exceptions; an axe pass alone is not conformance. |
| A11Y-02 | P1 | V | Complete shopping and seller/admin tasks with keyboard only: visible focus, skip links, logical order, no traps, Escape dismissal and correct focus return. |
| A11Y-03 | P1 | V | Test screen-reader names, landmarks, headings, tables, status announcements, price/stock updates, validation and payment progress with NVDA and VoiceOver on representative flows. |
| A11Y-04 | P1 | V | Measure actual text/non-text contrast and non-color status cues. Inspect overlays, disabled controls, muted guidance and validation against the accepted palette. |
| A11Y-05 | P1 | V | Test browser zoom/text enlargement and 320px reflow, long names/references and localized currency. Preserve full table values/actions in mobile record layouts. |
| A11Y-06 | P1 | V | Maintain the project's 44px primary-control/touch-target convention. Check WCAG minimum-target/spacing exceptions accurately; do not label every sub-44px inline link an AA violation. |
| A11Y-07 | P1 | V | Test reduced motion, autoplay pause, focus/hover carousel behavior, drag alternatives and video controls. State changes must stay understandable when motion is reduced. |
| A11Y-08 | P1 | V | Verify persistent labels, accessible errors, autocomplete/input purposes and password-manager/paste support. Authentication and MFA must have usable non-memory/puzzle alternatives. |

The conformance reference is [WCAG 2.2](https://www.w3.org/TR/WCAG22/). Its AA target-size rule and the project's larger 44px design convention are different requirements.

## 15. Seller handbook and in-product guidance

Owner: marketplace operations/content designer, reviewed by actual sellers.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| SELL-01 | P1 | D | Publish a current Seller quick start: credentials, owned-business requirement, pending/active/suspended states, verification/approval, recovery and where to ask for help. |
| SELL-02 | P1 | D | Document product setup with exact field meanings, valid photographs, variants/SKUs, INR amounts, stock, moderation, rejection correction and publication. Show the actual current screens. |
| SELL-03 | P1 | D | Explain available vs reserved vs sold stock, low-stock threshold, adjustment reason/history, archive vs delete and pending-order restrictions using worked examples. |
| SELL-04 | P1 | D | Provide a paid-order dispatch walkthrough including payment holds, next permitted stage, tracking and evidence. Explain which updates customers see and which are private. |
| SELL-05 | P1 | D | Provide a return/support walkthrough with read-only return status, private seller confirmation, attachments, deadlines, company decision authority and escalation. |
| SELL-06 | P1 | D | Explain earnings: merchandise gross, item discount, commission, refund reversal, shipping treatment, open balance, approved settlement and confirmed bank transfer. Use reconciled example numbers. |
| SELL-07 | P1 | D | Replace unspecified payout schedules/fees in terms with approved actual policy and a help link. Distinguish an expected payout from evidence that the bank transfer completed. |
| SELL-08 | P1 | D | Add contextual help to difficult fields and actions, with prerequisites, effects and retry advice. Keep instructions near the task rather than requiring users to find an engineering document. |
| SELL-09 | P1 | V | Have a representative seller complete onboarding, product entry, stock update, dispatch, support reply and earnings review from the manual without developer coaching; fix observed ambiguities. |

## 16. Super Admin/Admin handbook and content governance

Owner: marketplace owner and operations lead, supported by engineering.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| ADM-01 | P1 | D | Publish one current four-role permission matrix and operations home. Retire or banner historical eight-role/team/finance/support-portal instructions while retaining dated QA evidence. |
| ADM-02 | P1 | D | Document account creation, role replacement, session revocation, seller-profile repair, suspension and protected deletion. Explain last-Super-Admin and unfinished-order/return safeguards. |
| ADM-03 | P1 | D | Document seller approval, product moderation, prohibited content, inventory exceptions and appeals with evidence, authority, customer effect and audit reason. |
| ADM-04 | P1 | D | Document category add/edit/hide/order and product assignment using current routes. State prominently that category saves publish immediately; the retired general Super Admin editor is not available. |
| ADM-05 | P1 | B | Decide how other homepage/policy/navigation changes are authorized while the general publisher is retired. Preserve existing drafts/releases; do not expose retired mutations as a documentation workaround. |
| ADM-06 | P1 | D | Publish payment-review, refund and manual-settlement SOPs with permissions, second approval, reconciliation, retry restrictions and protected evidence. Separate support resolution from money movement. |
| ADM-07 | P1 | D | Define daily queues, weekly access/reconciliation reviews, monthly close/restore checks and on-call escalation. Every queue has an owner, aging threshold and handover procedure. |
| ADM-08 | P1 | D | Add an operator glossary for product approval, seller status, payment status, fulfilment status, return status, refund status and settlement status. Never use one generic status as the meaning of all seven. |
| ADM-09 | P1 | V | Conduct an Admin/Super Admin training rehearsal covering a real-looking exception scenario in staging. Require operators to find evidence, follow the manual and complete the correct audit trail. |
| ADM-10 | P2 | D | Add searchable help, short annotated walkthroughs and versioned release notes. Translate only where user research shows a need and keep examples synchronized with actual UI. |

## 17. Communication, mail, notifications and analytics

Owner: operations and backend engineer; privacy owner reviews tracking.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| COM-01 | P1 | V | Verify production sender/domain, SPF/DKIM/DMARC alignment as applicable, reply inbox ownership and reset/verification/order/dispatch/refund mail links. Historical provider acceptance is not current inbox delivery. |
| COM-02 | P1 | V | Verify durable retry, delivery status, bounce/complaint handling and suppression. Critical delivery failures must reach an operator without leaking action tokens or customer data. |
| COM-03 | P1 | V | Keep staging recipients isolated and transactional vs marketing preferences separate. Test that QA cannot email real customers or send production push notifications. |
| COM-04 | P1 | V | Test browser/device push opt-in, permission refusal, token/session ownership, logout and invalid subscription cleanup. Keep payloads generic and verify actual device receipt. |
| COM-05 | P1 | D | Approve customer/seller message templates for pending vs captured payment, refunds, delays, cancellations and support. Include correct contact/escalation details and no unsupported delivery promises. |
| COM-06 | P1 | V | Implement or verify privacy-appropriate error/performance telemetry. Optional one-visit diagnostics alone do not provide a representative production monitoring baseline. |
| COM-07 | P1 | B | Define analytics events for discovery→product→cart→checkout→capture and refunds. Exclude PII, deduplicate purchase events and use server-confirmed payment state for revenue. |
| COM-08 | P2 | O | Use measured search failures, mobile abandonment, payment failure and seller task completion to prioritize subsequent UX work. Consent and retention decisions govern any replay/tracking tooling. |

## 18. Legal, accounting, policies and truthful content

Owner: business owner with qualified India legal/accounting review.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| POL-01 | P0 | B | Confirm the marketplace legal entity, merchant/payment relationship, seller contracts, contact/grievance details, dispute handling and applicable consumer/ecommerce obligations for launch. Publish approved factual details. |
| POL-02 | P0 | B | Confirm GST/TCS/TDS, invoices/receipts, seller tax details, commission taxes and accounting treatment for this business model. Ordinary receipts are the current scope, not proof tax requirements are satisfied. |
| POL-03 | P1 | B | Approve privacy notices, terms, return/refund/shipping policy, seller agreement and consent records. Ensure actual workflows enforce the stated commitments and keep effective versions. |
| POL-04 | P1 | B | Update data/processor inventory for actual VPS/Coolify, database/storage, Firebase, Resend, Razorpay and analytics providers, regions, contracts and incident contacts. |
| POL-05 | P1 | B | Approve exact retention periods, deletion exceptions and processor deletion/retry responsibilities. Verify deletion queues and that restored backups do not silently reactivate deleted accounts. |
| POL-06 | P1 | B | Review phased applicability of India's DPDP framework against the actual launch date and processing activities; create dated obligations/tasks rather than assuming every provision starts together. |
| POL-07 | P0 | B | Verify category-specific requirements before enabling regulated goods, such as food, cosmetics or organic claims. Remove unsupported certifications, reviews, origin and sustainability assertions. |
| POL-08 | P1 | D | Publish complaint, IP/takedown, prohibited goods, fraud/chargeback and breach-response procedures with accountable owners and reviewed timelines. |

This audit does not make a compliance determination. The tasks require current professional review. Authoritative starting points include the [Department of Consumer Affairs](https://consumeraffairs.nic.in/) and [MeitY's DPDP Rules publication](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa). The published DPDP Rules provide different commencement periods; the applicability review must use the operative notifications and launch date.

## 19. QA coverage, observability and steady-state operation

Owner: QA/release engineer and on-call operator.

| ID | Priority | Type | Task and completion criterion |
|---|---|---|---|
| QA-01 | P0 | V | Maintain a release matrix across guest/customer/seller/admin/superadmin and happy/error/retry/authorization paths. Evidence names the SHA, environment, dataset and test outcome. |
| QA-02 | P1 | V | Verify Go unit/integration tests, race-sensitive financial tests, vet, architecture/sqlc/templ checks, CSS/JS builds, vulnerability/secret/container scans and required CI configuration. |
| QA-03 | P1 | V | Expand browser coverage beyond the configured customer-layout subset: full purchase/recovery, seller operations, role controls, return coordination, finance and category publication with safe isolated fixtures. |
| QA-04 | P1 | V | Run Chromium/Firefox/WebKit and physical mobile smoke checks on the release candidate. Include keyboard, zoom, screen reader, throttled network and third-party popup/payment failure. |
| QA-05 | P1 | V | Add or verify external uptime plus operational alerts for checkout errors, webhook failures, uncertain refunds, payment reviews, queue lag, backups and resource saturation. Test alert delivery to the actual on-call person. |
| QA-06 | P1 | V | Correlate request IDs with order/payment/refund/job references in structured logs. Redact secrets/PII and restrict log/metrics access; dashboards show actionable aging and failure rates. |
| QA-07 | P1 | D | Write incident playbooks for payment inconsistency, lost access, storage outage, failed deployment, database exhaustion and suspected breach. Rehearse one incident and record time to diagnose/recover. |
| QA-08 | P0 | V | Conduct a limited pilot with real operators and an explicitly authorized payment/refund process only after blocking gates close. Reconcile orders, stock, provider state, bank entries and seller balances before widening access. |
| QA-09 | P1 | B | Assign support coverage, incident response, maintenance windows, vendor renewals, budget alerts and backup ownership. A named backup operator covers absence of the primary engineer. |
| QA-10 | P2 | O | Review weekly business/reliability metrics and monthly capacity/security/recovery results. Convert observed issues into owned tasks rather than a permanent unprioritized polish list. |

Razorpay uses asynchronous delivery, duplicate events and retries; test duplicate and out-of-order events and monitor disabled/failed webhooks. See [Razorpay webhook best practices](https://razorpay.com/docs/webhooks/best-practices). Use [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/) to structure security verification.

## Operator-manual format and example procedures

The tasks above include writing manuals, not asserting that manuals have now been implemented. Recommended deliverables: a current operations home, Seller handbook, Admin/Super Admin handbook, finance reconciliation handbook and incident/recovery runbook. Link their relevant sections from the actual task screens.

Every procedure should have: role, purpose, prerequisites, exact current navigation, numbered actions, what the action changes, expected confirmation, evidence to retain, safe retry, common errors, escalation owner and last-verified release/date. Use redacted current screenshots at phone and desktop widths. Explain irreversible or financial consequences in the action itself, without filling routine screens with engineering details.

| Procedure | Required instructions and completion evidence |
|---|---|
| Seller first session | Sign in; check business ownership/status; complete approved information; know what approval unlocks; find support and recovery. An incomplete role/profile is repaired by authorized role reassignment, not by attaching an unrelated business. |
| Seller first product | Enter accurate product/category, SKU/variants, INR price, available stock and real media; submit through current moderation; resolve actual feedback; confirm public listing after approval. |
| Seller daily order queue | Inspect payment/hold state; process only eligible paid items; pack the correct seller-owned lines; book shipping outside the app; enter carrier/tracking and next permitted state; retain handover evidence. |
| Seller returns/support | Read return status; respond through the private confirmation when requested; supply permitted evidence; company replies to customer and decides the case. Closing a conversation does not issue money. |
| Seller earnings | Match order lines, discounts, commission and refunds to open ledger; understand approved vs bank-paid; report a mismatch with order/batch/bank references through a private channel. |
| Admin daily operation | Review paid-order dispatch delays, payment-review cases, uncertain refunds, return/support aging, moderation and failed communications; assign each exception to an operator. |
| Super Admin identity change | Verify identity and business references; choose the current role; explain session sign-out and seller activation effects; supply audit reason/confirmation; verify result and recovery access. |
| Category change | Use Products/Categories current controls; preview the intended information; understand that save publishes immediately; preserve stable URLs/product assignments; verify public discovery and prior media. |
| Manual payout | Prepare reconciled eligible batch; separate approver verifies seller recipient/amount/holds; authorized person transfers via bank; record protected bank evidence; reconcile before paid completion; communicate actual outcome. This requires the FIN controls above before routine use. |
| Refund uncertainty | Find the original record/provider reference; inspect reconciliation state; hold related payout/fulfilment where required; escalate uncertain/multiple matches. Do not re-POST a refund just because no response or match was seen. |
| Restore/incident | Identify affected release/time; preserve evidence; restrict affected operations; restore/reconcile using the tested runbook; replay deletions and verify provider events before reopening; communicate through approved channels. |

### Operational cadence to document

| Cadence | Seller | Admin/Super Admin/company |
|---|---|---|
| Daily | Paid/held order queue, dispatch due, inventory accuracy, requested confirmations and return status | Payment/refund exceptions, fulfilment/support aging, moderation, communications failures, backup/alert health |
| Weekly | Stock reconciliation, listings/media quality, unsettled earnings and outstanding cases | Bank/provider/ledger reconciliation, seller performance/holds, access changes, failed jobs, SEO/CWV issues |
| Monthly | Settlement statement and policy changes | Accounting close, access review, capacity/cost review, dependency updates, recovery evidence and vendor/secret lifecycle |
| After significant release/incident | Follow updated instructions and report anomalies | Exact-SHA smoke, payment/event reconciliation, rollback readiness, updated manual/screenshots and lessons learned |

## Measurement and acceptance matrix

The following engineering budgets are proposals, not measured current results or contractual guarantees. Validate them against expected peak traffic and available infrastructure.

| Area | Proposed evidence/acceptance |
|---|---|
| Real-user performance | LCP ≤2.5 seconds, INP ≤200 ms, CLS ≤0.1 at field p75, segmented by device/template where sample volume permits |
| Backend browse/search | Start with existing smoke thresholds p95 <500 ms, p99 <1 second, failed requests <1%; specify load mix/data size, exclude or measure provider waits separately, agree stricter/looser route targets explicitly |
| Stock/payment correctness | Zero overselling, double settlement, duplicate refund submission or cross-seller data access in concurrency/replay fixtures; all uncertainty becomes a recoverable explicit state |
| Media | Correct responsive candidate at 320/390/tablet/desktop and DPR 1/2; bounded decode/cache resources; warm/cold miss measurements; original zoom preserved; private media stays private |
| UI | No lost form data or blocked core action at supported widths; keyboard completion/focus restoration; truthful empty/unavailable states; no unresolved serious/critical automated findings plus manual review |
| SEO | Public canonical sitemap pages return intended status/metadata; valid factual markup; private/staging excluded; legitimate bots can fetch public pages; no ranking promise |
| Recovery | Verified DB-plus-media restore within approved RPO/RTO with measured timestamps and post-restore financial/privacy reconciliation |
| Operations | Seller and Admin can complete representative exception tasks using current manuals without developer coaching; finance evidence reconciles to bank/provider records |
| Observability | Alert is actually received and investigated; dashboards distinguish availability, correctness and processing lag; logs provide identifiers without secrets |

Test coverage should cross route types (home/catalog/search/product/cart/checkout/orders/account/seller/admin), roles, viewport sizes, browser engines, slow/offline/dependency failure, empty/large/long-text data, delayed media, concurrent edits and payment retries. Do not multiply every dimension into an impractical full Cartesian suite: use risk-based combinations plus focused negative/concurrency tests for financial and authorization boundaries.

## Recommended execution order

1. Reconcile current deployment/configuration and documentation; identify owners and confirm the release scope. Preserve the current stack and accepted designs.
2. Close privileged MFA, payout evidence/separation, backup/restore and legal/accounting policy gates. Continue application verification in isolated staging.
3. Rehearse the complete payment/refund/stock/settlement and seller dispatch/support lifecycle on the exact candidate. Confirm that all failure states are operable.
4. Establish current performance baselines; address measured query, cache, media and loading bottlenecks. Complete SEO crawl, accessibility and physical-device checks.
5. Finish current manuals and train operators; execute an approved limited pilot and reconcile it; widen availability only when blocking evidence is complete.
6. Schedule ongoing monitoring, maintenance and recovery drills. Evaluate optional courier/payout automation, expanded search, additional currencies or large architectural changes separately when demand justifies them.

No additional skill download is required for this assessment. Existing skills/tools are sufficient for the work identified. Remaining evidence needs working isolated test infrastructure, deployment/monitoring access, representative seller assets and business/legal/accounting decisions—not another generic skill package.
