# UI/UX implementation ledger

Plan: [140-task roadmap](ui-ux-implementation-plan-2026-10-14.md). Started 14 October 2026.

The initial-release evidence below is a historical snapshot. The user-approved [public discovery/account access follow-up](ui-ux-auth-followup-2026-10-14.md) supersedes its mandatory-login presentation statements, including K10 and the earlier guest header checks. Catalogue browsing is now public without a frontend gate; protected server actions remain protected.

Changes are restricted to presentation assets/templates, generated presentation output, UI verification and design documentation. Existing service/API/database code remains unchanged. Existing knowledge-graph files are unrelated in-progress changes.

## Design reference and constraints

Generated with the built-in image tool against the existing DESIGN.md palette and typography. These are composition references, not catalogue content. Maintained images, actual product/price/review data, the existing logo and supported routes override illustrative image content. No generated product data or maker biographies enter the application. New concepts do not authorize new services or role changes.

- [Home reference](qa/UI-UX-20261014/concept-home.png)
- [Listing reference](qa/UI-UX-20261014/concept-listing.png)
- [Detail reference](qa/UI-UX-20261014/concept-detail.png)
- [Checkout reference](qa/UI-UX-20261014/concept-checkout.png)

Locked tokens: paper #f6f0e6, surface #fffdf8, forest #124534, ink #2d201b, muted #625548, terracotta #9d4031, border #dfd3c5. Playfair Display for storefront headings; DM Sans for storefront interface. Portal families/palettes remain independent. Controls use 6px corners and 44px touch targets. The existing 1240px container is restored. Icons retain existing meanings. Hero images receive no tint. The first story is immediate, later imagery loads on user selection. No autoplay.

Resolved baseline issues: closed mobile filters in the tab order, sheet/focus loss after updates, source/runtime typography mismatch, duplicate suggestion rendering and a large icon TTF. Wider coverage also found and fixed dark-panel contrast, Tracking/Payments reflow, unfocusable seller Returns, semantic operational tables and narrow edit-media layout. Source/live comparison uses the isolated native build rather than the cached application at 8080.

## Evidence key and scope

The QA report bundle is `docs/qa/UI-UX-20261014/`. Full working captures remain in the local UI artifact directory; representative concepts/screenshots and result reports are included in that bundle. These are fictional demo fixtures, never production customer data.

- E1: `verification.json`: 127 route visits, 889 seven-width checks, zero reported automated violations; 20 Chromium and 13 each Firefox/WebKit core checks.
- E2: `shopping-verification.json`: three engines; chosen option, drawer/Escape, serialized row changes, current server total, removal focus, Wishlist and Back/query.
- E3: `checkout-verification.json`: local simulation only; double activation creates one order, persisted confirmation/refresh, failed capture remains pending and retry reuses that order.
- E4: `recovery-verification.json`: search/error, text spacing/enlargement, short viewport, forced colors/reduced motion, no-JS filtering, browser-only multi-media fixture, upload failure, native role fields, notification ownership and stock rejection.
- E5: Go presentation/handler suites; existing header (three engines, two account states, nine widths), four current role tests and focused seller save/archive regressions. Historical suites guarded to a different disposable database or retired roles were not forced against this setup.
- E6: `baseline-final-measurements.json` and `current-final-measurements.json`: repeatable cold/immediately-warm 390x844 lab runs, 150ms latency, 1.6Mbps download and 4x CPU. Long tasks/request counts/input sample included. Not field p75 or INP.

Browser coverage samples every connected current screen family and existing aliases, with owned detail records and explicit edge fixtures. Different records sharing a template are not separate screen designs; no claim is made that every production record or provider state was exercised. Native HTTP error/download responses remain server-owned. Automated results are coverage evidence, not accessibility certification.

## Task status

Statuses: Implemented and verified (covered presentation/states); Existing presentation verified; Conditional omitted (reason recorded); External verification required (implemented browser behavior, incomplete device/integration sign-off); Measured, field gate remains. Existing behavior is preserved where already suitable. Human study, physical-device and screen-reader work is not claimed from browser emulation.

All 140 task IDs are accounted for: 109 implemented and verified, 17 existing presentation verified, four conditional omissions, seven external verification gates and three measured tasks with field gates. This is not a claim that external certification or participant research is complete. The roadmap remains the original task specification; this ledger is the implementation-status record.

