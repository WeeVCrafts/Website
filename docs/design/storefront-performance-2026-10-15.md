# Storefront performance — 15 October 2026

Historical performance baseline: the subsequent customer redesign adds the
requested five-slide, four-second autoplay with pause and reduced-motion controls.
The manual-hero description below records the behavior measured at this stage.

This change addresses the requested image, layout-stability and interaction
performance improvements within the existing Go/templ storefront. The attached
ecommerce research and implementation roadmap informed the checks; they were
not treated as instructions to implement new marketplace features or replace
the accepted design.

## Changes

- Product cards and the main gallery declare square dimensions matching their
  existing reserved frames. The first hero and product image explicitly load
  eagerly at high priority. Existing below-fold product loading remains lazy.
- Inactive hero photographs keep both their source and responsive candidates in
  data attributes until selected. Activation installs `srcset` before `src`,
  preventing an unnecessary original-size download. The hero remains manual.
- Category cards, maker collection previews and search suggestion photographs
  use the existing maintained-image variants. Custom media URLs are preserved.
- Desktop gallery `sizes` reflects its actual column, thumbnail rail and maximum
  width. Zoom requests the original photograph on demand so smaller in-page
  candidates retain access to full-resolution detail.
- Hero controls reserve their final space while enhancement loads. Before
  enhancement they remain invisible and unavailable to keyboard users.
- Catalog filters request `ListingUpdate`, which contains the document title
  and `#listing-content`. The header, footer, scripts and delivery banner remain
  outside the swap. The catalog title, facet counts, checked controls, result
  count, sorting, chips and pagination update together. Native navigation and
  global search retain full-page responses. Existing request cancellation,
  mobile sheet state and focus restoration handle the new fragment target.

Existing opacity/transform feedback and reduced-motion behavior are retained.
Production still uses the existing local HTMX and Alpine scripts. No dependency
or third-party UI library was added. Firebase's existing provider SDK loading
remains conditional on its account controls; it was not removed from working
account flows to reduce a shopping-page byte count.

## Measurement method

`npm run measure:ui-performance -- <artifact-label>` measures the isolated
preview at `http://127.0.0.1:18080`. Start it using
`scripts/run-ui-ux-preview.ps1 -Mode preview` with the existing dedicated QA
PostgreSQL/Redis services available. It does not seed or reset a database.

Each label records three cold-cache navigations per route in Chromium at
390 × 844 CSS pixels, DPR 1, 150 ms simulated latency, 200,000 bytes/second
download throughput and 4× CPU throttling. PerformanceObserver records LCP,
CLS using session windows, long tasks and sampled event duration. Resource
entries record URLs and transferred bytes. Separate probes select the second
hero story and change catalog sort order. Screenshots cover mobile and desktop.

These are local lab measurements. Sampled event duration is not INP, three
navigation medians are not field p75, and emulation does not replace physical
device testing. Run performance measurements separately from browser regressions
and builds to reduce contention. `WEEVCRAFTS_UI_ARTIFACTS` selects the output
directory; otherwise the scripts use the existing temporary UI QA directory.

## Observed results

Raw navigation samples and probe results are recorded in
[the measurement record](storefront-performance-2026-10-15.json). Before and
after runs used the same isolated database and locally compiled application.

| Measurement | Before | After |
| --- | ---: | ---: |
| Second hero photograph, 348 px rendered width | 347,134 bytes | 30,370 bytes |
| Sort response HTML | 50,487 bytes | 26,285 bytes |
| Homepage median CLS | 0.0308 | 0.0038 |
| Homepage median LCP | 2,076 ms | 2,120 ms |
| Catalog median LCP | 2,004 ms | 2,236 ms |
| Product detail median LCP | 2,068 ms | 2,336 ms |

The selected hero download is 91% smaller and filter response HTML is 48%
smaller. The fragment probe confirms that the delivery banner and main element
survive sorting. The intentional catalog intro replacement keeps the heading
correct when categories change. The delayed-script test separately verifies
that showing hero controls adds no height or positional change to that region.

Initial LCP did not improve in this sample. Response times also varied across
runs; three local measurements cannot establish a reliable loading-time effect.
Catalog and product CLS remained approximately 0.0026 and 0.0037 respectively.
Both measurement runs recorded no JavaScript page errors or horizontal overflow.

## Verification scope

The focused Playwright tests exercise delayed scripts/images, reserved media
geometry at 320, 390, 768, 1024 and 1440 px, responsive hero/gallery selection,
original-image zoom, reduced motion, keyboard operation, filter state and
Back/Forward navigation. Existing shopping and recovery suites cover the
working cart, chosen options, totals, wishlist, search errors, no-JavaScript
forms, gallery/video controls and selected operational workflows.

The performance runner and tests are development tools, not production scripts.
Generated templ, CSS and minified JavaScript are included with their sources.

Completed verification:

- Go tests for `web/components`, `web/pages`, `internal/web/handlers` and
  `internal/transport/web` passed. The existing intrinsic-sizing assertion now
  expects the shared product card's square frame.
- `npm run css:build` and JavaScript syntax checks passed.
- The templ freshness check passed after temporarily normalizing CRLF in 42
  untouched generated files. Their original Windows line endings were restored;
  no unrelated generated source changes are included.
- `npm run test:ui-performance`: three focused Playwright tests passed.
- `npm run test:ui-ux:shopping`: passed in Chromium, Firefox and WebKit.
- `npm run test:ui-ux:recovery`: all 11 checks passed after resetting the existing
  QA fixture with `scripts/prepare-ui-ux-fixtures.ps1`. The first run found a
  previously read notification where the fixture expected an unread one.
- Mobile home/catalog and desktop product captures were visually inspected.
  Impeccable's detector reported two existing color advisories in unchanged CSS
  rules, with no new blocking findings.

## Remaining limits

Maintained static photographs have verified generated variants. Uploaded or
external images continue to use the media sources already supplied by the
application; this change does not introduce an image-resizing service or invent
variant URLs. A catalog containing large originals needs measurements using
that actual media before choosing an upload/storage transformation pipeline.

Field Core Web Vitals, physical devices, browser-native zoom and screen-reader
verification remain outside this local automated run. The existing opt-in
diagnostics setting and customer permissions are unchanged.
