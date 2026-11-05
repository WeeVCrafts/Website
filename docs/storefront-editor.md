# Storefront editor and publishing

**2 November 2026 update:** The owner has retired the current Super Admin
editor pending a replacement brief. Its navigation and shortcuts are removed;
legacy bookmarks return to the dashboard and mutations return 410. Published
customer pages, releases and saved drafts remain intact. The documentation below
records the retained Admin draft workflow and the historical publishing model.

## Current operator instructions

The general Super Admin Storefront editor is retired. Do not follow the historical publish/restore procedure below. Retained Admin drafts do not provide a current Super Admin publishing path. Leave published releases and drafts intact until the owner approves a replacement workflow.

Super Admin manages categories through **Products > Add category / Manage categories**. **Save category publishes immediately.** Read [catalog management](catalog-management.md) and the [current handbook](operations-handbook.md). Owner approval is required for other homepage, policy and navigation changes; never reopen retired mutation routes to make a change.

## Historical editor record

The retained customer renderer reads a published PostgreSQL release. The following sections describe earlier editor releases and their verification, not an available current publication procedure.

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

## Guided customization — 17 October 2026

The overview separates the current published release from editable and submitted
drafts. Continue editing resumes an existing draft; creating a draft remains an
explicit action. Existing editor links remain valid. Page design, Brand &
navigation, Content, Store settings and Review & publish group the tools.

Select a page, then choose a section in the outline or Edit preview. The selected
section owns its content controls and ordering actions. Hidden sections remain
available in the outline. Phones offer Sections, Edit and Preview tabs. Guidance
can be dismissed and reopened. Protected product and checkout content explains
that catalog facts and commerce services remain authoritative.

Typing refreshes a private, unsaved preview after 300 ms using the customer
renderer. Save draft remains explicit. Invalid proposals keep the last valid
preview and entered fields. Unsaved navigation offers Save draft and continue,
Discard changes or Keep editing. Structural actions require saving or explicitly
discarding current edits first. Version conflicts never overwrite another save.

The authenticated POST `/admin/storefront/render-preview` (also under
`/super-admin`) accepts the existing draft/version, workspace, selected target
and proposed form fields. It renders without persisting, requires CSRF, rejects
locked/stale drafts, and returns no-store JSON containing HTML or a validation
error. Saved GET previews remain separate and read-only. Frame messages are
accepted only from the current same-origin preview window and known targets.

Photograph fields offer a searchable, paginated library and in-place uploads,
including descriptions and focal controls for new photographs. Uploading retains
the asset without publishing it. URL entry remains under Advanced. Structured
navigation, footer, collection, FAQ and trust rows use additive JSON form fields;
legacy text forms remain usable without JavaScript. Destination suggestions use
real pages, categories, approved products and active makers.

Section starting points cover curated products, gifts, collection tiles, maker
stories, promotional banners and FAQs. They never create sellable inventory,
fabricate a maker, or publish automatically. Readiness findings link to the
relevant editor. Empty selected collections are warnings. Publication retains
the existing authorization, expected-version and review-note requirements.

## Selected-item checkout

Add to cart opens `/cart` after a confirmed addition. The header cart drawer stays
available through its existing trigger. Buy now opens `/checkout?intent=...` for
only the chosen variant and quantity. The normal shopping cart remains intact,
including any matching variants; its badge still counts only normal cart items.

POST `/api/v1/checkout/intents` requires customer ownership, CSRF, an idempotency
key, an available variant and a quantity from 1 to 99. It returns an opaque intent
ID, checkout URL and expiry. The intent expires after 24 hours before placement;
creating it does not reserve stock. Quote requests accept `intent`, and checkout
placement accepts the optional `checkout_intent_id`. Requests without an intent
retain normal cart behavior. Retries use the same context and saved order.

Migration 50 adds cart purpose, direct-purchase identity and the saved order link.
Existing carts become shopping carts. Normal-cart uniqueness, lookup, merging
and counts exclude direct purchases. Both purchase modes retain the same money,
policy, coupon, stock-locking, reservation and payment verification services.
Migration rollback must preserve new purchase contexts and saved orders; never
reclassify active Buy now carts as normal carts or restore a pre-order database.

## Guided homepage and category editing - 2 November 2026

Continue an existing draft from Overview or explicitly create one from the live
release. Homepage, Slideshow and Categories are primary destinations; the
remaining workspaces stay in More tools. Add row offers the existing starting
points and section types. Select a row/slide from the outline or private preview
to edit its content beside the preview. Move up/down provides keyboard reordering.

Product choices show photographs and a selection count. Move selected products
to set their manual display order; saved selections remain visible across picker
pages. Catalog products and featured status have a direct link to Products.
Category search filters the draft's bounded category list. Add/Edit category
opens a modal with name, stable slug, description, image library/upload, display
order and discovery visibility. New category presentation and photograph save
together. Image uploads show progress and retain the selection for retry.

Super Admin can delete editable drafts from Overview after a permanent-deletion
confirmation. The server checks current role, CSRF, state and version in a
transaction and records an audit event without copying draft contents. Submitted
drafts must first return to Draft; published versions cannot be deleted. Images,
releases, the live release pointer and commerce records remain unchanged.

Account editing is consolidated in Users with a modal; access assignments remain
in Roles. Security retains audit activity and links to those owning screens.

Verification for this update: `go test ./...`, the focused PostgreSQL draft
removal integration test, template generation, CSS/JavaScript builds and the
strict premium audit passed (zero audit findings). Eleven browser scenarios
passed: five new Chromium management flows, the account popup in Firefox and
WebKit, and four existing Chromium guided-editor regressions. Checks covered
320/390/768/1440px layouts, keyboard focus/cancellation, retained validation
values, upload failure/retry/progress, manual product order, draft deletion
protection, private previews, native editor forms and seller profile navigation
and saving. The account dialog and existing editor accessibility checks reported
no WCAG 2 A/AA axe violations in their tested scopes.

Verification used the dedicated loopback app on port 18085, the existing
`storefront_guided_20261017` QA database and a separate
`weevcrafts-admin-guided-qa` media bucket. No production deployment, customer
record mutation or credential reset was performed. Real-device/screen-reader
certification and a production rollout are outside this local verification.