| Task | Status | Evidence / remaining check |
| --- | --- | --- |
| F01 | Implemented and verified | Source-aligned native preview and original-revision baseline; isolated QA data only (Build record). |
| F02 | Implemented and verified | Current route registry inventoried; 127 visits across customer, seller, admin, super-admin and auth screens; no retired roles restored. |
| F03 | Implemented and verified | Accepted palette, 1240px container, gutters, 6px controls and shared semantic tokens (DESIGN.md). |
| F04 | Implemented and verified | DM Sans/Playfair storefront; Inter/Roboto portals; matching versioned WOFF2 preloads; currency/numeral checks and full font fallback. |
| F05 | Implemented and verified | Native controls, labelled fields, shared semantic states, confirmations, media and card contracts; E1/E4/E5. |
| F06 | Implemented and verified | Consistent action labels; unknown/multiple options navigate safely; misleading offer/verification claims removed; E2/E3. |
| F07 | Implemented and verified | Editorial storefront versus compact operational queues; native tables retain all mobile data/actions; E1. |
| F08 | Implemented and verified | Four pre-implementation composition references and shared desktop/mobile component spec; real assets/data override illustration (Fidelity review). |
| N01 | Implemented and verified | Existing header regression: three engines, guest/customer, nine widths, sticky positioning and readable controls; E5. |
| N02 | Implemented and verified | Shallow Shop/Account/Help drawer; category/brand links use existing query contracts; E1/E5. |
| N03 | Implemented and verified | Native customer dialogs plus role navigation inertness, Tab/Escape, restoration and scroll lock; three engines; E1/E5. |
| N04 | Implemented and verified | Mobile search expansion retains Cart; clear/Escape/profile dismissal checked at nine widths; E5. |
| N05 | Implemented and verified | Combobox keyboard selection, selected option, clear and Escape; fixed inherited fragment selector; E1/E4. |
| N06 | Implemented and verified | Existing 300ms debounce, replacement/cancellation and IME guards retained; no second suggestion renderer; E1/E4. |
| N07 | Implemented and verified | No matches, intercepted 503, retained query, Enter recovery and keyboard results; E4. |
| N08 | Implemented and verified | Native Back retains category/sort URL; HTMX filter updates preserve selection, focus and local state; E1/E2. Browser-native restoration retained. |
| N09 | Implemented and verified | Existing account/notifications/Wishlist/Cart/logout hierarchy and current role destinations preserved; E1/E5. |
| N10 | Conditional omitted | Optional device-local recents omitted: no demonstrated usability benefit; avoids extra storage and clutter. |
| H01 | Implemented and verified | Compact copy/image story and discovery rail; mobile/desktop screenshots reviewed (Fidelity review). |
| H02 | Implemented and verified | Three maintained craft stories, each with one headline/CTA and an existing destination. |
| H03 | Implemented and verified | Labelled previous/next/select controls, count, pressed state, inactive links inert; keyboard tested; E1. |
| H04 | Implemented and verified | Manual default; no rotation timer, autoplay, video autoplay or restart ambiguity; E1/E4. |
| H05 | Implemented and verified | Reserved geometry; first image eager/high priority; later images deferred; no-JS first story verified; E1/E4. |
| H06 | Implemented and verified | Compact Arts/Crafts/Textiles links retain supported catalogue mapping; E1/Fidelity review. |
| H07 | Implemented and verified | Bounded eight available real products; no fabricated Best sellers/Trending labels; source and E1. |
| H08 | Implemented and verified | Textile/home/craft-giving edits use maintained images and existing categories, without item-level gift eligibility claims. |
| H09 | Implemented and verified | Real maker description/technique fields; sparse content conditional; active seller rather than invented verification; maker detail audited; E1. |
| H10 | Implemented and verified | Shortened brand/footer copy; maintained policy/shipping/support links; E1. |
| D01 | Implemented and verified | Compact title/count/Filter/Sort toolbar; 320px and seven-width matrix; E1. |
| D02 | Implemented and verified | Two columns when usable, one at narrow width, four desktop; enlarged-text/reflow checks; E1/E4. |
| D03 | Implemented and verified | Square media, readable name/maker/price, real optional reviews/availability; showAction respected; E1/E2. |
| D04 | Implemented and verified | Closed sheet immediately hidden/inert; background isolated and focus contained; no-JS alternative; E1/E4. |
| D05 | Implemented and verified | Continuous selections retain sheet, groups, facet query, scroll and focus after HTMX settlement; three engines; E1. |
| D06 | Implemented and verified | Existing removable chips keep other state and reset page; source/rendering tests and Back checks; E1/E2. |
| D07 | Implemented and verified | Local-only facet filtering, no-match message, clear/reset; no unnamed-input remote update; E1. |
| D08 | Implemented and verified | Native labelled sort, selected query state, Clear all and no-JS Apply; E1/E4. |
| D09 | Implemented and verified | Prior results remain; busy/count/error feedback, retained filters and in-sheet Retry after 503; E1. |
| D10 | Implemented and verified | Existing finite pagination and URL contracts retained; no infinite scroll; rendering/history checks; E1/E2. |
| D11 | Implemented and verified | Existing catalogue/category/no-match states retained; filter/search failures now recover in place; E1/E4/E5. |
| D12 | Conditional omitted | Optional quick view omitted: detail is the choice owner; avoids duplicate media/option UI and unintended variant purchase. |
| P01 | Implemented and verified | Image-first mobile flow, compact decision copy and current price/actions; E1/E2/Fidelity review. |
| P02 | Implemented and verified | Named thumbnails, selected state, count, previous/next, keyboard and supplementary swipe; E4 browser-only media fixture. |
| P03 | Implemented and verified | Native lightbox with Close/Escape/trigger restoration; browser pinch zoom remains enabled; E1. |
| P04 | Implemented and verified | First media eager, later photos/video deferred; native playback/no autoplay, pause on leaving, video failure/Retry; E4 presentation fixture. Live storage gate below. |
| P05 | Implemented and verified | Actual options update selected price, SKU, availability and both action regions; unavailable option disabled; E1/E2. |
| P06 | Implemented and verified | Seller-authored specifications/description retained; absent fields not invented; detail/maker source and E1. |
| P07 | Implemented and verified | Labelled quantity buttons and duplicate-safe purchase/cart handlers; real stock failure retains choice/quantity; E2/E4. |
| P08 | Implemented and verified | Selected-price bar respects safe areas, entry focus and inline/policy visibility; short viewport/focus checks; E1/E4. Physical keyboard/zoom gate below. |
| P09 | Implemented and verified | Local stable pending state; confirmed success; intercepted stock failure retains option/quantity and re-enables action; E2/E4. |
| P10 | Implemented and verified | Existing delivery label and shipping/returns/size links; no new PIN/ETA service or invented guarantee; E1. |
| P11 | Implemented and verified | Supplied description/specifications/published reviews in labelled keyboard tabs; Home/End/Arrow behavior; E1. |
| P12 | Implemented and verified | Existing real related/maker data plus Share URL/clipboard feedback; no price/private-data recents cache added. |
| C01 | Implemented and verified | Confirmed Wishlist styling/pressed labels synchronized across views; save/remove round trip; E2. |
| C02 | Implemented and verified | Cart drawer gives Continue shopping/View cart/Checkout, without forced page change on Add; E2. |
| C03 | Implemented and verified | Native drawer keyboard/Escape, authoritative rows/summary, empty/loading/error handling; E2/E5. |
| C04 | Implemented and verified | Server-rendered rows reused; actual option/quantity/price; photo/maker enhanced from existing product response; E2. |
| C05 | Implemented and verified | Per-row serialization/disabled peers, one request on rapid changes and deliberate removal focus; E2. |
| C06 | Implemented and verified | Server rejection remains authoritative; usable rows/choices retained, no guessed recalculation or stock cap; E2/E4. |
| C07 | Implemented and verified | Cart totals copied from current server-rendered summary; subtotal/discount/shipping/total not computed in client; E2/E3. |
| C08 | Implemented and verified | Empty/last-item state retains shopping link, no visible empty Checkout summary; E2/E5. |
| C09 | Conditional omitted | Unsupported Undo/direct Wishlist Move to cart omitted; View product allows correct option selection. Existing variant-safe cart-to-Wishlist contract retained. |
| K01 | Existing presentation verified | Existing four steps retained with compact progress/current-step state and mobile presentation; E1/E3. |
| K02 | Implemented and verified | Back/Edit keep field values; first invalid and destination heading receive focus; E1/E3. |
| K03 | Existing presentation verified | Existing saved/new address selection/autofill/clear contract retained; presentation and source tests; E1/E5. |
| K04 | Implemented and verified | Persistent labels, native metadata, inline error references and first-invalid focus; E1/E3/E5. |
| K05 | Existing presentation verified | 16px entry controls, natural field order, optional labels; seven-width/short viewport checks; E1/E4. |
| K06 | Existing presentation verified | Existing authoritative quote/coupon contract retained; current cart total assertion and checkout review; E2/E3. No client arithmetic introduced. |
| K07 | Implemented and verified | Local simulation disclosed; failed capture stays pending, retry reuses saved order; E3. Real-provider integration is an external gate. |
| K08 | Implemented and verified | Immediate duplicate guard; one creation on double activation; saved order reused after payment failure, no fake success; E3. |
| K09 | Implemented and verified | Persisted confirmation, actual number/status/progress, View order/Continue shopping and refresh; E3. |
| K10 | Existing presentation verified | Current mandatory login preserved, Escape reaches sign-in; existing visibility/paste/autocomplete/errors/redirects retained; auth matrix and E5. |
| U01 | Implemented and verified | Native compact account disclosure and current section; seven-width matrix; E1. |
| U02 | Existing presentation verified | Shared labels/validation, long-value wrapping and existing profile/address/preferences save contracts retained; E1/E5. |
| U03 | Implemented and verified | Read/Unread text, same polling contract, ownership-scoped action and foreign-owner 404; E1/E4. |
| U04 | Existing presentation verified | Existing actual payment/fulfilment states remain separate; owned order detail and list audited; E1/E3. |
| U05 | Existing presentation verified | Recorded delivery events only, unknown times remain explicit; tracking and owned order views audited; E1. |
| U06 | Existing presentation verified | Existing eligible review/return controls remain server-owned; account reviews/returns/order presentations and Go checks; E1/E5. |
| U07 | Existing presentation verified | Customer conversation, internal notes and seller confirmation boundaries retained; shared readable forms/media feedback; owned company case audited; E1/E5. Storage gate below. |
| U08 | Existing presentation verified | FAQ/contact/shipping/size/Terms/Privacy/about/support routes audited; native disclosures/headings/links retained; E1. |
| S01 | Implemented and verified | Current seller shell inert/trapped mobile navigation and desktop restoration; no Team; E1/E5. |
| S02 | Existing presentation verified | Existing live summaries/bounded queues and View all destinations retained and audited; E1. |
| S03 | Implemented and verified | Editor fieldsets, legends, 16px controls, media instructions and validation; failed save retains values; E1/E4/E5. |
| S04 | External verification required | Bounded image/video previews, filename/size/type feedback, current-photo indication and retained upload retry; E4. Unsupported ordering omitted; real storage gate required. |
| S05 | Implemented and verified | In-memory dirty form tracking, Keep editing/Discard and browser warning; no autosave service/storage added; E1/E5. |
| S06 | Implemented and verified | Native inventory/return/ledger/item tables with mobile labels; existing stock/fulfilment/paise rules preserved; E1/E5. |
| S07 | Existing presentation verified | Existing earnings and separate private support/confirmation presentation retained with shared readable fields/statuses; E1/E5. Full storage/provider states depend on configuration. |
| O01 | Implemented and verified | Only permitted current destinations and active states; protected paths stay 403; E1/E5. |
| O02 | Existing presentation verified | Existing live actionable summaries and bounded previews retained; decorative heading art suppressed; E1. |
| O03 | Implemented and verified | Native data tables, scoped headers, named focusable overflow, complete mobile labels; including directory/inventory/settlement/configuration; E1/E4. |
| O04 | Existing presentation verified | Existing native search/status/query contracts retained; clear control and shared pending/failure form feedback; E1/E5. |
| O05 | Implemented and verified | Cancel-first specific confirmations, Escape/restoration, protected-account server rules unchanged; archive no-mutation regression; E5. |
| O06 | Existing presentation verified | Customer reply/internal note/private seller contexts remain distinct; actual action fields/status ownership preserved; owned case audited; E1/E5. |
| O07 | Existing presentation verified | Long identifiers/reasons/native account fields, permission summary and audit reflow; all current super-admin routes audited; E1/E4/E5. |
| A01 | Implemented and verified | Covered core journeys use keyboard controls, visible focus and skip links; modal traps/step focus tested in three engines; E1/E4/E5. |
| A02 | Implemented and verified | Names and expanded/pressed/selected/current states checked by axe and focused regressions; E1/E4/E5. |
| A03 | Implemented and verified | Native navigation/control semantics; custom all-link interception removed; standard new-tab/middle-click preserved; E1/E2. |
| A04 | Implemented and verified | Rendered text axe checks plus corrected dark-panel/status/control contrast; E1. Browser audits do not certify every media/transient state. |
| A05 | Implemented and verified | 44px shared controls/table actions, readable native entries and touch alternatives; E1/E4/E5. Comfort target, not a blanket WCAG target-size claim. |
| A06 | External verification required | 320px reflow and CSS text-enlargement/640px 400%-equivalent tests pass; actual browser-native zoom remains an external check; E1/E4. |
| A07 | Implemented and verified | Manual-only stories/video, reduced-motion overrides and immediate keyboard semantics; E1/E4/E5. |
| A08 | Implemented and verified | Native/role/filter dialogs isolate background, trap/Escape/restore; focused policies clear sticky controls; E1/E4/E5. |
| A09 | Implemented and verified | Polite result/save/cart/notification states and persistent errors; actual response controls success; E1/E2/E3/E4. |
| A10 | Implemented and verified | Semantic headings/landmarks, media alternatives, field error references and native table relationships; E1/E4/E5. |
| A11 | Implemented and verified | Native zoom/paste/password-manager metadata retained; non-drag controls and no hover-only core action; E1/E4/E5. |
| A12 | External verification required | Forced colors/text spacing browser checks pass. Actual NVDA/VoiceOver/TalkBack and media accessibility review remain external; E4. |
| R01 | Implemented and verified | Seven required widths across current route families, plus landscape/enlarged text; E1/E4. |
| R02 | External verification required | 390x360 and focused field/bar handling verified; real on-screen keyboards remain a physical-device gate; E4. |
| R03 | External verification required | Safe-area/dynamic viewport CSS and internal sheet scrolling implemented; physical browser chrome/insets remain a device gate; E1/E4. |
| R04 | Implemented and verified | One contextual purchase region yields to inline/policy/entry focus; no extra permanent bottom nav/popups; E1/E4. |
| R05 | Implemented and verified | Supplementary swipe and native page scrolling with visible gallery buttons; no drag-only path; E4. |
| R06 | Implemented and verified | Long product names, identifiers, emails, prices and error copy wrap; narrow edit/media overflow fixed; E1/E4. |
| R07 | Implemented and verified | Zero horizontal page overflow in covered route/width matrix; real tables own labelled focusable regions; E1. |
| R08 | External verification required | Chromium/Firefox/WebKit covered; native iOS Safari and Android Chrome controls/Back remain external. Desktop engines are not physical phones. |
| M01 | Implemented and verified | 120ms press / 160ms checkout / 180ms sheets / 250ms story opacity vocabulary; DESIGN.md and reduced-motion checks. |
| M02 | Implemented and verified | Fine-pointer press/hover only; geometry stable; keyboard transitions immediate; E1/E5. |
| M03 | Implemented and verified | Wishlist pressed/label styling changes only after confirmation, independent of animation; E2. |
| M04 | Implemented and verified | Origin-aware disclosures/sheets and repeated close/open tests; inert/scroll state restored; E1/E5. |
| M05 | Implemented and verified | Stable text plus busy indicator, duplicate guards, clear on failure; no fabricated percentage; E3/E4/E5. |
| M06 | Implemented and verified | Fixed media frame, deferred source, stationary controls and actual video load/error feedback; E1/E4. |
| M07 | Implemented and verified | Restrained suitable-pointer card/link affordances; no invented second image; source/Fidelity review. |
| M08 | Conditional omitted | Optional editorial scroll reveal omitted: content is immediately visible; no delayed product grid or forced scroll. |
| V01 | Measured; field gate remains | Original-revision baseline cold and immediate warm runs on five routes, fixed profile and same isolated dataset; E6. |
| V02 | Implemented and verified | 36 proportional maintained WebP variants plus existing 480px candidates; authenticated/custom media contracts unchanged. |
| V03 | Implemented and verified | Explicit square/intrinsic media geometry, closed-filter initial layout and no initial cart-row replacement; E1/E6. |
| V04 | Implemented and verified | First story/main product prioritized; later story/photo/video deferred; E1/E4/E6. |
| V05 | Implemented and verified | Deterministic WOFF2, currency/numeral checks, smaller portal fast path with full fallback, 354-ligature icon subset and matching versioned preloads. |
| V06 | Implemented and verified | Unused duplicate JSON rendering/navigation/search handlers and contradictory grid rules removed after source consumer checks; intentional portal styles retained. |
| V07 | Implemented and verified | Optional gallery/media behavior scoped to matching DOM; no new animation framework or provider changes; existing Firebase/auth behavior retained. |
| V08 | Implemented and verified | Idempotent initialization and filter/purchase cleanup; startup reads cancelled before teardown, POST outcomes unchanged; E1/E2/E4. |
| V09 | Implemented and verified | Short opacity/transform feedback, no heavy blur or decorative loops; native browser mechanisms retained; E1/E6. |
| V10 | Measured; field gate remains | Cold/warm bytes, requests, LCP/CLS, long tasks and lab interaction sample measured; not field p75/INP (E6). |
| Q01 | Implemented and verified | Changed-component state matrix below: normal/pending/error/empty/unavailable/invalid/recovery coverage and configuration gates. |
| Q02 | Implemented and verified | Isolated real options, sold-out/long-name/missing-media fixtures and successful shopping/local checkout; E1/E2/E3/E4. |
| Q03 | Implemented and verified | Existing header/role/focused seller regressions plus Go suites and three-engine focused checks; E1–E5. Old guarded datasets/retired-role suites not forced. |
| Q04 | Implemented and verified | Four coordinated references compared with desktop/mobile captures; intended differences and unresolved external gates recorded (Fidelity review). |
| Q05 | External verification required | Axe plus keyboard/reflow/motion/spacing/forced-color checks completed; native zoom, phones and screen readers remain external; E1/E4/E5. |
| Q06 | Implemented and verified | templ generation, CSS minification/build, asset generation, Go checks and presentation-only diff audit; E5. |
| Q07 | Measured; field gate remains | Repeatable slower connection/CPU cold and warm lab measurements; production media/field percentile requires separate data; E6. |
| Q08 | Implemented and verified | Five internal task evaluations mapped below to browse/options/Wishlist/cart/checkout/aftercare evidence; no recruited users, analytics or conversion claim. |
| Q09 | Implemented and verified | All 140 IDs mapped to evidence, route coverage, screenshots, build/data scope, conditional omissions and external gates. |

