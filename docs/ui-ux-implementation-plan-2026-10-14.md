# WeeVCrafts UI/UX implementation plan

Planning baseline: 14 October 2026, India time. This document proposes work; it does not record completed implementation or a full accessibility audit.

The intended experience is a calm, distinctive Indian craft marketplace where customers can find, understand, choose and purchase a piece comfortably on a phone. Interaction should make the next action clearer and provide immediate feedback. Product photography and maker information carry the personality; the interface stays restrained.

## Scope and boundaries

Scope confirmation during implementation: the shared foundation, accessibility, responsive, feedback, form, motion and speed requirements apply to **every current Customer, Seller, Admin and Super Admin screen**. Shopping features remain customer-specific; equivalent navigation, form, queue and confirmation requirements apply in each permitted portal. This does not place shopping carousels in operational dashboards or restore retired roles. The [implementation ledger](ui-ux-implementation-ledger-2026-10-14.md) records route-family coverage, state evidence, conditional omissions and external device checks. Different records using the same screen template are represented by owned QA records and explicit edge fixtures, not a claim that every production record was inspected.

Use the existing Go-rendered templ application, Tailwind/CSS, HTMX, Alpine and JavaScript. Presentation work can change templates and their generated Go files, browser scripts, styles and generated CSS, maintained static assets, UI tests and design documentation. Existing HTTP routes, form fields, query parameters, CSRF behavior, authorization, payment behavior and business transitions remain the contract.

Do not change database schemas, SQL, migrations, domain/application services, adapters, API handlers, route authorization, checkout arithmetic, inventory reservation, session rules, notification delivery or deployment configuration. Reading their contracts to understand the interface is appropriate. Rebuilding the existing application to incorporate its presentation assets is an implementation step, not a backend redesign.

Use existing public data and authenticated data within its existing scope. Never synthesize prices, savings, stock, reviews, maker identities, discounts, estimated arrival dates or delivery events. A fast interface can show immediate pending feedback; it must wait for the existing response before claiming a purchase, save or payment succeeded.

Priority: **P0** = usability/accessibility blocker or baseline integrity; **P1** = core shopping improvement; **P2** = supporting polish. **Conditional** = proceed only if the current response, rendered page or existing endpoint already provides the necessary information. Conditional work does not authorize new backend capabilities. All items are proposed until their state matrix and acceptance evidence are recorded.

## Evidence and reference direction

Reviewed the current browser homepage, public catalogue and mobile filter behavior; inspected the current template, script, font and design contracts; read the previous audit and the current four-role contract. The dated audit is historical, not a current defect list. Customer, Seller, Admin and Super Admin are the current roles; retired Team and legacy operator experiences are not being restored.

The local catalogue is empty. Populated product detail, cart, order and checkout states have not been exercised in this planning review. They require existing fixtures in an isolated QA environment or a separate presentation preview, without seeding the user's active data. Mobile review used a 390px viewport. A desktop viewport was requested, but the embedded browser's visible capture was clipped to its panel; full desktop visual acceptance remains future QA work. Real phones, screen readers and performance measurements are not yet verified.

| Before / current evidence | Proposed after | Why / evidence strength |
| --- | --- | --- |
| At 390px, catalogue introduction and separated filter/result/sort controls occupy substantial vertical space before results. | Compact title and one readable browse toolbar, followed promptly by results. | Observed layout; reduce the distance to the shopping task. |
| Keyboard Tab from the closed mobile filter trigger reaches its off-screen Close filters button. | Closed filters are absent from focus and accessibility navigation; open filters manage and restore focus. | Reproduced in the browser; keyboard/focus work is P0. Verify against WCAG 2.1.1 and 2.4.11 in the completed implementation. |
| Selecting Art replaces the catalogue, closes the mobile panel and leaves focus on the body. | Preserve the panel and focus during immediate updates, or use an explicit mobile Apply action. | Reproduced browser behavior; interrupted multi-filter selection is avoidable. |
| Current live preview uses DM Sans/Playfair; checkout source preloads and declares Inter/Roboto in relevant files. | One source-aligned preview and an explicitly agreed font/token specification. | Source/browser discrepancy; the previous preview used a cached image. Do not approve screenshots against a different build. |
| The checked-in Material Symbols TTF is 995,576 bytes. | A small verified icon subset or consistent SVG set. | Raw file size, not compressed transfer size or a measured LCP problem. Measure the real request before choosing the implementation. |
| Existing code already has server-rendered products, media, variant selection, mobile purchase controls, filter URL state, shared form handling and cart feedback. | Refine these components and their edge states. | Source evidence; do not create duplicate competing implementations. |

