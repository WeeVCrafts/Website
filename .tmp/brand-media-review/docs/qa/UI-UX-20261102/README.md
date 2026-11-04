# Authenticated role UX audit — 2 November 2026

The live-browser follow-up below supersedes the earlier browser-availability
limitations in this report. Open [the before/design/after gallery](comparison.html)
for the screen comparisons and full route inventory.

**Subsequent owner request:** The current Super Admin Storefront editor is now
retired pending a replacement brief. Its navigation and category/promotion
shortcuts are removed, legacy editor bookmarks return to the dashboard, and
editor mutations return 410 without saving. Published customer pages and stored
design data are retained. Go tests and all three focused Chromium removal,
Admin-category and identity-navigation regression cases passed. The gallery
records the earlier audit before this removal.

## Scope

Reviewed the authenticated route and shell contracts for the four canonical
roles: Customer, Seller, Admin and Super Admin. Support, finance and operations
remain Admin workspaces or legacy aliases. The review covered the shared
navigation, dashboard entry points, task queues, forms, tables, empty/recovery
states, mobile drawers and the customer account/order paths.

## Implemented

- Added the shared `TaskGuidance` component for one clear next action without
  changing routes, permissions, form names, CSRF fields or backend contracts.
- Added role-entry guidance to Customer account/orders, Seller dashboard,
  Admin/Super Admin dashboards and the live Support dashboard.
- Added responsive guidance-card behavior at narrow widths and constrained the
  customer presentation to the existing content measure.
- Kept operational tables native and added sticky headings only to tabular
  desktop tables; existing labelled record/mobile overflow behavior remains the
  mobile contract.
- Reconciled the durable decisions in `DESIGN.md` and `UX-CONTRACT.md`.

## Evidence and verification

- `templ generate ./web/...` — passed.
- `go test ./web/components ./web/pages ./web/admin ./web/selleradmin ./web/supportportal` — passed.
- `go test ./...` — passed.
- `go build ./cmd/website` — passed.
- `npm run css:build` and `npm run css:preview` — passed. Browserslist emitted
  its existing caniuse-lite maintenance advisory.
- Frontend premium audit in strict mode — passed with zero findings.
- Impeccable detector re-run after editing; no new side-tab finding from the
  implemented guidance pattern.

## Browser gate and unresolved items

The dedicated preview ports `18080` and `18081` were not running, and the
isolated QA database/Redis endpoints were unavailable in this environment. The
existing browser scripts therefore could not complete authenticated route,
keyboard, axe, screenshot or 320/390/720/1024/1440 viewport verification.
`npm run test:ui-ux` stopped on connection refusal to `127.0.0.1:18080`.

Two local static Playwright checks also remain red against the current source:
`test:storefront-header` fails its existing header expectation and
`test:firebase-ui` expects the older login error copy. These are recorded as
follow-up compatibility checks, not silently changed as part of this UX-only
pass.

Required external gates remain real iOS/Android viewport and keyboard behavior,
native zoom/safe areas, screen-reader validation, uploaded-media storage and
live payment-provider behavior.

## Live-browser follow-up

The application was built from the current working tree and run at
**http://127.0.0.1:18080**, using the existing disposable
`seller_workflows_20261017` database in `weevcrafts-storefront-qa-db` and the
dedicated QA Redis instance. The existing marketplace at port 8080 and its
records were preserved. Both applications returned HTTP 200 from `/health/ready`.

### Coverage

| Role | Rendered screens |
| --- | ---: |
| Customer | 23 |
| Seller | 16 |
| Admin | 37 |
| Super Admin | 46 |
| **Total** | **122** |

The original 99-screen sweep includes real customer order details, seller
product editors and order details, queues, accounts, support, finance and
security surfaces. The final coverage adds logged-in contact/support creation
and 11 existing storefront-draft workspaces per company role, including
homepage, slideshow, categories, page templates, theme, navigation, media,
content pages, policies, promotions and publishing.

Every screen was checked at **320, 390 and 1440 CSS pixels**: 366 responsive
checks, with 244 full-page implementation screenshots. Axe's WCAG A/AA rule
sets ran at both 390 and 1440px on every final screen: **244 audits with zero
violations**. All checked screens returned HTTP 200, with zero uncaught page
errors, unnamed visible controls or document-overflow findings. This is
automated coverage, not accessibility certification.

The media picker's `/storefront/library` endpoints return HTML fragments for
an existing fetch workflow. They were inspected separately and excluded from
the screen count; document-title/language rules on a fragment are not page
failures. Their original results remain in the raw editor report.

### Implemented improvements

- Customer account recommendations now exist in the live template, with a
  direct latest-order action and address/shopping guidance for empty accounts.
- Account support opens request history; tracking help carries the selected
  owned order into the existing support form.
- The catalogue banner no longer inherits an obsolete two-column grid that
  squeezed mobile text. Existing imagery, collections, filters and branding remain.
- Customer footer links have at least 32px target height. Payment activity
  reuses shared status colors, fixing the observed 4.11:1 captured-status contrast.