## Verification history

- Initial working tree: only existing .codebase-memory changes and the roadmap. These are preserved.
- Codebase-memory skill read. Its graph tools are unavailable in this session; targeted source searches are the documented fallback. No graph completeness claims are made.
- Original-revision baseline and current native builds use separate ports against the same dedicated QA database; active local data was not seeded/reset.
- All presentation/handler Go suites, existing current-role/header checks and focused seller regressions passed. Generated sources/CSS and maintained media/font builds completed.
- Search inheritance was corrected using HTMX's documented [hx-select child override](https://htmx.org/attributes/hx-select/); no second suggestion service/renderer was introduced.
- Browser failures were investigated before acceptance: sheet settlement, IME/clear behavior, missing-image recovery, semantic tables, reflow and navigation teardown reads. Reports contain final outcomes, not claims based only on code presence.
- The final 127-route/889-width Chromium matrix and WebKit core checks passed. A late Firefox recovery timeout interrupted later navigation checks; all 13 checks passed in an isolated Firefox repeat on the same preview, without source changes. `verification.json` explicitly aggregates those runs; the initial failed report, repeat report and failed-check metadata are retained in the QA bundle. This is not a claim that the earlier run passed.

## Changed-component state matrix

| Surface | States exercised | Gate / deliberate boundary |
| --- | --- | --- |
| Header, profile, navigation | guest/customer, desktop/mobile, expanded/collapsed, repeated open, Escape/Tab, backdrop, reduced motion | Current mandatory login unchanged; physical phone chrome remains external. |
| Search and filters | idle, typing/IME, suggestions, no matches, submitted query, pending, superseded request, 503, clear/retry, continuous selection, Back | No device-local recents; no extra facet endpoint. |
| Hero | first/default, three manual stories, selected/inert state, keyboard, deferred photos, reduced motion, no-JS | No autoplay or campaign backend. |
| Detail/media/options | single/multiple/unknown options, unavailable stock, long text, missing image, zoom, tabs, swipe/button alternative, deferred video/error/retry | Video uses a browser-only fixture; real uploaded media and caption/audio accessibility need configured storage/content review. |
| Cart/Wishlist | server-confirmed toggle, chosen option, pending/rapid activation, current totals, remove/focus/last item, empty and stock rejection | No guessed variant, optimistic success, client total, timed Undo or automatic recommendation loop. |
| Checkout | existing address/new input, invalid correction, Back/Edit, four steps, review, duplicate Place order, local capture failure/pending/retry, refresh | No real charge; live provider/coupon/storage combinations need deployment-specific sign-off. Existing idempotency metadata contract preserved. |
| Editor and operational forms | grouped fields, first-invalid focus, rejected save/retained values, unsaved Keep editing, file type/preview/prepare failure, cancel-first archive | No backend autosave; ordering service absent; no storage bucket provisioned. |
| Account/portals/support | default/empty/query layouts, read/unread, foreign-owner rejection, current role boundaries, protected account fields, owned company case, narrow table/record views | Internal notes/private seller messages remain separate; no notifications/emails sent externally. |

