# WeevCrafts branding and responsive homepage — 15 October 2026

Implementation on `main`, fast-forwarded from `020f058` to the approved existing `b4968a7`. No additional branch, database migration or business API change. Existing local volumes and catalogue were retained. The user subsequently authorized publishing the branding/header update through GitHub's existing Railway integration. New login and guest-popup redesigns require separate concept approval and are outside this release.

## Accepted references and implementation

- [Desktop concept](brand-theme-concept-2026-10-15.png): enlarged supplied woven mark and live WeevCrafts wordmark, forest controls, warm white/ivory surfaces, restrained terracotta/gold.
- [Mobile concept](brand-theme-mobile-concept-2026-10-15.png): explicitly approved after the desktop plan. Separate responsive introduction on the same `/` URL, with shared products, makers and collections rendered once.
- `brand.css` owns shared colour tokens and role aliases. Typography and operational layouts are retained. Semantic success, warning and destructive colours stay independent; legacy red tokens were not globally rebound to green.
- Removed the shared delivery shortcut, artisan-livelihood nav link, hero playback controls and automatic advancement. Address management and checkout delivery remain. The desktop hero retains five manual slides, arrows, selectors, announcements, inactive-slide inertness and deferred secondary media.
- Mobile uses a compact craft hero, full-width search, scrollable categories and two product columns at 390px and 320px. The editorial mobile banner was generated from the existing craft artwork against the approved composition. It is not a maker portrait or product record. Responsive picture sources avoid downloading both hero photographs on phones.
- Corrected homepage search to replace the main page, preventing nested `#page` elements and stale homepage styles after an HTMX search.
- Guests use a direct Sign in header link. The authenticated account dropdown and its private avatar, account links, notifications and sign out remain.
- Homepage title, description, Open Graph identity and the existing WebSite/Organization graph consistently use WeevCrafts. Canonicals, robots/sitemap handling, product schema and private-page restrictions are retained.

## Visual fidelity ledger

Compared both approved images with actual browser captures. Generated concept merchandise never replaced real catalogue records.

| Area | Rendered comparison and disposition |
| --- | --- |
| Brand visibility | Original woven artwork is retained. The final user correction keeps the desktop mark 52–70px and reduces wordmark text to 32–44px. Phone mark remains 32–38px, wordmark 22–30px. |
| Desktop composition | Two-row header, wide search, full-width photographic hero, seven-image category rail and featured selection follow the concept. At 1680px, header/hero boundaries are close to the reference. |
| Desktop hero | Initial screenshot exposed clipped actions; height, paragraph rhythm and button sizing were corrected. A browser regression now checks that actions fit inside the hero. |
| Colour and materials | Forest primary/hover, ivory search, warm white surfaces and terracotta/gold accents use the approved tokens. Portal nested wordmark and heading overrides were checked separately. |
| Mobile header | Menu/brand/cart/account share the first row; search fills the second. Wishlist remains available in navigation. No device redirect or separate indexed mobile URL. |
| Mobile hero | Clear wall behind live headline and actions, artisan/vase to the right. Text-free artwork replaces an unsuitable panoramic crop. No tint overlay. |
| Narrow content | Categories scroll; product cards remain two columns at 320px. At 320px the View all link wraps below the long featured heading to preserve readable text and usable targets. |
| Content fidelity | Real names, prices, availability, missing-image fallbacks and empty states take precedence over illustrative concept cards. Operational fonts remain Inter/Roboto; storefront remains DM Sans/Playfair Display. |

## Verification

Executed locally:

