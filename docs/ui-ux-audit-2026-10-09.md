# WeeVCrafts UI/UX review — 9 October 2026

## Implementation integrity verdict

**Pass for a coherent product identity; operational consistency needs work.**
The storefront and role portals consistently express an Indian craft marketplace
through their typography, imagery and palette. There are verified inconsistencies
in role navigation, missing-media handling, information shown to users and keyboard
access. Keep the current identity and address these issues before a broad redesign.

This is a review and recommendation report. Application UI files were not changed.

## Executive summary

**Audit health score: 12/20 — acceptable, with significant work needed.**
Scores are engineering triage judgments, not a WCAG certification or release approval.

| Dimension | Score / 4 | Key finding |
| --- | --- | --- |
| Accessibility | 2 | Low contrast; Safari/WebKit cannot keyboard-focus a scrolling seller table. |
| Performance | 3 | Lean server-rendered app, but the complete icon font is almost 1 MB. |
| Responsive design | 2 | No whole-page overflow in the survey; compact controls and overlapping seller intro artwork remain. |
| Theming | 3 | Existing CSS tokens and a coherent light theme; portal token families can be consolidated. |
| Implementation integrity | 2 | A shared CTA crosses role permissions; fixed location/order text misrepresents context. |
| **Total** | **12/20** | **Improve shared primitives and navigation first.** |

**14 grouped findings: 0 P0, 4 P1, 10 P2, 0 P3.** Repeated occurrences of a shared
component are counted once. The four P1 findings are contrast, keyboard access to
tables, missing media, and links to forbidden workspaces.

## Scope and evidence

Reviewed the running local Docker application at `http://127.0.0.1:8080`, the active
templates/styles/scripts, and the navigation and forms across guest plus all eight
roles. The browser survey contains **144 role/route combinations, 288 desktop/mobile
observations**, at widths **1440 and 390 pixels**. These combinations include shared
pages, selected record details, product editing, search/no-result examples and linked
tabs. They are not 144 distinct templates.

**188 observations received axe-core 4.14.0 scans**: all desktop routes and selected
mobile screens. Other mobile routes received DOM/layout surveys. Chromium version:
153.0.8010.12. The targeted seller-table keyboard check was additionally run in
Firefox 155.0 and WebKit 26.6. Screenshot inspection and source review supplied the
visual/content findings. The bundled design detector was run once over the UI source.

| Context | Role/route combinations |
| --- | ---: |
| Guest | 28 |
| Customer | 19 |
| Seller owner | 15 |
| Seller staff | 12 |
| Marketplace admin | 20 |
| Super Admin | 32 |
| Support agent | 4 |
| Finance operator | 8 |
| Returns operations | 6 |

Evidence:

- [Route inventory](qa/UI-20261009/route-inventory.csv).
- [Browser measurements and accessibility findings](qa/UI-20261009/browser-audit.json).
- [Targeted confirmations across browsers](qa/UI-20261009/confirmed-findings.json).
- [Detector output](qa/UI-20261009/detector.json).
- [Screenshots](qa/UI-20261009/screenshots/).

The read-only review did not submit business forms, create orders, edit products,
change permissions or delete records. It does not establish every save/error state,
provider integration, screen-reader experience, physical-device behavior or browser
compatibility across every page. Performance scores are based on implementation and
asset evidence; no production-network Core Web Vitals claim is made.

## P1 — address before release

### UI-01: Some text has insufficient contrast

**Category:** Accessibility. **Location:** `web/assets/css/seller-admin.css:118`,
`web/assets/css/theme.css:99` and `web/assets/css/theme.css:198`.

Examples measured by axe include white seller-profile initials on `#eadfce`
(**1.31:1**), category counts (**2.74:1**), and mobile hero-note text (**1.41:1**).
Contrast findings occur in 35 successful page/viewport observations, principally
shared storefront and seller components. Faint identity/status text is difficult
for low-vision users to read.

**Standard:** [WCAG 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)
requires 4.5:1 for ordinary text and 3:1 for qualifying large text, with specified exceptions.
**Recommendation:** Use darker foregrounds or stronger surfaces for category counts
and initials. Put image-overlay text on a predictable background and verify the actual
rendered combinations. **Suggested command:** `/impeccable colorize`.

