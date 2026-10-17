---
version: alpha
name: WeevCrafts
description: An Indian craft marketplace with an editorial storefront and calm operational workspaces.
colors:
  primary: "#123f32"
  background: "#fffdf8"
  surface: "#fffdf8"
  ink: "#20362d"
  muted: "#56665d"
  accent: "#9d4031"
  border: "#dfd8ca"
typography:
  heading:
    fontFamily: "Playfair Display, Georgia, serif"
  body:
    fontFamily: "DM Sans, Arial, sans-serif"
rounded:
  control: "6px"
  panel: "12px"
spacing:
  gutter: "24px"
  control: "44px"
components:
  button:
    height: "44px"
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
  input:
    typography: "{typography.body}"
    textColor: "{colors.ink}"
  field-hint:
    textColor: "{colors.muted}"
  divider:
    backgroundColor: "{colors.border}"
---

# WeevCrafts visual standards

## Customer workflow refinement — 17 October 2026

Customer task pages retain the approved shared brand, typography, cards and
responsive account navigation. Order history adds a readable result range and
wrapping Previous/Next controls with 44px targets. Order and return filters are
navigation links with the existing active treatment. Customer form failures and
confirmations reuse ui-standards presentation and behavior; no parallel visual
system or customer-specific dialog implementation is introduced.

## Seller workflow refinement — 17 October 2026

The seller portal retains its established palette, operational typefaces, sidebar,
shared validation and button family. Product forms use a four-step navigation
with a clear current step, compact optional specifications and an editable review.
Inventory adjustments expand within the relevant record. Orders use one paginated
table with explicit payment and fulfilment labels. Controls retain 44px targets;
phone layouts preserve every value and action through existing record-table rules.

Homepage slide changes use a 450ms ease-out opacity crossfade inside a shared grid
cell. Both images reserve identical hero geometry. No layout property is animated.
Reduced motion uses an immediate change. Runtime owners remain `experience.js`
and `experience.css`; seller-specific presentation remains in `seller-admin.css`.

## 15 October 2026 Woven Gallery sign-in

The approved C concept replaces the `/login` card with an open form and decorative pottery, textile and brass still life. Keep the woven mark larger than the restrained wordmark. Mobile uses a shallow photography strip and natural page scrolling. Login and the protected-action prompt share credential fields, 50px controls, Playfair headings, DM Sans interface text, 8px control corners and the existing forest/ivory tokens. `auth-gallery.css` owns this bounded surface after shared brand styles.

The user's approved amendment adds Continue with Google and puts username/password fields directly inside the mobile prompt. The phone prompt is a centred native dialog over a translucent, blurred backdrop, with an opaque readable surface and bounded vertical scrolling. Desktop retains a right-side prompt. Keep close/Escape/backdrop dismissal, focus trapping/restoration, password clearing, explicit Google consent, provider fallback and existing-account proof. Public browsing never opens it automatically. Decorative generated artwork is not catalogue inventory or a maker identity.

## Overview

Preserve the accepted Indian craft exhibition identity: ivory paper, forest green, terracotta accents, restrained serif headings and genuine object photography. The memorable element is the relationship between a maker and the object. Never manufacture maker biographies, product specifications or portraits to fill a layout. Storefront expression gives way to task clarity in seller, admin, finance, operations and support workspaces.

## Colors

Ownership model B: `web/assets/css/theme.css` owns storefront tokens. Existing `admin.css`, `seller-admin.css` and `support-portal.css` own their role-specific palettes; these are intentional variants, not a storefront rebrand. Shared semantic treatments in `ui-standards.css` use their existing variables. Success means completed or available; danger means failed or destructive; warning means pending or requiring attention. Every status also has a readable text label. Do not use color as the only signal.

## Typography

Playfair Display headings; DM Sans body and interface text. Controls use 16px, data tables at least 14px and a 1.5 line height. Prices, quantities and identifiers use tabular numerals. Headings wrap naturally; avoid forced line breaks that isolate articles. Long instructions and validation errors wrap.

## Layout