## Universal screen applicability

Shared foundation, semantics, fields, focus, feedback, motion and reflow rules apply to Customer, Seller, Admin and Super Admin. Customer discovery/purchase patterns are not copied into operational queues. Current directories, marketing/analytics, finance/reconciliation, settings, security/audit/health aliases, account/support/policy and editor/detail screens are included in the route-family audit. Retired Team/operator surfaces are excluded rather than reintroduced.

## Build record and reproducibility

Base revision: `d312f1f824ce3146cc9f236a0f8031b784a7b925` plus this UI-only working tree. QA app: `http://127.0.0.1:18080`; original-revision baseline: port 18081. Dedicated PostgreSQL database `weev_qa_ui_20261014`, container `weev-ui-qa-20261014-postgres`, host port 15432; dedicated Redis host port 16380. Both are distinct from the active application's data/services. Fixture preparation refuses an unexpected database port. Local payments only; real provider/email credentials are cleared by the runner. Test activity creates only isolated demo fixtures, cart changes, simulated orders and notification reads.

Final native preview executable SHA-256: `21bf9bfea3b31c3afd61f5a61370a682b00e4f3819390432f2a01e27d4f2dee4`. Local verification uses Go 1.27.1, the repository-pinned templ v0.3.1020 and Node 24.21.0. `templ generate -check` reports zero updates. This executable is a local QA artifact, not a production deployment or a committed binary.