### UI-02: Scrolling data tables need explicit keyboard access and table semantics

**Category:** Accessibility. **Location:** `web/selleradmin/portal_views.templ:26`,
`:42`, `:325`; `web/supportportal/live_views.templ:72`; shared admin table templates.

At 390 pixels, the seller dashboard product region is 348 pixels wide with 620 pixels
of content. It has no explicit tab stop or accessible region name. The targeted Tab
test reaches it in Chromium and Firefox, which provide native scroll-container
focus behavior, but **does not reach it in WebKit**. Axe flags similar regions in
16 observations. Do not describe all of these as confirmed failures in every browser.

Several tables use nested `div` elements and header `span`s, without table/header
relationships. A support queue has a labelled focusable region, which is a useful
partial precedent, but it also lacks native table semantics.

**Impact:** Safari keyboard users can miss off-screen columns; assistive technology
has less reliable row/header context. **Standards:** [WCAG 2.1.1](https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html)
and information/relationship semantics.
**Recommendation:** Use semantic `table`, `caption`, `th scope` and `td` for tabular
data. Give each overflowing container `tabindex="0"`, an accessible name and visible
focus styling, and verify arrow-key scrolling. Keep mobile cards where they improve
reading without losing information. **Suggested command:** `/impeccable harden`.

### UI-03: Missing media renders broken/blank product and maker imagery

**Category:** Implementation integrity / Responsive. **Location:**
`web/pages/pages.templ:270` (seller cards) and category/product-image rendering in
the same template file.

Sixteen observations contain broken images. Category media such as
`/media/products/120/preview` and `/media/products/18` were independently confirmed
to return **404**. Some maker cards render an empty `src`, which resolves to the
current HTML page instead of an image. Local fixture/media data contributes to the
missing files; the UI still needs a resilient empty-image treatment.

**Impact:** Customers cannot judge products reliably, and blank cards make the
catalogue look incomplete. **Recommendation:** Render a deliberate no-image state
when the URL is empty and a consistent fallback when an image request fails.
Investigate stale media records/storage separately. Do not silently substitute an
unrelated product photograph. **Suggested command:** `/impeccable harden`.

### UI-04: Finance and Operations share a CTA that leads to a forbidden page

**Category:** Implementation integrity / Navigation. **Location:**
`web/admin/admin.templ:114`, checked against `internal/web/handlers/portal_routes.go`.

The shared sidebar's **“View marketplace”** link points to `p.PortalRoot + "/analytics"`.
It appears throughout Finance and Returns Operations, but `/finance-portal/analytics`
and `/operations-portal/analytics` return **403** for those roles. The route survey
records the actual source pages linking to these destinations.

**Impact:** An apparently valid primary navigation action ends on an access-denied
page. **Recommendation:** Show an allowed, role-specific destination or omit this CTA
for these roles. Preserve server-side permission enforcement.
**Suggested command:** `/impeccable clarify`.

## P2 — improvements for the next pass

### UI-05: Seller mobile intro artwork competes with functional text

**Category:** Responsive / Layout. **Location:** `web/assets/css/seller-admin.css:143`,
`:147`, `:320`; `web/selleradmin/seller_admin.templ:118`.

The 390-pixel product-editor screenshot shows the craft image occupying the same
area as the introductory copy, with the decorative note crowded at the right.
The header tagline also competes with the profile avatar.
**Impact:** Important form instructions are harder to scan before entering product
information. **Recommendation:** Stack or suppress decorative artwork on phones,
give operational copy its own column, and hide/shorten the header tagline at narrow
widths. Keep the page title and primary action visible.
**Suggested command:** `/impeccable adapt`.

### UI-06: Seller owners cannot discover Team and Returns through the main navigation

**Category:** Implementation integrity / Navigation. **Location:**
`web/selleradmin/seller_admin.templ:72` (SellerSidebar).

