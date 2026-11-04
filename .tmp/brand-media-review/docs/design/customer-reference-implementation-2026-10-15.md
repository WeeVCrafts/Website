# Customer screen reference implementation

Approved inputs: the supplied homepage and Home & Living screenshots. Later user
instructions add the supplied teal/gold logo and five slides advancing every four
seconds. The original Go/templ, HTMX, Alpine, PostgreSQL and Redis stack remains.
Existing uncommitted UI and performance changes are preserved.

## Reference comparison and intentional differences

| Surface | Implemented composition | Intentional difference |
| --- | --- | --- |
| Header | Two desktop rows, prominent search, delivery, wishlist/cart counters and account | Supplied logo added; existing notifications, profile photo and logout retained; comfortable keyboard/touch controls |
| Homepage hero | Panoramic craft photograph, HTML heading, two destinations and selectors | Five slides and visible Pause/Play per later user request; 4-second automatic advance, reduced-motion pause |
| Categories | Seven rounded photographic discovery links | Uses maintained images and working categories/searches; unsupported region navigation omitted |
| Featured products | Six compact cards across wide desktops | Uses real names/prices/availability; preserves missing-media fallback; missing reviews and badges are not fabricated |
| Makers | Three horizontal cards with store and follow actions | Missing maintained portraits/biographies remain absent; no generated maker identity |
| Collections | Five shallow photographic tiles | Destinations map to existing category or explicit text-search queries |
| Trust/footer | Pale trust band and compact light footer | Verified policy language and notification preferences replace unsupported guarantees and newsletter signup |
| Home & Living | Shallow banner, left filter panel, promotion strip, native sort, active chips, three product columns | Current catalog count, categories, makers, price bands and availability retained; unsupported facets omitted |
| Product images | Real maintained/catalog images with responsive candidates | Product objects use contain instead of cropping; fewer live products naturally shorten result grids |
| Narrow screens | Scrollable discovery rails, two readable card columns, one at very narrow widths, stacked maker cards | Existing accessible filter sheet and task layouts preserved |
| Task screens | Shared customer palette, typography, controls and shell across account/shopping/support/auth/info pages | Existing narrower form layouts, CSRF, authorization, payment behavior and server totals retained |
| Operational portals | Supplied logo and favicon in existing branding | Existing role designs and workflows retained |

## Functional verification

- Go template/component and handler checks pass, including fragment rendering,
  explicit option selection and preservation of card presentation after mutations.
- Quick view uses the existing product GET route selected by its HTMX target
  header. Native links still return complete product pages. No database changes.
- Browser checks cover reference widths 1448, 1440, 1024, 390 and 320; customer
  routes, keyboard menus, modal focus/Escape, failed reads/retry, failed cart adds,
  sold-out options, successful adds, guest sign-in feedback and wishlist swaps.
- The customer and guest route/reflow/axe matrices pass. Seller/Admin/Super Admin
  matrices passed during the shared-style regression check. Shopping checks passed
  Chromium/WebKit; Firefox passed on an isolated retry after a navigation timeout
  under parallel load. Existing checkout and authentication checks passed.
- Automatic timing is verified with a controlled browser clock: no advance before
  four seconds, then each slide and wraparound; manual selection and Pause stop it;
  reduced-motion starts paused. Inactive media stays deferred.
- All nine focused storefront/performance tests pass. Header checks pass Chromium,
  Firefox and WebKit at nine widths in authenticated and anonymous states. The
  supplied operational logo/favicon and narrow reflow pass Seller/Admin/Super Admin
  smoke checks at 1440, 390 and 320 pixels. The strict design audit has no findings.
- Browser/IAB attachment was unavailable/unreliable locally; rendered acceptance
  uses the repository's Playwright Chromium tooling.
- Populated shopping states use only the isolated QA app/database on port 18080.
  The active catalog was neither seeded nor reset. The default local catalog
  currently has zero published products, so its honest empty states are expected.
- Local payment checks are simulated; no deployment or real provider transaction
  is part of this work.

## Local viewing

