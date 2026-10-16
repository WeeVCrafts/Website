# Storefront editor and publishing

The Go/templ Storefront workspace lives at `/admin/storefront` and
`/super-admin/storefront`. The customer site reads a published PostgreSQL release.
Routine changes use this workspace without rebuilding or deploying the app.

## Roles and workflow

1. Choose **New draft from published design**, then edit the relevant workspace.
2. Save changes. Preview uses the same customer templates as the live site.
3. Admin selects **Submit for review**. Submitted versions cannot be edited.
4. Super Admin reviews the comparison and responsive preview, adds a review note,
   then publishes or requests changes. Requested changes return the version to Draft.
5. **Restore as draft** copies a past release into a new draft based on the current
   release. It still requires review and publication.

Super Admin may publish its own reviewed draft. Seller and customer accounts
cannot access the editor. The existing four-role model is unchanged.

Each save and publication requires the expected draft version. Concurrent changes
return a conflict and preserve the proposed form values. Reload the latest saved
draft and apply the intended change again. A draft based on an older published
release cannot overwrite a newer release; start a draft from the current design
or restore the required release explicitly.

## Release 1: homepage and global appearance

- Hero photographs, mobile alternatives, accessible descriptions, headings,
  supporting text, buttons, visibility and numbered slide order. Four-second
  autoplay, swipe, reduced motion and existing focus/visibility safeguards remain.
- Homepage sections support numbered order, stable ID ties, Move up/down, drag,
  duplication, hiding and removal. Supported sections are hero, categories,
  products, makers, collections, banners, image-and-text, rich text, FAQ and trust.
- Product sections use featured order, newest arrivals, category rules or selected
  catalog products. The picker searches and paginates the catalog, retaining
  selected products. Rules include availability and INR price limits. Existing
  grid, rail, editorial and contrasting-band cards are reused.
- Category names, photographs, visibility and numbering publish together across
  discovery, navigation, listings and filters. Catalog product assignments remain
  available in **Products → Categories → Manage products**. Publish a new category
  before assigning products to it. Removing a referenced category is rejected;
  hide it to preserve its catalog and commission assignments.
- Navigation, footer groups, policy links, trust content, logo, supported fonts
  and readable color combinations use the same publication workflow.

Default content is copied from the accepted storefront and existing category,
customer-setting and promotion records. The legacy category and settings POST
routes cannot publish changes. Marketing retains its usage view and links to
Storefront for definition changes.

## Release 2: customer templates and content pages

The same sections can be used on category/search, product, maker/brand, wishlist,
cart, checkout, account/order and authentication templates. Default templates
apply unless a `category:slug` or `product:slug` assignment matches. Product
assignments take precedence. Overrides can be removed to restore the default.

Template controls include heading/SEO title, description, artwork, introductory
content and surrounding sections. The default introduction may be replaced with
configured sections. Authentication artwork and listing banners use the selected
template photograph. Product facts, prices, variants and photographs remain in
Catalog; template controls do not change inventory or product claims.

Built-in information and policy copy is available in Content pages. It continues
using the accepted renderer until edited. Additional published pages use
`/pages/{slug}`. Page metadata includes title, description and sharing image.
Rich text supports paragraphs, bold, emphasis and validated links without HTML.
Contact content retains the customer support form. Essential checkout, identity,
order-history, support and privacy components remain connected to their services.
The Page templates workspace includes a coverage list.

Return policy content is editable at `/pages/returns`. The existing `/returns`
account workflow remains available for authenticated customers and their requests.

## Media and preview

Media uploads accept JPEG, PNG and WebP up to 8 MiB and 16 megapixels. Images are
decoded and re-encoded as JPEG, with responsive variants, alternative text and
focal coordinates. Copy the permanent library URL into an image field. Use a
separate upload for a mobile composition where needed.

Unpublished uploads require current company-admin access. Published media is
served through the application's media route and retained for release history.
Replacing a photograph does not delete the previous object. Legacy category
photos referenced by release history remain resolvable after replacement.

Previews require authentication, send no-store/noindex headers and accept only
GET. Navigation stays within the private preview. Shopping, login, profile,
wishlist and other customer mutations are disabled; account/cart hydration is
skipped. Cart/checkout previews can show a clearly marked read-only example from
an available catalog product, without creating a cart or reserving stock.

## Release 3: shopping policies

Store settings contain INR shipping amounts, the free-shipping threshold,
optional Indian PIN restrictions, return windows, optional shopping actions and
availability of payment options already configured by the app. Credentials and
runtime payment mode are not editable here. The initial policy remains ₹99 below
₹1,500 and free shipping from ₹1,500, with new restrictions inactive.

Coupon definitions, product eligibility, dates, limits and activation require
publication. Dates use India time with an inclusive end date. Publishing and
restoration update definitions while preserving redemption rows and counters.
Removed definitions are archived, never deleted from financial history.

Read-only examples calculate proposed shipping and coupon effects without
creating orders or consuming promotions. Selected-product coupons and
per-customer usage are rechecked against the actual cart at checkout.

Cart totals use the published shipping calculation. Checkout quotes include
`policy_revision`; the client sends it as `expected_policy_revision` with the
reviewed total. Placement revalidates the live policy and stock in its transaction.
New orders store their policy revision and policy snapshot. Return eligibility
uses the recorded order policy and first delivery time. Older orders retain the
previous eligibility behavior and every recorded monetary amount.

## Persistence and rollout

Migration 49 adds drafts, immutable releases, the release pointer, retained media
metadata and nullable order-policy/delivery fields. Publication locks the release
pointer and applies validated category presentation, promotion definitions and
the new snapshot atomically. Failed publication preserves the previous release.
Catalog cache keys include the storefront revision. Runtime rendering pins a
published configuration for each customer request.

Back up production before applying the migration. Bootstrap creates the first
release only when no release exists; application restarts do not overwrite drafts
or published changes. Verify existing accounts, catalog, inventory and order
totals after deployment. Rollback must not restore an older database over new
orders or payments. Content restoration is performed through a new draft.

Each release has a verification gate: role/CSRF/conflict and homepage/media flow;
customer-template/content-page rendering and functional-component preservation;
then policy, coupon, quote/order and historical-snapshot invariants. Tests use
isolated databases. Browser mutation suites refuse production URLs.

Scheduled publication, unrestricted HTML/CSS/JavaScript, new integrations,
tax/commission/payout changes and merchant-supplied inventory/policy wording are
outside these releases.

## Brand identity

Use the exact customer-facing brand spelling **WeevCrafts** and a locally hosted
Cormorant Garamond Semibold wordmark. The owner requested curved, softer serif
letterforms and authorized the final choice. Preserve the woven logo mark and
existing interface typography. Account identifiers, URLs, credentials and
historical business records retain their original values.

## Verification

The release is checked with the full Go suite against an isolated PostgreSQL
database, templ generation, pinned sqlc 1.30.0 output comparison, the existing
Tailwind/JavaScript build and the strict premium UI audit. Focused storage and
renderer tests cover subsequent integration corrections.

Playwright covers the four homepage sections, autoplay/swipe and image failures;
draft isolation, stale edits, keyboard operation and publication/restoration;
private uploads and retained published media; content pages, templates and
shopping-feature publication. Responsive checks include 320, 390, 768 and 1440
pixels and verify the actual theme fonts, heading sizes and page overflow.
Customer-route and role-portal regressions preserve their existing services.

Genuine inventory publication remains dependent on merchant-supplied CSV details,
confirmed stock and matching product photographs. Use the existing opt-in
importer described in [catalog management](catalog-management.md); do not create
sample sellable listings to meet the launch product-count target.
