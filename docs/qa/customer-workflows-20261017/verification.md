# Customer workflow verification — 17 October 2026

## Changes

- Customer order history filters the complete owner-scoped history before server pagination (20 records per page). Counts, result ranges, page clamping and Back to orders retain the relevant list state. The existing JSON order-list contract remains unchanged.
- Legacy payment-state URLs cannot claim payment success. They open the owned order confirmation or neutral payment activity, with 404 responses for unknown or foreign orders. Confirmation separates recorded payment and parcel progress, including partial deliveries and refunds after delivery.
- Customer profile, address, preference, review, support, privacy-request and order-action forms reuse shared validation, pending indicators, duplicate guards, retained errors and unsaved-change protection. Session expiry retains the current document's entries and explains reauthentication. Private drafts are not stored in browser storage.
- Checkout relies on the existing order transaction to save its address once. An interrupted response can retry the same order without creating another address or order. Unavailable payments explain that the cart is saved.
- Order help carries a validated owned reference into support. A partial support upload opens the saved conversation, preventing recovery from creating another request. Address deletion, unpaid cancellation and eligible privacy deletion use shared confirmation.
- Wishlist actions wrap at 320px, retain every control and contain the entire photograph. Notification inbox and preference links have distinct labels. URL-backed order/return filters use navigation semantics.
- The customer order workspace contains its live-update controls and queue together, keeping the desktop queue beside the sidebar. Refreshed forms regain their shared confirmation/validation handlers.

## Isolated environment

All business mutations use PostgreSQL `customer_workflows_20261017`, cloned from the existing disposable `storefrontqa` dataset, and Redis database 5 on the existing local QA services. The current-source native application uses `127.0.0.1:18083` with explicitly enabled deterministic local payment. A second isolated application at port 18084 has payment credentials absent and object storage intentionally unavailable.

Registered customers, product variants, queues, support uploads, returns, refunds, security changes and privacy deletion are synthetic QA fixtures only. Successful profile/support uploads use the existing local Garage bucket with generated private object keys. SMTP/Resend and remote search are disabled; the database search fallback is exercised. No live payment or external email is sent. Temporary QA application processes were stopped after verification; the isolated database, Redis data and synthetic media remain available for a repeat run. Task-created temporary binaries were removed.

The ordinary Docker preview at port 8080 was already failing its PostgreSQL connection. This task does not reset its database, rebuild that preview or deploy production. Codebase-memory calls failed with `Transport closed`; relevant source was inspected directly.

## Verification

The complete Chromium suite passed 11 scenarios before the final bounded order-layout and refund-history corrections. It covers order pagination/counts/ownership, wishlist and Quick view, explicit variants, cart quantities, checkout correction, payment capture/pending/failure/retry/cancellation, seller delivery, receipt/PDF, verified reviews, returns, support uploads/replies, private attachment isolation, profile photos, address recovery, consent/preferences, password/CSRF/session revocation, unavailable integrations, stock changes, uncertain checkout completion, session-expiry draft recovery, operations-gated local refunds, privacy requests and deliberate QA-account deletion.

Firefox and WebKit responsive matrices passed. Native selection, failed Quick view reads, retry, nested Escape behavior and focus restoration passed in Chromium, Firefox and WebKit. The Chromium matrix checks 320, 390, 768, 1024 and 1440px; secondary engines check 390 and 1440px. The matrix also checks landscape, reduced motion, a 720px reflow equivalent, JavaScript errors and axe checks on the order page.

`go test ./...` passed with isolated projection integration tests enabled. Relevant Go suites passed again after subsequent template changes. Templ generation, the existing CSS build, JavaScript minification/syntax checks and the strict premium UI audit passed. The audit report has zero findings. Six seller route/viewport checks passed without JavaScript errors.

Final-source Chromium regressions passed three scenarios: desktop order placement plus confirmation after live refresh, the complete responsive route matrix including Privacy center, and operations-gated refund with all recorded delivery steps retained. WebKit passed its final responsive/keyboard checks. Firefox keyboard checks and its isolated final responsive rerun passed (47.5 seconds for the scenario). One concurrent Firefox run had timed out waiting for startup scripts; the test now uses the navigation-aligned 45-second startup budget and reports the URL and initialization state on failure.

The final `go test ./web/pages` passed after the refund-history correction. The final binary built successfully, JavaScript syntax checks passed, and `git diff --check` reported no whitespace errors.

Screenshots were visually inspected at desktop and narrow widths: [orders at 1440px](orders-1440.png), [orders at 390px](orders-390.png), [wishlist at 320px](wishlist-320.png), [Quick view at 390px](quick-view-390.png), [checkout at 390px](checkout-390.png), and [support at 390px](support-390.png).

## Practical limits

The 720px viewport exercises reflow equivalent to 200% of a 1440px desktop, not native browser zoom. Physical-device virtual keyboards, assistive-technology output, actual carrier scans, production payment webhooks and external email delivery are not certified by these checks. Earlier browser runs exposed test-selector and asynchronous-navigation assumptions; those were corrected without weakening business assertions. The real responsive defects were wishlist action overflow and the injected order controls changing desktop grid placement.

## Repeating the guarded suite

Use the dedicated QA services and the two application modes above. The suite refuses mutation against another database or non-loopback application port. The failure-mode test additionally requires the explicit port-18084 environment value.

```powershell
$env:WEEVCRAFTS_BASE_URL='http://127.0.0.1:18083'
$env:WEEVCRAFTS_QA_DATABASE='customer_workflows_20261017'
$env:WEEVCRAFTS_FAILURE_BASE_URL='http://127.0.0.1:18084'
npx playwright test -c tests/e2e/customer-refinement.config.js --project chromium
npx playwright test -c tests/e2e/customer-refinement.config.js --project firefox --project webkit --grep 'responsive customer|quick view retries'
```