Air v1.67.4 is installed. Run `npm run dev` and open
[http://localhost:8090](http://localhost:8090). Port 8080 serves the Go application;
the browser reload connection belongs to 8090. The launcher retains configured
environment settings and supplies missing local defaults. It does not seed data.
The proxy allows up to 60 seconds for Windows process startup during refreshes.
An actual source edit and restoration both rebuilt embedded CSS and automatically
reloaded a connected Chromium browser through Air's SharedWorker/SSE connection.
The temporary verification change was removed.
The proxy login form also retained real authentication and returned the expected
credential rejection for a deliberately nonexistent local test account.
Generated templates, minified assets, responsive outputs and binaries are excluded
from watching. See the [official configuration](https://github.com/air-verse/air/blob/v1.67.4/air_example.toml).

The supplied original logo is kept at
`web/assets/images/brand/weevcrafts-logo-original.jpeg`. Its optimized PNG and
favicon are mechanical crops/resizes of the original artwork, with proportions
and cream background preserved.

## Editorial artwork

The built-in image generation tool produced editorial banner art only. Files are
saved inside `web/assets/images/customer/`, with proportional responsive variants
in its `responsive/` directory. HTML owns all text, controls and navigation.
Generated artwork is never written into catalog product or maker records.

Retained rendered evidence using isolated QA data:
[homepage desktop](customer-reference-2026-10-15/home-1448.png),
[Home & Living desktop](customer-reference-2026-10-15/home-living-1448.png),
[homepage mobile](customer-reference-2026-10-15/home-390.png),
[Home & Living mobile](customer-reference-2026-10-15/home-living-390.png).

The first two banners (`craft-banner.webp`, `living-banner.webp`) use warm
panoramic Indian craft-studio and handmade-home compositions derived from the
approved references, reserving quiet left space for HTML text. The three additional
generation prompts are recorded verbatim below.

- **textiles-banner.webp**: Use case: photorealistic-natural. Asset type: wide homepage photographic banner for Indian craft marketplace WeevCrafts. Scene: sunlit Indian craft interior, warm cream plaster, wooden surfaces, Folded maroon, mustard and indigo handwoven Indian cotton textiles on a wooden loom bench, a shuttle and tactile thread details. Composition: ultrawide 3:1 landscape; main craft objects occupy right half, quiet pale cream negative space across left 45% for separately rendered HTML heading. Natural editorial photography, real tactile imperfections, restrained warm off-white, terracotta, dark wood and subtle green. No words, letters, logos, UI, watermark, collage or borders. Not a product catalogue claim; editorial atmosphere only.
- **gifts-banner.webp**: Use case: photorealistic-natural. Asset type: wide homepage photographic banner for Indian craft marketplace WeevCrafts. Scene: sunlit Indian craft interior, warm cream plaster, wooden surfaces, Handmade gifts wrapped in plain cream paper and rust cotton ribbon, a hand-painted ceramic bowl and carved wooden keepsake box on a wooden table. Composition: ultrawide 3:1 landscape; main craft objects occupy right half, quiet pale cream negative space across left 45% for separately rendered HTML heading. Natural editorial photography, real tactile imperfections, restrained warm off-white, terracotta, dark wood and subtle green. No words, letters, logos, UI, watermark, collage or borders. Not a product catalogue claim; editorial atmosphere only.
- **metalwork-banner.webp**: Use case: photorealistic-natural. Asset type: wide homepage photographic banner for Indian craft marketplace WeevCrafts. Scene: sunlit Indian craft interior, warm cream plaster, wooden surfaces, Traditional brass diya lamps and small metalwork objects on a warm wooden console, soft marigold accents and natural light. Composition: ultrawide 3:1 landscape; main craft objects occupy right half, quiet pale cream negative space across left 45% for separately rendered HTML heading. Natural editorial photography, real tactile imperfections, restrained warm off-white, terracotta, dark wood and subtle green. No words, letters, logos, UI, watermark, collage or borders. Not a product catalogue claim; editorial atmosphere only.

## Subsequent real-data Docker completion pass

The later user request supersedes the earlier isolated preview handoff. QA/demo
containers have been removed. The final preview uses one `weevcrafts` Compose stack
and retained real database/media/identity volumes, without seed commands, simulated
payments or local delivery simulation. Database/media backups precede cleanup.
Four existing accounts remain; there are no published products or orders. The old
isolated screenshots above are historical verification evidence, not the live app.

The image now regenerates responsive images, CSS/JS and templ during its build and
includes the supplied PNG artwork. The final application runs as user `app`, not
root. Normal Compose service list: app, PostgreSQL, Redis, Meilisearch, Garage and
its one-time initializer. Operator tools are optional; unrelated Docker containers
and all data volumes are preserved. Explicitly identified old application build
cache reclaimed 27.42 GB.

`npm run dev:docker` uses Air to rebuild the actual Compose app and refresh its
browser proxy at http://localhost:8090. The fallback direct URL is
http://localhost:8080. No GitHub push or remote deployment occurred.

Final automated browser inspection was rejected by the in-app browser URL security
policy. No alternate browser path was used to bypass that rejection. Source checks,
container health and asset verification pass; final live empty-state composition
and the last toolbar-spacing adjustment are awaiting the user's manual review.
Real Razorpay credentials are missing, so live payment acceptance remains unavailable.
Existing Firebase and Resend configuration is retained; no real email or payment was
sent during verification. See `implementation-status.md` for the active checklist.

The Docker Air path was subsequently verified through its reload stream: a source
change rebuilt the actual app container, waited for its health check and emitted
`event: reload`. Both direct and proxied readiness return 200 after completion.
The final shared CSS bytes match the current source build. No browser URL-policy
restriction was bypassed to obtain this service-level evidence.
