# Seller workflows and homepage fade verification

Date: 17 October 2026.

The seller portal keeps its existing Go/templ implementation, branding,
authorization, CSRF protection, exact money parsing, inventory transactions and
fulfilment revision checks. No migration or production deployment is included.

## Delivered behavior

- Guided Details, Pricing & stock, Photos and Review steps, including native
  form fallback, retained validation errors, first-invalid focus, optional
  specifications, unsaved-change protection and a separate review submission.
- Saved-product upload recovery, so a cover failure cannot send a seller back
  through product creation. Editing returns to the saved product editor.
- Explicit stock direction and positive quantity, estimated availability,
  stock-level filters, committed quantities, active-variant totals and the
  selected variant's most recent 20 adjustments.
- One searchable, paginated order queue with distinct page and matching totals.
  Details retain Returns navigation, list state and next-stage actions. Shipping
  requires tracking; stale enhanced submissions retain notes and offer reload.
- A 450ms opacity crossfade with image decoding, overlapping geometry, immediate
  outgoing-slide inertness, rapid-selection handling and animation cleanup.
  Existing autoplay, pause conditions, swipe and reduced motion remain supported.

## Verification completed

- `go test ./...` passed, with the dedicated projection database enabled.
- Focused transport, seller projection and seller template tests passed after
  subsequent validation and conflict recovery refinements.
- PostgreSQL reservation concurrency and suspended-seller availability tests
  passed against isolated data.
- The eight Chromium seller workflow tests passed. Coverage includes step
  validation, server-error focus, editing, native form recovery, review
  submission, saved-draft upload failure, stock filters/history, concurrent
  additions, reserved units, legacy deltas, CSRF, seller isolation, full
  fulfilment progression, live-refresh conflict notices, retained notes,
  pagination, keyboard selection, drawer focus restoration, 390px layouts and
  actual intermediate crossfade geometry.
- The existing catalog carousel autoplay and interaction tests passed across
  320px, 390px, 768px and 1440px. These include four-second playback, hover/focus
  pauses, offscreen pause, swipe, reduced motion and failed images.
- Firefox and WebKit passed the seller route/keyboard/layout matrix and the
  intermediate crossfade test: four additional browser checks. Firefox's first
  route-matrix attempt timed out during navigation; its isolated retest passed
  with the same longer timeout used for multi-route browser matrices.
- `templ generate`, `npm run css:build`, JavaScript syntax checks and
  `git diff --check` passed. Generated assets are included in the change.
- The strict premium UI audit returned zero findings; see `premium-audit.json`.

Browser fixtures run only on port 18080 with the dedicated
`seller_workflows_20261017` database. Projection tests create isolated schemas in
their own dedicated database. Live business data was not used for mutations.
The temporary test application is stopped after verification. The normal local
application and production deployment are unchanged.

## Evidence and limits

- `product-entry-390.png` and `product-entry-1440.png` show the rendered guided
  form at phone and desktop sizes.
- Upload failure recovery was exercised with deliberately unavailable isolated
  storage. Successful persistent storage uploads and live payment/carrier
  integrations were not revalidated in this change.
- `scripts/check-architecture.sh` fails on existing Razorpay-name literals and
  a comment in unchanged payment/storefront domain files. No domain,
  application or port files were changed by this work.
- Automated browser checks do not establish physical-device or screen-reader
  certification.