- Seller stock guidance opens the existing low-stock queue. First-time sellers
  get a draft-entry explanation. Header search names and submits to a supported
  queue, retains applicable filters and falls back to catalogue search elsewhere.
- Store profile marks its current location. Escape respects form/dialog focus
  when the seller navigation drawer is closed. Dashboard record links are visibly
  underlined in both operational shells.
- Super Admin identity actions lead directly to Users, Roles and Security.
  Category presentation actions fit 320px in both company roles.

The six responsive design studies were generated before implementation, using
the running app's own components and fixture data. See
[the design decisions and drift review](responsive-designs.md).

### Engineering verification

- `templ generate ./web/...` — passed.
- `npm run css:build` — passed, including CSS/JS minification. Existing
  caniuse-lite maintenance advisory remains.
- `go test ./...` — passed. Integration tests with external opt-in dependency
  gates are not represented as an exercised live-provider integration.
- `go test ./web/components ./web/pages ./web/selleradmin` — passed against the
  final source after the captured-payment tone change.
- Source Go build and preview startup — passed.
- Strict frontend premium audit — zero findings.
- `designmd lint DESIGN.md` — zero errors; one warning for the existing unused
  `colors.accent` component-token reference.
- Whitespace check on the touched implementation files — passed with the
  repository's existing CRLF policy. Unrelated in-progress edits were preserved.

The focused suite covers latest-order/support navigation, catalogue
reflow, stock guidance, seller search/filter state, profile location, Escape and
confirmation focus, retained input after a simulated 503, company drawers and
role-gated identity links. It never writes to the main marketplace database.

| Engine | Workflow result |
| --- | --- |
| Chromium | 9/9 passed |
| WebKit | 9/9 passed; structured result in `tmp/authenticated-ui-audit/webkit.json` |
| Firefox | Unresolved harness failure: context teardown timed out; 1 reported failure and 8 not run in the final bounded retry |

Firefox reached the expected support destination before teardown, but that is
not reported as a passed test. Repeated attempts with a longer timeout, no
concurrent route sweep and trace recording disabled still failed in
`browserContext.close`. Browser logs include `RenderCompositorSWGL failed
mapping default framebuffer`, Juggler `NS_ERROR_FAILURE` and a timed-out
Playwright `generateAriaTree` call. The final failure is preserved in
`tmp/authenticated-ui-audit/firefox.json`. Firefox workflow verification remains
open; the UI was not changed to conceal this browser-harness failure.

Offscreen descendants in the gallery's geometry details were inspected:
collection/finance navigation rails and named, focusable operational table
regions intentionally own horizontal scrolling. They are distinct from the
fixed category action that previously escaped the document at 320px.

### Evidence and reproduction

- `tmp/authenticated-ui-audit/before/report.json` — original screen audit.
- `tmp/authenticated-ui-audit/designs` — responsive HTML studies and screenshots.
- `tmp/authenticated-ui-audit/after/report.json` — combined final 122-screen audit.
- `tmp/authenticated-ui-audit/editor/report.json` — raw additional editor audit.
- `tmp/authenticated-ui-audit/premium-audit.json` — strict static audit.
- `tests/e2e/authenticated-ux.config.js` — Chromium, Firefox and WebKit workflow suite.

Start the preview with `node scripts/run-authenticated-ui-preview.cjs` after
stopping the previous audit preview. It uses only the named existing QA
containers/database, builds Go source, and never reads `.env` or seeds records.
To repeat the main screenshot/audit sweep, set `WEEVCRAFTS_UI_ARTIFACTS` to an
output directory and run `node scripts/audit-authenticated-ui.cjs`; set
`WEEVCRAFTS_AUDIT_MOBILE_AXE=true` for both axe viewport checks. Additional editor
coverage uses `WEEVCRAFTS_AUDIT_EXTRA=true` and an existing QA draft ID in
`WEEVCRAFTS_AUDIT_DRAFT` (this run used draft 2).

For the browser workflow suite, set
`WEEVCRAFTS_QA_DATABASE=seller_workflows_20261017` and run
`npx playwright test -c tests/e2e/authenticated-ux.config.js`.

The local preview uses the repository's existing disposable demo accounts:

| Role | QA email |
| --- | --- |
| Customer | `demo.customer01@weevcrafts.local` |
| Seller | `demo.owner01@weevcrafts.local` |
| Admin | `demo.marketplace01@weevcrafts.local` |
| Super Admin | `demo.super01@weevcrafts.local` |

Their existing fixture password is `WeeVCrafts-Demo-2026!`; these are local QA
identities, not credentials for the marketplace on 8080 or a hosted deployment.

### Remaining limits

This preview has no QA object-storage endpoint or transactional email sender.
Existing images/fallbacks are visible, but successful uploaded-media persistence
and outbound email delivery were not verified. Local deterministic payments do
not establish live Razorpay behavior. Physical iOS/Android keyboards, safe areas,
OS zoom and an actual screen-reader session remain unverified. No deployment
to Railway or production account/data change was performed.
