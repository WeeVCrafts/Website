---
version: alpha
name: WeeVCrafts
description: An Indian craft marketplace with an editorial storefront and calm operational workspaces.
colors:
  primary: "#124534"
  background: "#f6f0e6"
  surface: "#fffdf8"
  ink: "#2d201b"
  muted: "#625548"
  accent: "#9d4031"
  border: "#dfd3c5"
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
    backgroundColor: "{colors.accent}"
    textColor: "{colors.surface}"
  input:
    typography: "{typography.body}"
    textColor: "{colors.ink}"
  field-hint:
    textColor: "{colors.muted}"
  divider:
    backgroundColor: "{colors.border}"
---

# WeeVCrafts visual standards

## Overview

Preserve the accepted Indian craft exhibition identity: ivory paper, forest green, terracotta accents, restrained serif headings and genuine object photography. The memorable element is the relationship between a maker and the object. Never manufacture maker biographies, product specifications or portraits to fill a layout. Storefront expression gives way to task clarity in seller, admin, finance, operations and support workspaces.

## Colors

Ownership model B: `web/assets/css/theme.css` owns storefront tokens. Existing `admin.css`, `seller-admin.css` and `support-portal.css` own their role-specific palettes; these are intentional variants, not a storefront rebrand. Shared semantic treatments in `ui-standards.css` use their existing variables. Success means completed or available; danger means failed or destructive; warning means pending or requiring attention. Every status also has a readable text label. Do not use color as the only signal.

## Typography

Playfair Display headings; DM Sans body and interface text. Controls use 16px, data tables at least 14px and a 1.5 line height. Prices, quantities and identifiers use tabular numerals. Headings wrap naturally; avoid forced line breaks that isolate articles. Long instructions and validation errors wrap.

## Layout

Storefront max width remains 1240px with 24px desktop gutters and 16px narrow gutters. The homepage order is hero, shallow category rail, available product selection, makers, curated collections. Avoid repeated mission statements before products. Mobile header keeps 44px controls, a compact brand/action row and prominent search. Details retain the mobile purchase bar without covering focus. Operational dashboards show permitted actions, summaries and bounded previews; full queues own dataset navigation. Document scrolling remains natural; only genuine tables may scroll horizontally.

The shared storefront header places Menu at the top left and centers the logo and branding. The next row runs from delivery location at the left to search near the right and then the profile control. On narrow screens, delivery location sits above the search/profile row, preserving this reading and keyboard order. Wishlist and cart are direct, counted shopping actions alongside the profile control. Account, notifications and sign out remain in the profile disclosure; a direct logout button remains at the top right. Its trigger displays the signed-in customer's existing private photo when available. On narrow account pages, a collapsible Account menu precedes the current section without delaying it behind the full navigation.

## Elevation & Depth

Keep existing light panel shadows; no new decorative gradients, floating ornaments or animated data rows. Hero photographs have no added tint. Media always reserves its aspect ratio and uses an honest missing-image fallback.

## Shapes

Controls use the existing 6px radius. Panels use restrained rounding. Hearts retain the existing circular shape. No giant rounded page wrappers.

## Components

| Role | Runtime owner | Consumers |
| --- | --- | --- |
| Paper/ink/brand/type | `theme.css` root variables | shared header, footer, product cards, pages |
| Storefront refinements | `customer-polish.css` | compact header, hero, facets, account |
| Shopping experience / responsive media | `experience.css`, `experience.js`, shared templ components | manual hero, filters, gallery, mobile purchase controls |
| Semantic status and operational tables | `ui-standards.css`, `components/presentation.go` | all role portals and order history |
| Native fields and validation | `ui-standards.js` | seller, support, admin and checkout |
| Canonical media | product database and upload workflow | cards, galleries, seller editor |

Pointer press feedback uses 120ms. Pointer-driven checkout transitions use 160ms opacity/transform feedback; keyboard and reduced-motion transitions are immediate. Pending actions use stable-size busy indicators. Drawer semantics and focus restoration follow existing shared implementations. Keyboard focus is visible on links, buttons, fields and overflow regions. Global scrollbars stay operable; forced-colors uses system colors.

## Do's and Don'ts

Keep the real logo, catalogue values, seller permissions and lifecycle rules. Use verified content and customer-facing option names. Do not replace Go/templ with another framework or turn generated concept product names/photos into real catalogue data. Generated references guide composition; maintained data and existing accepted assets override their illustrative content.

Header update: the storefront uses one sticky toolbar at every scroll position. Menu, wordmark, delivery, search and account actions share one row. At narrow widths delivery and Wishlist move to the drawer; search expands across the toolbar with Escape restoring focus. Account sidebars use compact identity panels and 44px navigation rows.

## 14 October 2026 implementation contract

The foundation, focus, feedback, form and reflow rules apply to all four current roles and their detail screens. Shared badges, tables, dialogs and labels inherit the document's interface family: DM Sans in the storefront, Inter in operational portals. Storefront headings remain Playfair Display; operational headings remain Roboto. Font preloads and CSS use the same WOFF2 URLs. Original TTFs and licences remain maintained sources. The Material Symbols subset preserves the required ligature feature and referenced glyphs.

Controls use 44px comfort targets, 16px entry text, visible 2px focus outlines, 6px corners and a meaningful control border (#8e7d6b); decorative dividers keep the lighter border. Hero navigation is manual, three stories maximum, 250ms opacity only, with inactive links inert. Later images/video are deferred. Product media reserves a square frame and uses contain rather than cropping the object. Product photography, real choices and current prices override generated composition references.

Listing updates preserve the open sheet, facet searches, scroll, expanded groups and active control through HTMX settlement. Hidden mobile sheets are inert immediately, including before enhancement. Drawer/sheet motion is 180ms; keyboard and reduced-motion access is immediate. One contextual purchase region is visible; it yields to inline controls, policy links and text-entry focus. No bottom navigation or promotional layer is added.

Operational data uses native tables with scoped column/row headers and named, focusable overflow regions. Narrow summaries retain every value/action and explicit cell labels. Private conversations, internal notes and seller confirmations keep their established boundaries. Busy controls retain their text and geometry; success styling waits for the actual response. Archive/restoration confirmations use consequence-specific intent, cancel-first focus and Escape. An unsaved editor stays in memory and offers Keep editing before navigation.

See the dated implementation ledger for actual route/state coverage and limitations. Physical devices, browser-native zoom and screen readers remain release checks; a browser score is not certification.