The owner has working Team and Returns routes, but the dashboard has **zero links**
to `/seller-admin/team` or `/seller-admin/returns`. The sidebar omits both.
**Impact:** Users must know the direct URL to find important work.
**Recommendation:** Add permission-aware Team and Returns entries with active states;
show Team only for owners and Returns according to the assigned permissions.
**Suggested command:** `/impeccable clarify`.

### UI-07: Product pricing requires sellers to perform rupee-to-paise conversion

**Category:** Implementation integrity / Form usability. **Location:**
`web/selleradmin/portal_views.templ:126`, `:127`, `:179`, `:199`.

Visible fields ask for “Price (paise)”, compare-at price in paise and unit cost in
paise. The label is explicit, but it makes everyday pricing harder and increases the
chance of a 100-fold entry mistake.
**Recommendation:** Present `₹`/INR decimal inputs, show the formatted selling price,
and convert to integer minor units at the boundary. Retain server-side validation.
Also align upload instructions with the supported WebP format and 25 MiB limit.
**Suggested command:** `/impeccable clarify`.

### UI-08: Delivery location is fixed to Bengaluru

**Category:** Implementation integrity / Content. **Location:**
`web/components/components.templ:11`.

The shared header always says “Deliver to Bengaluru, Karnataka”, including guest
visits. The component does not derive this text from a chosen address.
**Impact:** Customers can mistake it for their selected delivery destination or an
availability promise. **Recommendation:** Show “Choose delivery location” until a
selection exists, then display the selected city/PIN code consistently with checkout.
**Suggested command:** `/impeccable clarify`.

### UI-09: Support order-list breadcrumb names a fixed unrelated order

**Category:** Implementation integrity / Content. **Location:**
`web/supportportal/support_portal.templ:87`.

The live page heading is “Orders linked to cases”, but its breadcrumb ends with
**“Order #WC2504267831”**, even though the page is a list rather than that order's detail.
This was confirmed in the running UI.
**Impact:** Agents can mistake which record they are viewing.
**Recommendation:** Use a list breadcrumb on the queue; show a number only on a
detail view populated from the selected order. **Suggested command:** `/impeccable clarify`.

### UI-10: Compact mobile controls are difficult to tap

**Category:** Responsive / Accessibility. **Location:** `web/assets/css/seller-admin.css:320`,
`web/assets/css/utility.css:12`, and shared header action styles.

Examples include the **28×28-pixel seller menu button** and **31×31-pixel password
toggle**. Dense links and several compact portal controls are also below 44 pixels.
**Impact:** More missed taps on phones, especially for users with limited dexterity.
**Recommendation:** Aim for 44×44-pixel hit areas for standalone controls, keeping
icons visually smaller if needed. Preserve spacing around adjacent actions.
The 44-pixel recommendation is a comfort target, **not a blanket WCAG AA failure**:
[WCAG 2.5.8](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)
uses a 24-pixel minimum with spacing and other exceptions.
**Suggested command:** `/impeccable adapt`.

### UI-11: Shared footer content sits outside landmarks and skips heading levels

**Category:** Accessibility. **Location:** `web/components/components.templ:102`, `:103`.

The trust strip precedes the actual `footer`; axe reports content outside landmarks
in 74 successful observations. Auth pages have an H1 followed by footer H3 headings,
producing heading-order advisories in 10 observations.
**Impact:** Screen-reader landmark and heading navigation is less predictable.
**Recommendation:** Include the trust strip within a named landmark and make footer
section headings consistent with the document outline. These are best-practice
findings; the heading-order advisory alone is not proof of a WCAG failure.
**Suggested command:** `/impeccable harden`.

### UI-12: The full icon font adds avoidable first-load weight

**Category:** Performance. **Location:** `web/assets/css/fonts.css:63` and
`web/assets/fonts/material-symbols-outlined.ttf`.

The raw icon font is **995,576 bytes**. Self-hosting and `font-display: swap` are good,
but shipping the whole set is costly for a comparatively small icon vocabulary.
This is the raw asset size, not a measured compressed network transfer or LCP value.
**Recommendation:** Subset the glyphs actually used or use a small SVG icon set;
prefer WOFF2 for web fonts and verify no icons disappear.
**Suggested command:** `/impeccable optimize`.