Discovery content uses a 1320px maximum width with full-width photographic banners. Forms and task pages retain their established narrower layouts. The homepage order is hero, shallow category rail, available product selection, makers, curated collections. Avoid repeated mission statements before products. Mobile header keeps 44px controls, a compact brand/action row and prominent search. Details retain the mobile purchase bar without covering focus. Operational dashboards show permitted actions, summaries and bounded previews; full queues own dataset navigation. Document scrolling remains natural; only genuine tables may scroll horizontally.

The desktop storefront header has a brand, delivery, search and account/actions row above category navigation. The supplied logo accompanies the wordmark. Narrow screens use a compact menu/brand row and prominent search, with delivery available through its established control. Wishlist and cart are direct, counted shopping actions alongside the profile control. Account, notifications and sign out remain in the profile disclosure; a direct logout button remains available. Its trigger displays the signed-in customer's existing private photo when available. On narrow account pages, a collapsible Account menu precedes the current section without delaying it behind the full navigation.

## Elevation & Depth

Keep existing light panel shadows; no new decorative gradients, floating ornaments or animated data rows. Hero photographs have no added tint. Media always reserves its aspect ratio and uses an honest missing-image fallback.

## Shapes

Controls use the existing 6px radius. Panels use restrained rounding. Hearts retain the existing circular shape. No giant rounded page wrappers.

## Components

| Role | Runtime owner | Consumers |
| --- | --- | --- |
| Paper/ink/brand/type | `theme.css` root variables | shared header, footer, product cards, pages |
| Storefront refinements | `customer-polish.css` | compact header, hero, facets, account |
| Shopping experience / responsive media | `experience.css`, `experience.js`, shared templ components | automatic hero, filters, gallery, mobile purchase controls |
| Semantic status and operational tables | `ui-standards.css`, `components/presentation.go` | all role portals and order history |
| Native fields and validation | `ui-standards.js` | seller, support, admin and checkout |
| Canonical media | product database and upload workflow | cards, galleries, seller editor |

Pointer press feedback uses 120ms. Pointer-driven checkout transitions use 160ms opacity/transform feedback; keyboard and reduced-motion transitions are immediate. Pending actions use stable-size busy indicators. Drawer semantics and focus restoration follow existing shared implementations. Keyboard focus is visible on links, buttons, fields and overflow regions. Global scrollbars stay operable; forced-colors uses system colors.

## Do's and Don'ts

Keep the real logo, catalogue values, seller permissions and lifecycle rules. Use verified content and customer-facing option names. Do not replace Go/templ with another framework or turn generated concept product names/photos into real catalogue data. Generated references guide composition; maintained data and existing accepted assets override their illustrative content.

Header update: the storefront uses one sticky toolbar at every scroll position. Menu, wordmark, delivery, search and account actions share one row. At narrow widths delivery and Wishlist move to the drawer; search expands across the toolbar with Escape restoring focus. Account sidebars use compact identity panels and 44px navigation rows.

## 14 October 2026 implementation contract

The foundation, focus, feedback, form and reflow rules apply to all four current roles and their detail screens. Shared badges, tables, dialogs and labels inherit the document's interface family: DM Sans in the storefront, Inter in operational portals. Storefront headings remain Playfair Display; operational headings remain Roboto. Font preloads and CSS use the same WOFF2 URLs. Original TTFs and licences remain maintained sources. The Material Symbols subset preserves the required ligature feature and referenced glyphs.

