# UI/UX verification — 14 October 2026

See the [140-task ledger](../../ui-ux-implementation-ledger-2026-10-14.md) for individual evidence, state coverage and release gates. These results use fictional isolated demo data, not active/production customer records.

## Tested scope

Current Customer, Seller, Admin and Super Admin plus authentication. The broad matrix contains 127 route visits and 889 viewport checks at 320, 360, 390, 430, 768, 1024 and 1440px; additional seller-order and recovery checks cover an owned record and configured edge states. Chromium, Firefox and WebKit core paths were exercised. Axe tags cover WCAG 2 A/AA, 2.1 AA and 2.2 AA. Zero reported violations is not certification.

Screenshots ending in `390` / `1440` show actual browser renders. Files prefixed `concept-` are illustrative composition references; their logos/people/merchandise/copy/prices are not application data. Maintained assets, accepted branding and current server data win.

## Reports

- `verification.json`: route/reflow/accessibility and core browser matrix.
- `verification-initial.json` / `verification-firefox-repeat.json`: preserved late Firefox recovery/navigation failures and the separate passing repeat on the same preview. The final report aggregates the passing Chromium matrix/WebKit checks with that Firefox repeat and retains the earlier failed-check metadata.
- `recovery-verification.json`: failed requests, retained input, no-JS, spacing/motion/forced colors, media retry, owned seller detail and notification ownership.
- `shopping-verification.json`: chosen-option shopping, Wishlist retrieval, Cart/current totals, rapid activation and Back/query.
- `checkout-verification.json`: local simulated payment, duplicate activation, refresh and same-order failed-payment recovery.
- `baseline-final-measurements.json` / `current-final-measurements.json`: cold then immediate warm runs under the same mobile/network/CPU profile.
- `premium-audit.json`: static UI consistency check, not a functional/accessibility certificate.

Performance is a single lab navigation per cache state, not production percentile/INP data. Profile: 390x844, 150ms latency, 1.6Mbps down and 4x CPU. The interaction sample opens/closes the profile disclosure; it is not a field INP assertion. No analytics, participant recruitment or conversion claims.

## Build and repeat checks

Normal app: `npm ci`, `templ generate ./web/...`, `npm run css:build`, then the existing Go build. Responsive image/font artifacts are committed. Optional regeneration uses `npm run assets:images` and `npm run assets:fonts` (fontTools/Brotli).

`css:build` also regenerates minified first-party JavaScript (`assets:js`). Templates reference those outputs; maintained readable JavaScript remains the source of truth. No backend compression/cache-header change is included.

The accessibility scripts resolve pinned build/test-only `axe-core` from installed dependencies; no uncommitted temporary audit download is required.

Only run mutation-capable QA against the dedicated setup: PostgreSQL container `weev-ui-qa-20261014-postgres`, host port 15432, database `weev_qa_ui_20261014`; Redis host port 16380; app port 18080. `scripts/run-ui-ux-preview.ps1` uses those exact endpoints and clears real provider/email credentials. It does not provision infrastructure, silently kill occupied ports, or reset a database. Prepare the isolated demo data using the existing seed command through this runner, then `scripts/prepare-ui-ux-fixtures.ps1`. Never seed/reset the application at 8080.

The isolated preview uses `demo.customer01@weevcrafts.local`, `demo.owner01@weevcrafts.local`, `demo.marketplace01@weevcrafts.local` and `demo.super01@weevcrafts.local`; password `WeeVCrafts-Demo-2026!` is an existing local-demo fixture credential, not a production secret. Local payment tests charge no money.

Set `WEEVCRAFTS_UI_ARTIFACTS` to a chosen output directory, or use the temporary-directory default. Run the UI scripts from repository root. Current checks:

```text
go test ./web/... ./internal/web/handlers ./internal/transport/web
npm run test:ui-ux
npm run test:ui-ux:recovery
npm run test:ui-ux:shopping
npm run test:ui-ux:checkout
```

The existing header/role tests also require `WEEVCRAFTS_BASE_URL=http://127.0.0.1:18080`. Legacy tests guarded to another disposable database/retired roles must not be forced against this dataset. Native print controls are rendered/audited, but OS print dialogs were not driven.

## Required external sign-off

Real iOS/Android, native zoom, on-screen keyboard/chrome/safe areas, NVDA/VoiceOver/TalkBack, representative uploaded video/captions, configured storage and live payment provider. Browser-only media fixtures/intercepted failures do not establish storage/provider integration. Existing build-tool advisories require separate maintenance review; no major Tailwind migration or backend/security-rule change is included here.