Build: `npm ci`, `templ generate ./web/...`, `npm run css:build`, then normal Go application build. Generated WOFF2/responsive images are committed, so optional asset regeneration is not required to run the app. Regeneration: `npm run assets:images` (pinned build-only Sharp), `npm run assets:fonts` (Python fontTools/Brotli). Original photos/TTFs/licences remain intact. The interface font fast path retains the full original font as fallback; CSS/preload URLs share the content hash.

`css:build` also regenerates the six first-party minified JavaScript files using pinned build-only esbuild; no bundling, property mangling or framework change is introduced. All document families reference their generated assets with content hashes. The maintained image helper upgrades only verified owned candidate sets; seller-uploaded/authenticated/custom srcsets pass through unchanged. Listing's first two images are eager, with the first prioritized; off-screen seller photography is deferred. Unit tests cover maintained legacy candidates and custom-media preservation.

Accessibility scripts resolve pinned test-only axe-core 4.14.0 from installed dependencies, identical to the earlier temporary audit engine. Native select height differs between browser engines; the mobile toolbar regression checks shared row centres, not equal control heights. Local facet fields explicitly stop their change event reaching the server-filter form; their search/clear regression asserts zero result requests. One Chromium navigation reported `ERR_NETWORK_IO_SUSPENDED` during an earlier long audit; nine repeated seller-path visits returned 200 with no page errors, and the full audit was repeated rather than accepting that failed run.