Controls use 44px comfort targets, 16px entry text, visible 2px focus outlines, 6px corners and a meaningful control border (#8e7d6b); decorative dividers keep the lighter border. The homepage has five stories and advances every 4 seconds, with visible Pause/Play and manual navigation. Hover, focus, manual selection and a hidden browser tab pause playback; reduced-motion users start paused. Inactive links remain inert. Later images/video are deferred. Product media reserves a square frame and uses contain rather than cropping the object. Product photography, real choices and current prices override generated composition references.

Listing updates preserve the open sheet, facet searches, scroll, expanded groups and active control through HTMX settlement. Hidden mobile sheets are inert immediately, including before enhancement. Drawer/sheet motion is 180ms; keyboard and reduced-motion access is immediate. One contextual purchase region is visible; it yields to inline controls, policy links and text-entry focus. No bottom navigation or promotional layer is added.

Operational data uses native tables with scoped column/row headers and named, focusable overflow regions. Narrow summaries retain every value/action and explicit cell labels. Private conversations, internal notes and seller confirmations keep their established boundaries. Busy controls retain their text and geometry; success styling waits for the actual response. Archive/restoration confirmations use consequence-specific intent, cancel-first focus and Escape. An unsaved editor stays in memory and offers Keep editing before navigation.

See the dated implementation ledger for actual route/state coverage and limitations. Physical devices, browser-native zoom and screen readers remain release checks; a browser score is not certification.

Account access is an optional, focused task within the accepted storefront identity. A compact single-column card retains existing paper/forest/terracotta tokens and type families, readable labels, 48px fields and 44px password controls. Public browsing is a visible exit. Account screens replace the discovery/trust footer with small Help/Privacy/Terms links; they do not add hero photography, decorative animation or disabled provider controls above the primary form. This presentation is shared by sign-in, sign-up and account recovery without changing provider or server security policy.

## Approved customer references — 15 October 2026

The two customer-supplied reference images supersede earlier storefront header, footer, discovery density, and card geometry descriptions above. This is an approved customer-only visual evolution; operational portals keep their palettes and typography. `theme.css` owns the palette through `.storefront-body` variables; `storefront-reference.css` owns the reference composition and responsive variants, loaded after existing experience styles. Shared existing interaction owners remain authoritative.

The storefront uses near-white paper (#fcfbf8), warm white surfaces (#fffefa), dark brown headings (#66250f), terracotta actions (#ad3619), light warm dividers (#e9e2d7), and green availability (#245b34). Playfair Display remains the serif display/card-name face; DM Sans remains interface/body type. The wordmark reads WeevCrafts. Discovery containers cap at 1320px with 40px desktop and 16px phone gutters. The desktop shell has a brand/delivery/search/actions row and a category-navigation row. On phones the brand/menu row precedes visible search and shopping actions. Notifications, private avatars, and sign out remain available.

Homepage order: panoramic five-slide craft hero, seven circular category links, six compact live product cards, three horizontal maker cards, five collection links, pale trust band, light compact footer. Listings use a shallow lifestyle banner, existing functional filters, editorial collection links, native sort, removable URL-backed chips, and three wide-desktop product columns. Mobile discovery rails scroll visibly, cards use two columns or one at 360px and below, and filtering retains its accessible sheet. Product images contain the real object without cropping; wide card frames therefore intentionally differ from the sample's staged photographs. Real products and truthful metadata take precedence over sample names, ratings, badges, and counts. Missing maker portraits use text, never a generated identity.

All customer task surfaces adopt these materials and controls through the shared shell and scoped styles while retaining their current layout and workflow. Auth keeps its compact help/policy footer. Quick view is a native modal with real product/variant data and existing cart behavior. Editorial banner artwork is illustrative; it is never used as a maker portrait or product record. The footer links to existing notification preferences rather than introducing newsletter delivery.

The supplied teal-and-gold logo is used in storefront and operational branding and favicons; its cream background and proportions are retained. This explicitly requested brand update does not restyle operational layouts.


## Approved brand theme — 15 October 2026

The approved `docs/design/brand-theme-concept-2026-10-15.png` supersedes the earlier terracotta-primary palette and automatic carousel descriptions. This change applies shared branding to all application roles while retaining their layouts and typography.

`brand.css` owns common identity tokens: primary #123f32, hover #0d3529, ivory #f8f2e8, warm white #fffdf8, terracotta #9d4031, gold #906220, and ink #20362d. Storefront paper is warm white; ivory appears in search and supporting surfaces. Role aliases map navigation, surfaces, focus and primary actions to these tokens. Success, warning, failed-payment and destructive-action styles remain distinct.

Use the supplied woven artwork with live WeevCrafts wordmark text. Following the user's final size correction, the desktop logo mark remains 52–70px while the wordmark scales 32–44px. Phone branding keeps its 32–38px mark and 22–30px wordmark. Storefront typography stays Playfair Display and DM Sans; operational typography stays Roboto and Inter. No hero photograph tint is added.

Shared navigation omits the delivery shortcut and Support Artisan Livelihoods link. Account address management and checkout delivery remain available. The five-slide hero is manual: arrows, slide selectors, inert inactive slides, announcements and deferred photographs remain; playback buttons and timers are removed.

Guests use a direct Sign in link in the header, with no account chevron or profile dropdown. Authenticated customers retain the existing profile disclosure, private avatar, account links, notifications and sign out.

The homepage document and social title is WeevCrafts | Handmade Indian Arts, Crafts & Sarees. Existing WebSite/Organization schema uses the same name, canonical URL and supplied logo, with historic spelling aliases. Private routes retain their indexing restrictions. Google recrawl requests follow production release, not a local preview.


### Approved mobile homepage — 15 October 2026

The user approved `docs/design/brand-theme-mobile-concept-2026-10-15.png` after requesting a distinct mobile composition. DesktopHomeIntro and MobileHomeIntro share the `/` URL; CSS selects the introduction at 800px. The product, maker and collection sections render once from the same catalogue. No device redirect, device URL, user-agent gate or additional backend endpoint is introduced.

The mobile shared header places menu/brand/cart/account on the first row and full-width search on the second. Wishlist stays in the navigation drawer; sign out stays in profile and drawer controls. The mobile homepage uses a compact photographic hero with the approved headline, Shop collections and Meet the makers; desktop eyebrow, paragraph, quote and carousel controls are omitted. Its category rail uses 64px photographs, and featured products remain two columns even at 320px. The desktop five-slide hero retains manual arrows/selectors. Product names, prices, images and empty states remain real catalogue data.

The mobile editorial banner is a text-free recomposition of the existing craft hero, generated against the approved mobile concept. It is illustrative craft artwork, never a product photograph or maker identity. Its clear wall preserves headline contrast without a CSS tint. Responsive picture sources prevent a second hero photograph from downloading on phones.

## Homepage and catalog amendment — 16 October 2026

This amendment supersedes the manual-only desktop hero and static mobile introduction above. One responsive five-slide carousel preserves the accepted photographs and compact mobile composition. Autoplay advances every four seconds. The user's explicit follow-up removes Pause/Play buttons; retain arrows, selectors and swipe. Hover, keyboard focus, hidden tabs, offscreen viewing and reduced motion pause automatic rotation. Inactive slides remain inert and later images are deferred.

Keep the current shared palette, typography and product-card family. Homepage order is hero, administrator-managed circular category links, Featured compact grid, New arrivals scrolling rail, Shop by category editorial selection, Gifts under ₹1,500 background band, makers, then existing collections. Category photographs reserve their circular frames. Product sections show real approved availability, with honest empty states until inventory is supplied. Mobile cards retain two columns. Operational category management reuses existing portal forms, native selects, checkboxes, notices and tables; no new design system is introduced.

## Storefront editor

The Storefront workspace extends the established AdminDocument shell and shared
forms. It uses a section list, a settings panel and the same customer renderer
in a responsive preview. Native selects and date inputs remain platform-owned.
The public design is initialized from the accepted hero compositions, category
photographs, cards, makers, collections, navigation and footer. Theme overrides
are validated data: supported self-hosted fonts and readable hex colors. The
published configuration supplies body CSS variables; no arbitrary CSS, HTML or
JavaScript is accepted. Existing product facts and photographs remain catalog data.

## Guided storefront customization — 17 October 2026

The editor retains AdminDocument, Roboto/Inter operational typography and shared
forest, ivory and semantic tokens. Grouped workspace navigation leads to a page
selector, compact section outline, contextual inspector and private preview.
Readable names and existing Material Symbols identify section purpose; no maker
portraits or catalog photographs are invented. Outline actions appear on the
selected section. Structured content rows collapse by their own names. Native
selects remain platform-owned. Phones show Sections, Edit and Preview tabs;
forms retain natural document scrolling and only the preview scrolls horizontally.

Editor status distinguishes unsaved changes, an unsaved preview, a saved draft,
submitted review and publication. In-preview edit controls exist only in the
private authoring preview. Customer pages preserve their approved composition;
purchase facts and shipping/returns summaries extend existing product controls.
Runtime owners are storefront-editor.css/js, storefront-preview.js and the shared
ui-standards controls. Existing tokens and homepage fade remain authoritative.
