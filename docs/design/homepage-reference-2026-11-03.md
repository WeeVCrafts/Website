# Homepage reference and local image packs — 3 November 2026

## Scope and implementation

Home alone loads `homepage-reference.css` and receives the approved forest utility
bar, compact header, native rounded search, Offers navigation, inset photographic
hero, circular categories, trust band and Featured spacing. Current logo, published
policy, account state, live catalogue and lower homepage sections remain authoritative.
Other storefront and role shells retain their existing presentation.

Shipping copy uses the configured free-shipping threshold (₹1,500 in the QA release),
not the reference's ₹999, city or international claims. Trust content is the existing
published copy and renders once after category discovery, before Featured. Empty
categories and product selections explain that published collections will appear
there; generated review content is never a catalogue fallback.

The shared hero retains five slides. Autoplay uses 3000ms, with wraparound, 450ms
crossfade and fresh intervals after manual navigation. Arrows and indicators retain
44px targets. On phones, arrows sit above indicators to prevent overlap. Hover,
keyboard focus, hidden tabs and offscreen viewing stop rotation. Reduced motion
keeps manual navigation without autoplay/fade. Next-image prefetch begins after the
visible hero loads; readiness checks retain the current usable slide on slow/failing
images. Home search performs document navigation; other searches keep their shared
HTMX fragment behavior.

## Reference comparison

Compared the supplied 1536×1024 reference with rendered QA Home at that exact
viewport. Checked utility/header/nav height, rounded hero framing, serif typography,
terracotta emphasis, circle crop, trust position and Featured spacing. Inspected
separate 320/390px captures and automated 768/1024px layouts. Short action labels
remain intact; whole actions move between rows. Category overflow has existing
keyboard-accessible arrows and horizontal scrolling. No document overflow detected.

Intentional differences: the accepted current logo; real shipping/trust copy; guest
Sign in; five indicators matching five stories; 44px controls; photographs and cards
from saved records. The generated folk portrait and product concepts stay local.
Reference leaves and illustrations are not substituted for real published content.

Evidence is outside Git at:
`C:/Users/Vinay/.codex/visualizations/2026/11/02/01a19c9b-5a2b-7541-8147-f0a5b366a765/homepage-reference/`.
This includes each engine's five viewport captures, browser-results.json, audit,
image validation and asset hashes. The 200% reflow check uses the equivalent 768 CSS
pixel viewport for a 1536px screen; this is desktop browser emulation, not physical
mobile-device or assistive-technology certification. Visibility events and touch
coordinates are deterministic browser test inputs.

## Local review photographs

Generated each of 21 square PNGs separately through imagegen. Seven category
photographs are in root `categories/`; fourteen product covers are in root `products/`,
two per category subfolder. All are 1254×1254, below 16 megapixels and the category
8 MiB / product 25 MiB upload limits. Warm light, ivory backgrounds, centered framing,
no embedded labels or watermarks were inspected. Filename maps, prompts and size
validation live beside the images. No prices, stock, seller identities or verified
provenance claims accompany these concepts.

Both directories are ignored by Git and Docker. Go embeds only web assets; existing
image builds and Air source roots never import these packs. Manual upload/publication
remains an owner action. Existing unrelated screenshot changes and research document
are preserved.

## Verification

- `templ generate`: passed; required generated Go sources committed.
- `npm run css:build`: passed, including CSS and JS minification. Existing outdated
  caniuse-lite advisory remains; no dependency update was needed for this task.
- `go test ./...` and `go vet ./...`: passed.
- Database-backed catalogue/search/storefront regressions passed on dedicated
  `homepage_regression_20261103`, not production or the normal development database.
- `homepage-reference.config.js`: 18/18 passed, six scenarios each on Chromium,
  Firefox and WebKit; no skips or flaky results. Covers exact 2999/3000ms boundary,
  all five slides, wraparound, manual reset, swipe, hover/focus/visibility/offscreen
  pause, reduced motion, slow/failed photographs, guest/authenticated headers,
  native search transitions, profile/menu keyboard behavior and responsive reflow.
- Empty-state browser checks on a separate isolated app at port 18089 passed at
  all five widths: zero product cards/categories, explicit category/product messages,
  one trust band and no overflow. Category Next also passed keyboard scrolling.
- Fresh responsive captures repeated on all three engines after final CSS refinements.
  Axe WCAG A/AA checks on homepage content reported zero violations.
- Existing catalogue carousel/section tests, storefront reference reflow/manual
  controls and media/filter-performance regressions passed.
- Existing customer cart/checkout layout suite: 5/5 passed, including long names,
  large totals, 320/390/720/1440px and variant restoration.
- Strict premium audit: zero findings. Design-contract lint: zero errors, existing
  unused accent-token warning. Scoped source review and whitespace checks passed.

Initial harness failures were corrected: Firefox needs a future clock pause point
and portable touch events; WebKit font readiness can await a deliberately blocked
image; an already-cached photograph cannot test first-load failure. Fresh-context
failure tests now block the initial prefetch and verify recovery explicitly. Final
mobile review removed the inherited 90px cap from short section actions so complete
labels fit or move to the next row; Firefox/WebKit overflow checks caught this.

## Release

Deployment branch is verified `main` on WeeVCrafts/Website. Coolify application
`vy2hpttvmehhauvskvilgoju` builds automatically from the pushed commit. Verify exact
image revision, readiness, homepage assets and public routes after deployment.
Production smoke checks are read-only. Previous image
`vy2hpttvmehhauvskvilgoju:5c8de6803a2d9ca1d05b57729501a4133605e0ba`
is retained as the rollback revision. Deployment results are recorded with external
QA evidence and the final task response.
