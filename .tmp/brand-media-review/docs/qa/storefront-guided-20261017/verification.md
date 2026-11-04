# Guided storefront and selected-item checkout verification

Date: 17 October 2026

## Implementation

The storefront overview separates published releases, editable drafts and submitted
reviews. Existing editor routes and draft links remain valid. Grouped tools,
a page selector, named section outline, contextual inspector, dismissible guidance,
responsive views and private click-to-edit controls explain what is being changed.
Only the selected section exposes outline mutation controls. Hidden sections stay
selectable. Product and commerce content identifies its protected ownership.

Preview rendering uses the customer renderer with a proposed configuration and
never saves it. Updates debounce 300 ms, abort obsolete requests and ignore stale
responses. Invalid proposals retain the last valid preview. The editor shows
unsaved/saved/submitted/published states and the current customer release. Saving
is explicit. Shared confirmation supports Save and continue, Discard and Keep
editing; catalog searches and structural actions also protect unfinished edits.
Frame messages validate origin, source window and allowed targets.

Image selection includes a bounded searchable library, private uploads, retained
file selection on failure, descriptions and focal controls for the new asset.
Structured repeaters retain separators as ordinary text, collapse by readable
names and offer real destinations. Existing text forms remain available without
JavaScript. Draft presets support curated products, gifts, collection tiles,
verified maker stories, banners and FAQs. Comparisons use readable section names,
financial formatting and image thumbnails. Readiness findings link to corrections.

Add to cart navigates to the cart page after confirmation. Buy now uses a separate
customer-owned checkout cart for the chosen variant and quantity. Its 24-hour
context does not reserve stock. Normal shopping-cart contents, quantities and
counters remain separate. Checkout quotes, coupons, stock locking, policy
revisions, reservations, payment verification and order retries use the existing
services. Completed contexts resolve to their saved owned order. Migration 50
preserves existing carts as shopping carts and scopes normal-cart uniqueness,
lookups and merging accordingly. Native generated SQL models were regenerated.

Product pages expose existing specifications and published shipping/return
information near purchase controls. Branding, product photography, shared cards,
existing filters and homepage crossfade remain in the accepted design system.
No new inventory or claims were copied from the reference websites.

## Environment and data boundaries

Business verification uses the native Go application on loopback port 18085,
PostgreSQL database `storefront_guided_20261017` cloned from the local QA fixture,
and Redis database 6 on the existing local QA service. Test accounts and the
`guided-checkout-piece` product are synthetic. Payment completion uses only the
explicit deterministic local provider. Email delivery is disabled.

QA uploads use unique IDs in the existing local Garage service; their metadata
and publication access are scoped to the QA database. No internet storage,
production data, live payment, external email or carrier action is exercised.
The normal Docker preview is rebuilt separately, retaining its own database and
accounts. No seed/reset command is part of that startup. The rebuilt application is
healthy at `http://127.0.0.1:8080`; the Super Admin login and guided editor
respond successfully. Migration 50 is applied to this local preview database.

The codebase-memory transport returned `Transport closed`, so relevant source
and maintained contracts were inspected directly. Native browser automation was
unavailable in Work settings; browser verification uses real headless Playwright
Chromium, Firefox and WebKit.

## Verification results

- Full `go test ./...` passed with isolated PostgreSQL integration and projection databases.
- The final editor/handler/page changes passed targeted Go regression tests.
- PostgreSQL tests cover normal-cart preservation (including the same variant),
  selected quantities, no reservation on context creation, quote/placement parity,
  exactly-once order retry, foreign ownership, 24-hour expiry, independent contexts,
  concurrent duplicate requests and insufficient stock.
- Guided Playwright checks cover unsaved live rendering, invalid preview retention,
  save-and-continue, private saved previews, missing CSRF, stale versions, structured
  text, private media, presets, purchase destinations, duplicate clicks, interrupted
  checkout responses, preserved carts, completed-context recovery and account isolation.
- The final guided run passed ten browser scenarios (six Chromium, two Firefox,
  two WebKit). Ten duplicate business-mutation cases were intentionally skipped.
  The added catalog-search navigation scenario passed separately in Chromium
  after correcting its test-only native-select locator.
- Responsive checks exercise 320, 390, 768, 1024 and 1440 pixels in Chromium,
  representative 390/1440 layouts in Firefox and WebKit, 844×390 landscape and
  720-pixel reflow equivalent to 200% on a 1440-pixel surface. The iframe owns its
  intentional horizontal scrolling; document overflow is checked independently.
- Keyboard checks cover editor tabs, native option selection, dialog Escape and
  focus restoration. No-JavaScript editor saves and role denial are exercised.
- Axe checks target the changed editor and product content with WCAG 2 A/AA rules.
- The maintained publication suite passed four scenarios: Admin review submission,
  Super Admin change requests/publication, restoration, stale edits, private media,
  responsive forms, templates/content and shopping-feature publication.
- The maintained homepage suite passed autoplay, swipe/focus/hover/reduced motion,
  image failure and responsive catalog checks. The five-slide 450 ms crossfade
  remains governed by the existing controller.
- templ v0.3.1020 generation, sqlc v1.30.0 generation, CSS/JavaScript asset builds,
  JavaScript syntax checks and `git diff --check` pass.
- The strict premium audit reports zero findings; see `premium-audit.json`.
  DESIGN.md lint reports zero errors and its existing nonblocking unused-accent
  warning. Existing runtime tokens and shared owners are retained.

## Corrections and retests

Browser inspection found and corrected missing icon glyph names, an overly long
outline, hidden-control display overrides and verbose preview edit labels.
Structured initial data now comes from typed configuration, so a literal pipe in
a label survives reload. Preview messages use the document's effective origin
for srcdoc rendering. The shared Admin shell now restores navigation focus only
when its sidebar is actually open; closing an unrelated dialog restores its own
trigger. GET catalog searches now use the same unsaved-navigation protection.

A publication regression run reached its assertions but collided with another
runner's shared trace folder during teardown. Its four scenarios were rerun
serially with separate output and passed. A temporary templ file-lock failure
while Go read generated files was resolved by rerunning generation after the
reader completed. No generated-source drift remains on untouched surfaces.

## Evidence and limits

Screenshots: [desktop editor](editor-1440.png), [contextual preview](editor-context-1440.png),
[mobile editor](editor-390.png),
[structured navigation](editor-navigation-1440.png), and
[mobile product](product-390.png). These are implementation
screenshots using isolated fixtures, not approved catalog inventory.

Physical devices, browser-native zoom, screen readers, real Razorpay outcomes,
carrier integration and internet email delivery remain outside this local
verification. No accessibility certification or production deployment is claimed.
