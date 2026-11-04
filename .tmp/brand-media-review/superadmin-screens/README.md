# Super Admin references and local verification

The eight original PNGs in this folder are the accepted visual direction.
`verified/` contains screenshots of the implemented Go/templ application using
synthetic accounts and records in the isolated local QA database. These are
rendered application captures, not mockups.

## Capture coverage

Each primary workspace and User Management section has a 390px mobile and 1440px
desktop capture. Reference comparison captures use 1536px. Additional captures
show mobile account actions, the existing account dialog and empty queues.
Viewport captures show the initial screen; `accounts-actions-390.png` shows the
account records farther down the page. The desktop dashboard is also full-page.

The eight reference compositions were reviewed for background/forest palette,
heading hierarchy, summary cards, illustration placement, panel/table spacing,
chart composition and control alignment. Content and counts come from the QA
database. Storefront remains excluded; User Management replaces the four old
sidebar entries. Supported finance operations and account safeguards are retained.

## Reproduce

Use the existing isolated preview on port 18080 and its established fixture data:

```powershell
node scripts/run-authenticated-ui-preview.cjs
$env:WEEVCRAFTS_QA_DATABASE='seller_workflows_20261017'
npx playwright test -c tests/e2e/superadmin-redesign.config.js
npx playwright test -c tests/e2e/authenticated-ux.config.js --project=chromium
```

The redesign suite checks 29 routes at 320, 360, 390, 720, 900, 1024 and 1440px,
plus 1536px comparison captures and an existing product edit form. It checks
document overflow, clipped action labels, sidebar collapse persistence, mobile
focus containment/restoration, internal navigation, filters, CSV exports,
account dialogs, empty queues, reduced motion, 200% CSS zoom reflow, accessibility
and other-role authorization boundaries. Browser coverage is Chromium; these
captures do not claim physical-device or Safari verification.

Other verification: templ generation, `npm run assets:fonts`, `npm run css:build`,
`go test ./...`, `go vet ./...`, the strict premium UI audit, and PostgreSQL tests
for full filtered aggregates/pagination, last-Super-Admin protection, session
revocation, deletion restrictions and reserved-stock concurrency. Integration
tests use the separate local projections QA database and temporary schemas.

The preview intentionally disables external email and real payments. Local
storage/search readiness can show unavailable when those optional QA services
are absent. Those states are displayed truthfully. No production data or
deployment settings were changed during verification.

The release is pushed to the existing GitHub branch for Coolify auto deployment.
