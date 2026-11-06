# Homepage and product gallery — 6 November 2026

Implemented the four requested screenshot adjustments within the existing Go,
templ, JavaScript and Tailwind application. New arrivals immediately precedes
Featured from our makers in defaults, saved releases and previews. The complete
shopping trust strip appears once after Curated Collections, with an end-of-page
fallback when Collections is missing or hidden. The utility bar places shipping
left, the slogan in the center and Support right. At 1100px and below it hides
the slogan and retains both links; phones keep one compact row.

The gallery has explicit desktop grid areas and a bounded mobile thumbnail rail.
Counters do not wrap. Photos and enlarged images have stable loading/error states
and Retry. A previous image's completion cannot update the current image's status.
The shared missing-image handler leaves gallery failures to the gallery owner.
Single-image navigation geometry is decided in the server-rendered markup.
Existing purchase and wishlist busy/error handling remains authoritative; Share
now uses that same busy indicator.

Public photographs have content-based ETags for conditional GET/HEAD delivery,
retaining authorization checks and the five-minute cache policy. Private previews
remain private and uncached. The maintained header/footer logo now uses the
existing content-versioned asset URL. Uploaded, external and query-bearing logos
retain their saved references. Inactive full gallery media remains deferred;
original photographs remain available through explicit enlargement.

## Verification

- Go tests passed for `web/components`, `web/pages`, `internal/domain/storefront`,
  `internal/transport/web` and `internal/web/handlers`. Tests cover saved ordering
  without mutation, missing/hidden Collections, server-known gallery geometry,
  content validators, replacement media, private previews and authorization denial.
- All nine new homepage/gallery checks passed across Chromium, Firefox and
  WebKit. Four existing Chromium performance regressions passed. Checks cover
  320–1920px, breakpoint boundaries, landscape, seven media items, two-digit
  counts, delayed/failed/cached photos, Retry, stale events, zoom, keyboard
  navigation and synthesized swipe events. No page overflow occurred at tested
  viewport widths. Desktop/mobile screenshots were visually inspected.
- Existing header checks passed in all three engines. Shopping regressions
  passed in all three engines after replacing the obsolete drawer expectation
  with the application's current Add to cart navigation to `/cart`.
- Affected templ outputs were regenerated. CSS/JavaScript build, JavaScript
  syntax checks, asset minification regression and `git diff --check` passed.

## Local performance observations

The existing runner measured three cold navigations per route in Chromium at
390 × 844 CSS pixels, DPR 1, 150ms latency, 200,000 bytes/second download throughput
and 4× CPU throttling. The runner now records one additional warm navigation.
Browser checks and builds were stopped before the final measurement.

| Route | Median LCP before | Median LCP after | Median CLS before | Median CLS after |
| --- | ---: | ---: | ---: | ---: |
| Home | 3,772ms | 3,488ms | 0.00780 | 0.00633 |
| Catalog | 2,944ms | 3,556ms | 0.00481 | 0.00481 |
| Product detail | 3,072ms | 2,820ms | 0.00232 | 0.00232 |

An intermediate measurement identified a 66,978-byte logo transfer on every
navigation. Content-versioned logo delivery reduced measured warm image transfer
from 67,734 to 756 bytes on Home and Catalog. Product warm image transfer changed
from 66,978 to 14,158 bytes across the same measurements. Conditional photograph
tests separately verify HTTP 304 responses with no response body and no additional
storage opens for warm derivatives.

Cold resource transfer increased approximately 3.7–4.1KB for loading feedback.
These small local samples do not establish field performance or a sitewide loading
improvement; Catalog timing was slower in this run. All final measured navigations
had no horizontal overflow or JavaScript page errors. Samples and medians are in
[the measurement record](homepage-gallery-performance-2026-11-06.json).

## Verification limits

The isolated preview used maintained photographs and the existing QA database.
Its S3 service was unavailable, so uploaded photograph delivery was verified with
Go HTTP/storage tests rather than live bucket traffic. No production data was
seeded, migrated or reset by this task.

Windows WebKit reported a playback error for the MP4 fixture despite advertising
MP4 support. Its visible failure state and continued photo navigation passed;
successful MP4 decoding in that engine remains unverified. Chromium and Firefox
loaded the fixture. Swipe evidence uses synthesized events, not a physical device.

Physical devices, screen-reader announcements and native browser zoom remain
unverified. A 720px viewport checks the reflow expected from a 1440px window at
200% browser zoom. A separate 200% CSS-zoom stress probe, which retains desktop
media-query breakpoints, exposed overflow in the existing desktop header; it is
not evidence of native browser-zoom behavior.

Changes remain local on `main`; no production deployment or push was performed.