[Vastramay](https://vastramay.com/) provides useful examples of category/occasion discovery, campaign imagery, product rails and quick-view entry. [Jaypore](https://www.jaypore.com/) provides useful examples of broad category navigation, a craft-focused shopping identity and image-led collections. These observations are references, not evidence that either site meets accessibility or performance targets. Their entire UI, promotional density and assets are not a specification for WeeVCrafts.

WeeVCrafts keeps its accepted ivory, forest green and terracotta identity, serif display headings, legible interface type and maker-to-object storytelling. Concepts will use the real logo and maintained content. Text and shopping controls remain HTML, rather than being baked into banner images. Hero photography retains its existing untinted treatment.

## Detailed implementation backlog

### 1. Foundation and design consistency

- [ ] **F01 · P0:** Build a source-aligned preview; record revision, build and dataset so later screenshots are comparable.
- [ ] **F02 · P0:** Inventory the customer journey and the four current roles, mapping every visible control to an existing destination/action. Identify any UI-only gaps separately from missing services.
- [ ] **F03 · P1:** Consolidate storefront color, spacing, typography, radius, border, elevation and focus tokens around the accepted craft identity; retain intentional portal variants.
- [ ] **F04 · P1:** Resolve documentation/runtime/source font differences; use only needed weights and verify Indian currency symbols, numerals and fallback metrics.
- [ ] **F05 · P1:** Define shared button, input, disclosure, dialog, badge, message, media and product-card variants with default, hover, focus, pressed, pending, disabled, error and success states.
- [ ] **F06 · P1:** Standardize primary/secondary action hierarchy and customer-facing language; use consistent labels such as Add to cart, Continue shopping and View order.
- [ ] **F07 · P1:** Specify separate layout densities for editorial shopping and operational work. Tables and queues should remain task-focused.
- [ ] **F08 · P1:** Produce coordinated desktop/mobile concepts for homepage, listing, product detail and cart/checkout before substantial visual implementation; record their reusable component specification.

### 2. Navigation and search

- [ ] **N01 · P1:** Refine the compact sticky header's alignment, readable logo and breakpoint behavior while preserving its existing core shopping actions.
- [ ] **N02 · P1:** Make category discovery direct through a small set of links and a shallow drawer hierarchy. Match customer-facing labels to supported live catalogue queries.
- [ ] **N03 · P0:** Verify drawer open/close, background inertness, Escape, focus containment, focus restoration and page scroll position across desktop/mobile.
- [ ] **N04 · P1:** Make mobile search easy to recognize and open; expanding it must retain Cart access and offer a clear close/clear path.
- [ ] **N05 · P0:** Refine the existing search combobox: keyboard selection, announced selected option, Escape, clear focus and accessible labels.
- [ ] **N06 · P1:** Preserve the existing request debounce/cancellation contract; prevent superseded suggestions from replacing newer results.
- [ ] **N07 · P1:** Design useful search idle, typing, searching, suggestions, no matches, failed request and submitted-results states. Retry keeps the query.
- [ ] **N08 · P1:** Preserve query, filters, sort, page and useful scroll position through product navigation and browser Back/Forward, with an intentional focus destination.
- [ ] **N09 · P1:** Clarify profile, account, notifications, Wishlist, Cart and logout hierarchy. Keep the accepted logout access and current role-specific destinations.
- [ ] **N10 · P2 · Conditional:** Add a small, dismissible recent-search list stored only on the device, if it proves useful. Never store checkout data, credentials or authenticated responses in browser storage.

### 3. Homepage, slideshow and craft discovery

- [ ] **H01 · P1:** Rebalance the first screen so the offer, one shopping CTA and category access are readable on a phone without a tall introductory stack.
- [ ] **H02 · P1:** Introduce one hero slideshow with at most three maintained craft/collection stories; give each one headline and one destination using existing routes.
- [ ] **H03 · P0:** Give the slideshow labelled previous/next and slide-selection controls, an accessible current-slide state and keyboard operation. Inactive slides must not retain focusable links.
- [ ] **H04 · P1:** Default to manual slide navigation. If automatic rotation is retained, use an approximately 7-second interval, a visible pause control, stop on user interaction/focus and require explicit restart. Disable autoplay for reduced motion and initially on phones.
- [ ] **H05 · P1:** Reserve slideshow height, prioritize only the first image and load later images as needed. Essential links remain available outside the slideshow and the first slide works without enhancement.
- [ ] **H06 · P1:** Keep a compact Arts/Crafts/Textiles discovery rail using verified category mapping and short, useful labels.
- [ ] **H07 · P1:** Present a bounded selection of real available products near the top; use accurate section names instead of unsubstantiated Best sellers or Trending claims.
- [ ] **H08 · P1:** Create a small set of collection edits such as For your home or Gifts with meaning, linking only to valid existing categories/filter combinations. Do not claim item-level gifting eligibility without data.
- [ ] **H09 · P1:** Refine the maker section around actual portraits, technique and story fields. If content is sparse, use a compact honest state rather than a large empty section.
- [ ] **H10 · P2:** Reduce repetitive slogans and lengthy footer/brand blocks; keep policy, delivery and support links easy to find. Editorial depth is available on dedicated pages.

### 4. Product listings, categories and filtering

- [ ] **D01 · P1:** Place a concise title, result count, Filter and Sort in a compact mobile toolbar, with stable layout at 320px.
- [ ] **D02 · P1:** Use two readable product columns where space permits; adapt to one column when narrow widths or enlarged text require it. Desktop column counts follow usable card width.
- [ ] **D03 · P1:** Standardize image ratio, name, maker, price and optional real review/availability information; remove competing badges and repeated buttons.
- [ ] **D04 · P0:** Hide the closed mobile filter panel from keyboard/screen-reader navigation. Use an appropriate dialog/sheet pattern rather than only moving it off-screen.
- [ ] **D05 · P1:** Support continuous multi-filter selection without closing the sheet or losing focus after each request. Preferred default: immediate updates preserving the sheet; evaluate an explicit Apply flow if it simplifies use and reduces requests.
- [ ] **D06 · P1:** Keep active filter chips readable and independently removable, preserving all other query state. Reset page when a filter changes.
- [ ] **D07 · P1:** Retain local-only category/maker facet search; provide clear no-matching-options and reset states without extra remote requests.
- [ ] **D08 · P1:** Make Clear all, selected values and sort state predictable; use the existing labelled native sort select.
- [ ] **D09 · P1:** Design pending result updates without blanking already usable products. Announce the actual final count, preserve focus and provide a retry state.
- [ ] **D10 · P1:** Keep navigable pagination and browser history; make current page and loading states clear. Avoid endless scrolling as the default.
- [ ] **D11 · P1:** Distinguish empty catalogue, empty category, filtered no matches and failed request with the appropriate recovery action and preserved query.
- [ ] **D12 · P2 · Conditional:** Offer quick view only after the main detail flow is strong and existing HTML/data can supply all required choices. Multi-option items must never be added using an unintended variant.

### 5. Product detail and purchase confidence

- [ ] **P01 · P1:** Put product imagery, title, maker, price, availability and the primary purchase action in a clear mobile sequence.
- [ ] **P02 · P1:** Refine the existing gallery with labelled thumbnails, selected state, previous/next buttons, image count and swipe as an additional touch path.
- [ ] **P03 · P1:** Add an accessible image zoom/lightbox with close/Escape/focus restoration. Keep browser pinch zoom enabled; dragging is never the only navigation method.
- [ ] **P04 · P1:** Load active gallery media first; lazy-load additional photos and load existing video only when needed, with native controls and no autoplay.
- [ ] **P05 · P1:** Make the existing variant choice clear and compact; consistently update price, stock, SKU and both desktop/mobile purchase controls.
- [ ] **P06 · P1:** Show optional dimensions, material, technique, care and artisan-variation explanations only where current seller-authored information supports them. Art, textiles and functional objects need different content emphasis.
- [ ] **P07 · P0:** Make quantity controls readable, labelled, keyboard-operable and duplicate-safe. Do not show a client-guessed stock cap as authoritative.
- [ ] **P08 · P1:** Refine the mobile purchase bar around the selected price/action; account for safe areas, keyboard, zoom and focused content beneath it.
- [ ] **P09 · P1:** Give Add to cart and Buy now stable-size pending/success/error feedback. Preserve selected options and quantity on failure.
- [ ] **P10 · P1:** Present the existing delivery label and policy links beside the decision. Do not add a PIN-code availability checker without an existing service or imply a guaranteed arrival date.
- [ ] **P11 · P1:** Make description, specifications and published reviews easy to scan. Keep accessible tabs on desktop and evaluate accordions on phones using the same underlying content.
- [ ] **P12 · P2 · Conditional:** Refine the existing maker/related-product section and optional Share link using existing product URLs. A locally stored Recently viewed rail is optional; revalidate data and hide unavailable entries, never store prices or private details as truth.

### 6. Wishlist and cart

- [ ] **C01 · P1:** Keep Wishlist state synchronized across cards and detail views, with pressed state, explicit save/remove labels and server-confirmed feedback.
- [ ] **C02 · P1:** Make Cart feedback provide one clear next action: Continue shopping or View cart/Checkout, without forcing a full-page interruption.
- [ ] **C03 · P1:** Refine the existing cart drawer's hierarchy, scroll boundaries, keyboard handling, empty state and loading/error state.
- [ ] **C04 · P1:** Make cart rows show the correct photo, item, option, quantity, item price and maker without dense repeated labels.
- [ ] **C05 · P1:** Make quantity/remove pending states local to the affected row; serialize or supersede requests appropriately and retain a focus destination after removal.
- [ ] **C06 · P1:** Handle unavailable or changed-price items using existing server responses, preserving usable items and explaining the next action.
- [ ] **C07 · P1:** Keep an uncluttered summary that clearly separates item subtotal, actual discount, shipping estimate/confirmed charge and authoritative total.
- [ ] **C08 · P1:** Refine empty Wishlist/Cart and last-item removal with useful shopping links; do not leave a Checkout CTA on an empty cart.
- [ ] **C09 · P2 · Conditional:** Offer Move to cart or Undo only where existing actions can safely perform it with the proper option and current stock. Any optional recommendation rail stays below the checkout decision.

### 7. Checkout and authentication presentation

- [ ] **K01 · P1:** Keep the current checkout steps and transaction contract while simplifying their presentation; place the current step and next action within easy mobile reach.
- [ ] **K02 · P0:** Preserve fields when using Back/Edit or encountering validation errors. Move focus to the new step heading without a keyboard animation delay.
- [ ] **K03 · P1:** Clarify saved versus new address selection, account autofill and editable contact fields using the existing address behavior.
- [ ] **K04 · P0:** Provide persistent labels, appropriate autocomplete/inputmode, inline correction text and first-invalid focus. Client validation supplements current server checks.
- [ ] **K05 · P1:** Use readable mobile input sizes and natural field order; make optional fields explicit and avoid forcing users to interpret technical labels.
- [ ] **K06 · P1:** Keep server-confirmed totals and coupon outcomes legible. Applying a coupon preserves input and cannot conceal the updated payable amount.
- [ ] **K07 · P1:** Present the payment mode honestly: provider handoff, unavailable state or local simulation as already configured. Preserve failure/pending/retry routes and saved-order reuse.
- [ ] **K08 · P0:** Make Place order duplicate-safe, clearly pending and recoverable. A client animation never reports payment success before the existing authoritative result.
- [ ] **K09 · P1:** Refine order confirmation around the actual order number, payment status, next step, View order and Continue shopping. Keep it useful after refresh.
- [ ] **K10 · P1:** Refine login/signup/recovery presentation: password visibility with state, password-manager/paste support, current policy hints, existing redirects and clear errors. Do not change login requirements or add guest checkout.

### 8. Account, orders and support

- [ ] **U01 · P1:** Make the mobile account navigation compact, labelled and clear about the current section; keep the section's task visible immediately.
- [ ] **U02 · P1:** Refine profile/address/preferences forms around shared labels, save feedback, validation and readable long values.
- [ ] **U03 · P1:** Make the notification inbox distinguish unread/read with text as well as color; preserve existing ownership-scoped open behavior and background polling contract.
- [ ] **U04 · P1:** Make order cards show the actual payment and fulfilment states separately, with price, date and one primary detail action.
- [ ] **U05 · P1:** Refine order detail and delivery progress using existing recorded events. Unknown delivery time or pending payment must remain explicit.
- [ ] **U06 · P1:** Make existing eligible review and return entry points discoverable, with useful instructions and preserved form data; eligibility stays server-owned.
- [ ] **U07 · P1:** Refine customer support conversations, attachment previews and reply feedback. Never blend internal notes or private seller confirmations into customer content.
- [ ] **U08 · P2:** Refine FAQ, contact, shipping, returns, size guide, Terms and Privacy presentation with scannable headings, accessible disclosures and valid links.

### 9. Seller interface

- [ ] **S01 · P1:** Refine the existing seller shell's responsive navigation, current-page state and primary task access without introducing retired Team workflows.
- [ ] **S02 · P1:** Make dashboard summaries and bounded previews readable; give each queue an existing View all destination.
- [ ] **S03 · P1:** Group the product editor into clear sections with current validation, supported media instructions and optional-field guidance.
- [ ] **S04 · P1:** Improve existing image/video preview, cover selection and gallery ordering presentation. Provide buttons wherever drag ordering is already supported.
- [ ] **S05 · P1:** Keep unsaved form state in memory during local UI transitions and warn before navigation where useful. A server-backed autosave feature is outside this plan.
- [ ] **S06 · P1:** Refine mobile inventory/order layouts, visible row actions and recorded fulfilment forms; preserve rupee/paise conversion and stock rules.
- [ ] **S07 · P1:** Improve existing earnings and private confirmation/support layouts with clear states, reply fields and empty/error feedback.

### 10. Admin and Super Admin interface

- [ ] **O01 · P1:** Refine role-aware navigation and active states using current permitted routes; do not surface controls that require broader permissions.
- [ ] **O02 · P1:** Make operational dashboards prioritize existing actionable queues and bounded previews rather than decorative metrics.
- [ ] **O03 · P0:** Verify semantic tables, explicit headers and keyboard-operable named overflow regions; use mobile row summaries only when all needed data/actions remain available.
- [ ] **O04 · P1:** Refine existing search/filter/status toolbars and empty/no-results/loading/error states, preserving URL/query contracts.
- [ ] **O05 · P0:** Keep destructive-action confirmations specific, focus-safe and faithful to server eligibility, reasons and protected-account behavior.
- [ ] **O06 · P1:** Visually separate customer conversation, internal note and seller confirmation contexts; clarify the effect of existing reply/status actions.
- [ ] **O07 · P2:** Improve Super Admin user/settings/audit layouts for long identifiers, reasons, validation and narrow screens without modifying roles or security rules.

### 11. Accessibility throughout

- [ ] **A01 · P0:** Walk complete journeys using keyboard only, including all overlays and sticky regions; retain visible focus and correct DOM order.
- [ ] **A02 · P0:** Verify accessible names, roles and current/expanded/pressed/selected states for every icon-only or custom control.
- [ ] **A03 · P0:** Use native links, buttons, selects and form controls wherever appropriate; keep middle-click/new-tab behavior for navigation.
- [ ] **A04 · P0:** Measure rendered text contrast and meaningful control/state contrast, including media backgrounds, hover, focus and errors.
- [ ] **A05 · P1:** Aim for 44px touch targets and adequate separation; evaluate WCAG 2.5.8's 24px minimum and exceptions accurately.
- [ ] **A06 · P0:** Verify 320px reflow, 200% text enlargement and 400% desktop zoom, including sticky purchases, long product names and form errors.
- [ ] **A07 · P0:** Respect reduced motion, disable autoplay in that mode and retain immediate semantic feedback.
- [ ] **A08 · P0:** Make dialogs/sheets inert-background, Escape-capable and focus-restoring; prevent focused content being obscured by header/footer/purchase bars.
- [ ] **A09 · P0:** Announce result counts, save/cart outcomes and pending/error states appropriately; avoid repeated or assertive success chatter.
- [ ] **A10 · P1:** Review image alternatives, decorative icons, landmarks, headings and error references; use real table/header relationships.
- [ ] **A11 · P0:** Preserve zoom, paste, password-manager and autocomplete behavior; offer non-drag paths and never require hover for core actions.
- [ ] **A12 · P1:** Test forced colors, text-spacing overrides and targeted screen readers. Record coverage and limitations instead of claiming certification from an automated score.

### 12. Mobile and responsive behavior

- [ ] **R01 · P1:** Design around actual content at 320, 360, 390, 430, 768, 1024 and 1440px, plus portrait/landscape and large text. Breakpoints follow content fit.
- [ ] **R02 · P1:** Verify short viewports and the on-screen keyboard: active field, inline error and primary action remain reachable.
- [ ] **R03 · P1:** Account for safe-area insets and dynamic browser chrome in sheets and purchase controls; overlays have reliable internal scrolling.
- [ ] **R04 · P1:** Keep only one contextual bottom action region; avoid stacking permanent bottom navigation, purchase bar, chat bubble and promotional overlays.
- [ ] **R05 · P1:** Use touch-friendly gallery/rails with visible button alternatives; normal vertical page scrolling must remain comfortable.
- [ ] **R06 · P1:** Wrap long names, emails, IDs, INR prices and validation text without clipping; use tabular numerals where alignment matters.
- [ ] **R07 · P1:** Keep page content free of horizontal overflow; confine real two-dimensional table data to labelled, operable regions.
- [ ] **R08 · P1:** Test native iOS/Safari and Android/Chrome form controls and Back behavior; hover enhancements apply only to suitable pointers.

### 13. Motion and microinteractions

- [ ] **M01 · P1:** Define one motion vocabulary with exact properties/easing, purpose and reduced-motion behavior; extend the existing 120ms press/160ms step conventions.
- [ ] **M02 · P1:** Add subtle pointer press feedback without layout shift; keyboard activation remains immediate.
- [ ] **M03 · P1:** Refine Wishlist feedback after a confirmed response; its label and selected state change even when motion is disabled.
- [ ] **M04 · P1:** Refine drawer/profile/sheet transitions around their origin, clear state and interruptibility. Repeated open/close must not leave stale overlays.
- [ ] **M05 · P1:** Keep busy indicators inside stable-size controls; clear them on all result paths. A spinner is not a fabricated progress percentage.
- [ ] **M06 · P1:** Refine slideshow/gallery transitions without gating interaction or flashing unready media; keep controls stationary.
- [ ] **M07 · P2:** Use restrained desktop hover/focus affordances on product links and cards; an alternate image is optional only when an actual second image exists.
- [ ] **M08 · P2:** Consider a one-time editorial section transition only when it improves continuity. Product content stays visible before JS runs; no delayed grid reveal or forced scroll effects.

### 14. Frontend speed and stability

- [ ] **V01 · P0:** Measure cold and warm homepage, listing, detail, cart and checkout loading before changes under a fixed mobile/network test profile.
- [ ] **V02 · P1:** Right-size maintained static images and use existing product image variants/srcset where available. Do not introduce a new media-generation backend.
- [ ] **V03 · P1:** Use explicit width/height or aspect ratios, correct sizes and reserved skeleton geometry so photos and feedback do not move the layout.
- [ ] **V04 · P1:** Prioritize only the visible hero/active main image; lazy-load below-fold photos and later slides. Do not lazy-load the LCP image.
- [ ] **V05 · P1:** Subset/convert required fonts to WOFF2 where practical and replace/subset the large icon font; preserve glyph coverage, licensing and stable fallback metrics.
- [ ] **V06 · P1:** Remove demonstrably unused duplicate style rules, font declarations and presentation handlers after consumer checks; retain intentional role styles.
- [ ] **V07 · P1:** Scope optional gallery/editor/campaign behavior to pages that need it. Audit guest-page Firebase and storefront script cost without changing sign-in, notification or existing provider behavior.
- [ ] **V08 · P1:** Keep event initialization idempotent after HTMX swaps, clean up obsolete listeners/observers and avoid fetching data already usable in the rendered page.
- [ ] **V09 · P1:** Use short opacity/transform transitions, batch DOM reads/writes and avoid heavy blur, continuous scroll handlers or unnecessary animation libraries.
- [ ] **V10 · P1:** Measure each increment against baseline, including request counts, scripting, long tasks, input response and carousel asset cost. If server response time dominates, record it as a backend limitation rather than disguising it with animation.

### 15. Verification and delivery

- [ ] **Q01 · P0:** Create a state matrix for each changed component: default, pending, success, unavailable, empty, no matches, invalid, failed request and recovery, as applicable.
- [ ] **Q02 · P0:** Exercise the existing shopping journey with single/multiple variants, sold-out stock, long labels and missing media in an isolated existing test setup.
- [ ] **Q03 · P0:** Run existing header, customer, role, checkout and relevant browser checks for changed surfaces; add focused regressions for actual behavior defects.
- [ ] **Q04 · P1:** Compare desktop/mobile implementation against the coordinated concepts for copy, spacing, layout, typography, tokens, media and control states.
- [ ] **Q05 · P0:** Perform targeted automated accessibility checks plus manual keyboard/zoom/motion/screen-reader verification. Report precisely what was and was not tested.
- [ ] **Q06 · P1:** Re-run CSS build/minification and templ generation as needed; verify the diff contains only presentation artifacts, tests and documentation.
- [ ] **Q07 · P1:** Run a consistent mobile performance audit after each major slice and a final slower-device/connection review.
- [ ] **Q08 · P1:** Evaluate five realistic tasks internally; an optional consented usability study can follow without contacting users automatically or installing analytics.
- [ ] **Q09 · P1:** Maintain an implementation ledger with task IDs, screenshots, tested build, functional evidence and remaining limitations. Review each slice before expanding scope.

## Interaction specification

| Interaction | Proposed behavior | Time / constraints |
| --- | --- | --- |
| Pointer press | Small existing-style press response, fixed geometry. | About 120ms; keyboard immediate. |
| Wishlist | Pending state immediately; saved styling after confirmation. | Brief 160–220ms feedback if motion permitted. |
| Profile disclosure | Open from trigger, predictable dismissal and focus. | About 150–180ms; no delayed access. |
| Cart/filter sheet | Clear entry/exit, inert background and restored focus. | About 180–240ms; keyboard/reduced motion immediate. |
| Checkout step | Preserve values and focus the next heading. | Existing pointer transition around 160ms. |
| Hero slide | Crossfade or restrained transition with fixed geometry. | Around 250–300ms; manual default. |
| Search | Existing debounce, replace superseded requests, keyboard selection. | Preserve current approximately 300ms debounce unless measurement supports adjustment. |
| Result update | Keep previous content usable, local busy status, announce final count. | No intentional delay or fake loading minimum. |
| Error/action message | Persistent inline notice or dismissible accessible message. | No disappearing error or timed-away Undo. |

These durations are proposed product choices, not accessibility thresholds. Prefer existing CSS/Alpine/HTMX mechanisms; a new animation framework is not required. [WAI's carousel guidance](https://www.w3.org/WAI/tutorials/carousels/) informs user controls, focus and accessible slide changes.

## Accessibility and performance acceptance

Target [WCAG 2.2 AA](https://www.w3.org/TR/WCAG22/) across complete affected journeys. Normal text contrast is at least 4.5:1, large text 3:1 and meaningful UI/state contrast 3:1 where the criteria apply. The 44px touch size is our comfort target; the AA target-size criterion uses 24px with exceptions. Reflow, resize, input labels, focus visibility, focus not obscured, status messages and accessible authentication are explicit acceptance checks. Automated audits do not establish full conformance.

Performance goals are **LCP <= 2.5s, INP <= 200ms and CLS <= 0.1**, assessed at the 75th percentile separately for mobile and desktop when real-user measurements are available, following [Google's Web Vitals guidance](https://web.dev/articles/vitals). These are goals, not measured results for the current application. Local lab testing uses a repeatable throttled profile; it cannot claim the field percentile. If server/network time prevents a target, report that boundary transparently.

Initial frontend budgets to confirm after the baseline: no more than about 50KB compressed incremental JavaScript for the first storefront redesign; maintained mobile hero imagery generally 150–250KB when visual quality allows; no eager downloads for inactive slides/video; no additional requests caused by a decorative loop. These are provisional engineering budgets, not standards. Existing production media may need different treatment within the current delivery capability.

No new tracking integration is required. Use existing test tooling, browser performance inspection and available field data. UI changes should reduce unnecessary bytes and work while retaining the rendered content's immediate usefulness.

## Implementation sequence and review gates

| Phase | Work | Exit evidence |
| --- | --- | --- |
| 0 — Baseline | F01–F02, V01, existing contract/role check, isolated QA-state inventory. | Source-aligned preview; screenshots; state map; baseline measurements; unchanged backend contract. |
| 1 — System and urgent usability | F03–F08, D04–D05, essential A/N work; short desktop/mobile concept specification. | Closed filters leave the tab order; continuous selection/focus works; agreed tokens/components; narrow/keyboard checks pass. |
| 2 — Discovery | Header/search, homepage/categories, compact listing, supported filters and product cards. | Find/browse/filter/back journey passes; slideshow is controlled; empty/error states work; no measured performance regression. |
| 3 — Product decision | Gallery/zoom, variants, information hierarchy, reviews, Wishlist and mobile purchase bar. | Choose a real option and reach Cart safely on phone/keyboard; unavailable/media-error states pass. |
| 4 — Purchase and aftercare | Cart, checkout presentation, confirmation, account, orders, support and policy layouts. | Existing end-to-end shopping/recovery path passes with unchanged requests, totals and payment states. |
| 5 — Workspaces | Seller, Admin and Super Admin visual/task improvements. | Existing permission/role checks pass; mobile forms/tables work; private conversation boundaries remain intact. |
| 6 — Completion review | Cross-browser/device/accessibility/performance evidence and remaining consistency work. | Fidelity ledger and actual check results; no known critical usability/accessibility blocker in delivered scope; limitations explicitly recorded. |

Accessibility, responsive behavior and speed are gates in every phase, not work postponed until Phase 6. Ship/review coherent page slices rather than changing every template simultaneously. Conditional features can be dropped without blocking core shopping.

Start with a **customer-first release**: baseline, closed-filter/focus fixes, compact browse toolbar, shared mobile controls, search, homepage slideshow and product-card consistency. Then product detail/cart/checkout. Portal polish follows once the shopping journey is coherent. Calendar estimates should follow Phase 0 and the approved slice scope; this review is not enough to promise a reliable completion date.

## Features deliberately outside this UI-only release

New recommendation/ranking engines; live courier ETA/PIN service; restock notifications; guest checkout if not already permitted; wishlists synchronized differently from the existing model; new gift wrapping/message/order fields; rewards/referrals; abandoned-cart messaging; new payment methods; CMS/marketing scheduling; changed role/security rules; translation persistence; inventory logic; uploads requiring a new transformation service; backend autosave; changed cache/CDN headers or infrastructure.

Do not add discount popups on arrival, false urgency, fabricated scarcity/reviews, a permanent promotional ticker, forced scroll animations, background autoplay video, multiple competing carousels, heavy parallax, cursor effects, endless product scrolling or a bottom toolbar on every page. They increase attention cost without helping the current core journey. Optional quick view and device-local recents need a usability benefit before inclusion.

## Five usability tasks and proposed success measures

1. Find a piece for the home within a budget, apply/remove a filter, inspect it and return to the same results.
2. Choose the correct option of a textile, inspect photos and relevant supplied details, then add it to Cart.
3. Save a piece, retrieve it from Wishlist, update Cart and understand the authoritative total.
4. Complete the existing address/delivery/payment/review flow on a phone; recover from an invalid field or payment failure without creating another order.
5. Find an existing order and understand its payment/fulfilment state, then reach the existing eligible help/return action.

During implementation, record completion, errors, backtracking and task time. Suggested initial target: at least 90% task completion in a small consented usability sample and no severe keyboard/screen-reader blocker in covered tasks. Small samples are directional, not a conversion forecast. Do not claim increased sales or performance without measurement. Recruiting people, contacting customers or adding tracking remains a separate user-authorized activity.

## Presentation ownership and verification commands

| Surface | Existing presentation owners |
| --- | --- |
| Shared header/footer/cards/cart feedback | `web/components/components.templ` |
| Home/listing/detail/cart/checkout | `web/pages/pages.templ`, associated `web/pages/*.templ` and current customer-workflow presentation |
| Customer tokens/layout | `web/assets/css/theme.css`, `customer-polish.css`, `utility.css`, `fonts.css` |
| Shared semantics/forms/feedback | `web/assets/css/ui-standards.css`, `web/assets/js/ui-standards.js` |
| Storefront interactions | `web/assets/js/storefront.js`, existing inline presentation initializers; preserve `backend.js` request contracts |
| Seller | `web/selleradmin/*.templ`, `web/assets/css/seller-admin.css`, existing product-media presentation |
| Company workspaces | `web/admin/*.templ`, current shared support presentation, role-owned CSS |
| Regression evidence | Existing `tests/e2e`, header checks, generated-template checks, QA screenshots and this ledger |

Run the repository's templ generation and `npm run css:build` when their sources change. Use `npm run test:storefront-header`, the focused customer/role/checkout Playwright suites and relevant Go template-rendering tests according to each change. Inspect suites before execution: fixture-creating tests run only against an isolated QA environment. Do not run resets or seeds against active business/local user data to obtain prettier screenshots. Use Chromium, Firefox and WebKit for affected core paths, with real iOS/Android checks before declaring their support verified.

The skill selection shaped this plan: frontend-app-builder supplies coordinated concepts and component fidelity before larger changes; better-accessibility supplies keyboard, semantics, form, zoom and motion acceptance; emil-design-eng supplies short, purposeful and interruptible interaction feedback. Existing project contracts and this UI-only scope take precedence over generic framework or visual defaults.