QA artifact path defaults to the OS temporary directory and can be set with `WEEVCRAFTS_UI_ARTIFACTS`. Use the dedicated runner and demo fixtures, never an active business database. The runner's `status` mode is read-only. It does not kill an occupied port or reset data; verify process ownership before restarting a preview. See the QA bundle README for checks and physical-device sign-off.

## Final lab performance

Same isolated dataset and original-revision baseline, 390x844 viewport, 150ms latency, 1.6Mbps download and 4x CPU. Each row is one cold navigation followed by an immediate warm navigation. No concurrent browser tests or builds ran during the final measurement. Reports retain request counts, transfer bytes, long tasks, the LCP element/resource and a lab interaction sample.

| Page | Baseline cold LCP | Current cold LCP | Current warm LCP | Current cold CLS | Transfer reduction |
| --- | --- | --- | --- | --- | --- |
| Home | 5,244ms | 2,216ms | 440ms | 0.031 | 82.6% |
| Listing | 4,080ms | 2,140ms | 596ms | 0.003 | 76.8% |
| Detail | 3,064ms | 2,116ms | 548ms | 0.004 | 81.0% |
| Cart | 2,752ms | 2,216ms | 520ms | 0.090 | 81.3% |
| Checkout | 2,768ms | 1,968ms | 792ms | 0.010 | 79.7% |