- `npm run assets:images`, `npm run css:build`, `templ generate -path . -w 2` and `templ generate -path . -check -w 2`.
- `go test ./web/... ./internal/web/... ./internal/transport/web/... ./cmd/website/...` passed.
- Storefront reference and performance suites: 12 tests passed. Coverage includes manual five-slide navigation/no advancement, deferred media, one mobile hero download, stable product media, facets/history/focus, quick view/cart variants, wishlist and customer route families.
- After the final palette adjustment, the responsive, mobile and seller/admin cases were rerun: 3 passed. Seller, marketplace admin and super admin were checked at 1440px, 390px and 320px for overflow, retained fonts, green branding/primary actions and distinct semantic states.
- The added homepage search regression passed after the page-swap fix, including actual product results, one main page, removed homepage styles and browser Back. All 13 relevant browser regressions have passed across these runs. Go component/page checks and the affected generated template check passed again after this fix.
- Chromium and WebKit shared-header checks passed for guests and signed-in customers at 320, 390, 560, 768, 820, 960, 1024, 1280 and 1440px. Checks include counters, profile dismissal, focus return, menu trapping, Escape, backdrop dismissal and reduced motion.
- The final read-only Air audit passed for the homepage/listing at 1680, 1448, 1440, 1024, 390 and 320px, public route reflow, filter focus and all five slides, with no browser errors or fixture writes. Axe audits reported zero violations on home, listing, sign-in and contact. [Machine-readable audit](brand-theme-live-verification-2026-10-15.json).
- JavaScript syntax and `git diff --check` passed.

Only isolated QA services on ports 15432/16380 and the isolated app on 18080 received fixture/account/cart/wishlist writes. The PowerShell fixture helper was corrected to retain Unicode through native stdin. Main Air acceptance inserts no records.

Air runs through `npm run dev:docker` at <http://localhost:8090>. A serial Compose helper prevents overlapping Docker rebuilds when Air restarts its Windows build shell. Existing application data is not seeded or reset. The Air catalogue is currently empty, so its screenshots truthfully show an empty state. Populated screenshots are labelled as isolated QA.

Codex's in-app browser could not initially attach; the preview was opened in Chrome through Codex's browser controls, with temporary viewport sizing reset. Automated evidence uses the existing Playwright harness. The codebase-memory connector was unavailable (`Transport closed`), so structural checks used repository source.

## Google visibility baseline and release follow-up

The final release review also checked the smaller desktop wordmark (36.17px text with the unchanged 55.95px mark at 1365px), preserved mobile sizing, direct guest sign-in and authenticated dropdown behavior. Go component/page/handler checks, the header suite in Chromium/WebKit at nine widths, and responsive/search regressions passed again. Production dependency audit reported zero vulnerabilities. [Final desktop header](brand-theme-final-header-2026-10-15.png), [final phone header](brand-theme-final-sign-in-header-390-2026-10-15.png).

Rollback reference: the previous successful Railway deployment was `35def85d-6d28-4e58-bf1f-18550247798f`, commit `b4968a76c6b5d6a606714ea0b5fc7d9b19f17580`. If the branding release requires rollback, revert its new commit on `main` and push normally, then verify the automatic deployment and `/health/ready`. No database rollback is needed for this release.

The earlier read-only Search Console inspection found the homepage indexed, a successful fetch, Google's selected canonical matching `https://weevcrafts.com/`, and a successful sitemap with 34 discovered pages. The inspected performance view recorded 1 impression and 1 click, with no usable query rows. That does not establish why the brand-only query performs poorly.

The new metadata is local until release. After production review and deployment, inspect the updated root URL in the existing property and request indexing once. Record the report date range when comparing subsequent brand-query impressions with this baseline. No indexing request was submitted during local implementation. Google does not guarantee indexing or timing after a request: [recrawl guidance](https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl).

Physical-device testing, browser-native zoom, screen readers and external Google sign-in/payment providers remain untested. Automated accessibility checks are supporting evidence, not certification.

## Saved browser evidence

- Air: [desktop 1680px](brand-theme-air-desktop-2026-10-15.png), [phone 390px](brand-theme-air-mobile-390-2026-10-15.png), [phone 320px](brand-theme-air-mobile-320-2026-10-15.png), [sign-in 390px](brand-theme-air-sign-in-2026-10-15.png).
- Isolated QA catalogue: [desktop](brand-theme-qa-desktop-2026-10-15.png), [mobile 390px](brand-theme-qa-mobile-390-2026-10-15.png), [seller](brand-theme-qa-seller-2026-10-15.png), [admin](brand-theme-qa-admin-2026-10-15.png).