### UI-13: Unknown category and brand URLs do not communicate “not found” correctly

**Category:** Implementation integrity / Error states. **Location:**
`internal/web/handlers/live_storefront.go:236`, `:600`, plus category fallback routing.

Fresh requests to `/category/ui-audit-missing-20261009` and
`/brands/ui-audit-missing-20261009` return **200**. This reconfirms the existing
QA-GST-001 finding; it is not a newly introduced defect.
**Impact:** Bad links and nonexistent collections can appear as successful pages.
**Recommendation:** Distinguish an existing empty collection from an unknown slug.
Return 404 with a useful recovery page for the latter, including browse/search links.
**Suggested command:** `/impeccable harden`.

### UI-14: Footer policy names are plain text rather than usable links

**Category:** Implementation integrity / Navigation. **Location:**
`web/components/components.templ:103`.

“Terms & Conditions | Privacy Policy | Cookie Policy” is rendered in a `span`.
The app has Terms and Privacy pages, but these footer labels do not link to them.
**Impact:** Customers cannot open policies from the conventional footer location.
**Recommendation:** Link Terms and Privacy to their real pages. Add a Cookie Policy
link only when a supported page exists. Update the fixed 2024 footer year as part of
this small component cleanup. **Suggested command:** `/impeccable clarify`.

## Patterns and positive findings

- Prioritize shared primitives: one fix to seller avatar contrast, footer structure,
  scroll-container access or role-aware navigation will improve many pages.
- Preserve the craft identity. Tokens already exist; consolidate semantic button,
  surface, focus, status and typography rules across portal families gradually.
  Absence of a dark theme is not reported as a defect because it is not an established
  product requirement.
- No whole-document horizontal overflow or JavaScript page errors was recorded
  in the 288 observations. Internal table scrolling still needs the work above.
- Pages use real labels, explicit empty states, skip links and visible focus styling.
  The responsive navigation opens and moves focus into the menu. Source includes
  Escape/focus-return handling and reduced-motion rules; full screen-reader/menu
  testing remains outside this survey.
- Role-specific pages and server enforcement are present. Correct the forbidden
  CTA instead of expanding access to make the link work.
- UI assets and fonts are self-hosted; CSS/JavaScript are checked in and served by
  the Go application, without a runtime frontend build service.

## Findings intentionally not promoted to defects

- The design detector produced six border-accent warnings: three authored rules
  plus their minified copies. They correspond to low-stock alerts, agent replies
  and informational callouts. These accents have a useful semantic purpose;
  they are **not evidence that the UI needs a redesign**.
- The simple DOM detector marked upload/print buttons as potentially inert.
  `web/assets/js/product-media.js` attaches handlers through `data-media-*` and
  `data-print-kind`, so these are not established dead controls. Their complete
  upload/print workflows were not executed in this read-only audit.
- `/support` returned 403 for the customer but has no incoming customer link in
  the survey; it is a restricted legacy portal entry, not a customer navigation bug.
- `/operations-portal/disputes` returned 404 and had no incoming UI link. Treat
  disputes as a requirements/coverage gap if that workflow is required, rather
  than claiming a broken visible link.
- The initial customer menu focus-return capture ran before the framework's next
  tick. A null captured label alone does not establish a focus-return defect.

## Suggested implementation order

1. **P1 `/impeccable harden`:** Media fallbacks and accessible tables; verify Safari.
2. **P1 `/impeccable colorize`:** Contrast corrections in shared storefront/seller styles.
3. **P1/P2 `/impeccable clarify`:** Role CTA destinations, seller navigation, pricing,
   delivery location, support breadcrumb and real policy links.
4. **P2 `/impeccable adapt`:** Seller mobile intro/header and touch hit areas.
5. **P2 `/impeccable optimize`:** Icon/font payload after functional corrections.
6. **`/impeccable audit`:** Recheck the changed surfaces and compare evidence.
7. **`/impeccable polish`:** Final consistency pass after behavior/accessibility work.

You can ask me to run these one at a time, all at once, or in any order you prefer.
Re-run `/impeccable audit` after fixes to see your score improve.