All five current cold samples are below the 2.5-second LCP budget and 0.1 CLS budget; warm samples are below 0.8 seconds. Cart CLS increased from the baseline's 0.067 and remains close to the budget, so representative production media/font conditions deserve follow-up. No page errors or horizontal overflow were recorded. Earlier listing/cart speed misses were recorded during diagnosis and corrected by prioritizing listing images, using maintained responsive candidates and delivering minified assets. These final lab samples do not establish production percentiles, field INP or universal device performance.

## Fidelity review

Compared the four generated composition references with 1440px and 390px browser viewport captures. The accepted real logo/header, public maintained images, actual product names/options/prices/reviews, current account policy and role-specific typography take precedence over illustrative logos/copy/data. No concept merchandise or maker biography entered the application.

Home retains compact story/category/product hierarchy and manual controls. Listing retains two readable mobile columns (one when narrow), one toolbar and URL-based filters. Detail retains image-first choice, square object-containing media, real option updates and one contextual purchase region. Checkout keeps four existing steps, labels and authoritative summary; it does not copy the concept's fictitious values or add an extra permanent bottom toolbar. Operational screens use shared semantic components and task density, not editorial carousels. These are intentional departures, not a pixel-identity claim.

## Internal five-task review

| Task | Evidence / outcome |
| --- | --- |
| Browse/filter a piece and return | Continuous selection/clear/query and native Back checked (E1/E2); history contracts retained. |
| Choose a real textile option and inspect it | Large/unavailable option, SKU/price/stock, gallery/zoom and one Add request checked (E1/E2/E4). |
| Save a piece and understand Cart | Confirmed Wishlist round trip, selected-option Cart, current server total, serialized change/remove and focus checked (E2). |
| Complete checkout and recover | Invalid correction/Back, one order on duplicate activation, local failure/pending and same-order payment retry checked (E1/E3). |
| Understand an order and find aftercare | Owned order, real payment/fulfilment/tracking, account reviews/returns/support and owned company case audited (E1/E3/E5). |

This is an internal browser/code task evaluation, not a consented participant study. No user recruitment, analytics or conversion/time improvement claim is made.

## Outstanding release gates

- Physical iOS Safari and Android Chrome, portrait/landscape, dynamic browser chrome/safe areas, on-screen keyboard and native form/Back behavior.
- Actual 200% text/native browser zoom and 400% desktop zoom; CSS enlargement/reflow equivalents passed but do not replace native checks.
- Targeted NVDA/VoiceOver/TalkBack journeys, including tabular mobile summaries, status announcements and uploaded-media caption/audio review.
- Configured storage upload/finalization and real payment-provider handoff/recovery. UI failure/retry and local payment paths were covered; no new backend capability was provisioned.
- Real-user performance percentiles/INP and representative production media; lab results alone do not establish Web Vitals field targets.
- Existing Tailwind 3/source-map development-tool advisories remain; the new build-only Sharp dependency is pinned to 0.35.5. No forced major CSS/toolchain migration is included in this UI-only release.

These gates are documented rather than described as completed. Conditional recents, quick view, direct Wishlist Move to cart/Undo and decorative scroll reveals were deliberately omitted within the roadmap's stated boundaries.
