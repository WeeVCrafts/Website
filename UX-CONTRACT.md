# WeevCrafts UX contract

## Shopping reliability — 17 October 2026

This amendment supersedes earlier references to hero arrow controls. Hero slide
selectors and swipe remain functional, with existing rotation and reduced-motion
rules. Category scroll buttons appear only when needed, expose previous/next
names, disable at the respective boundary and recalculate after content/size
changes. Native scrolling and category links remain available without enhancement.

Wishlist writes commit their visible state only after server confirmation. Every
rendered control for the same product shares saved/pending state, blocks duplicate
activation, retains its visible heart and synchronizes after HTMX settlement.
Failed writes retain the prior state and allow retry; expired sessions use the
existing sign-in prompt. Removing a saved card focuses the next/previous saved
card or the empty-state browsing link. Existing wishlist APIs and CSRF checks own
persistence and authorization; no shopping state is stored in browser storage.

Background cart/wishlist counts refresh independently without product hydration.
Transient reads retry once; cancelled, unauthorized and obsolete reads do not.
Background failure preserves the last usable count, never shows an error toast
and never disables shopping. A count failure cannot turn a confirmed wishlist
write into a reported failure. Older refresh responses cannot overwrite newer
confirmed wishlist changes. Direct action failures still receive useful feedback.

## Customer workflow refinement — 17 October 2026

Customer order history uses server pagination at 20 records per page. Existing
All, Active, Delivered and Cancelled filters apply before pagination; their
counts describe the full owner-scoped history. Empty/out-of-range pages clamp
to a usable page. Detail links carry only validated order-list filter/page state,
and Back to orders restores that state. URL-backed order and return filters use
ordinary navigation semantics.

Legacy payment-state URLs cannot establish payment success. Without an order
reference they open payment activity; with an owned reference they open the
persisted order confirmation. Unknown or foreign references return not found.
Confirmation payment and delivery progress reflect saved financial and parcel
states, including packing, ready-to-deliver and partial deliveries.

Customer profile, address, preference, review, support and order-action forms
reuse shared validation, stable busy controls, duplicate submission prevention,
inline failure recovery and unsaved-change protection. Recoverable failures keep
entered values and selected files in the current document. A session redirect
shows an inline reauthentication instruction instead of silently discarding work.
Nothing saves private form drafts to browser storage. Address deletion and unpaid
order cancellation use the shared confirmation dialog with focus restoration.

Order help links carry an ownership-validated reference into the support form.
If support text saves but photo storage fails, open the saved conversation with
an explicit partial-save notice; do not invite the customer to recreate it.
Notification inbox and notification preferences have distinct navigation labels.
Customer order live-update controls stay inside the content column. Refreshed
forms regain shared validation and confirmation behavior. Privacy requests and
consent changes retain recoverable input; eligible data deletion requires explicit
shared confirmation before its existing server-authorized action.
Unavailable checkout payments explain that the cart is saved and offer a retry
later without exposing deployment configuration.

## Seller workflow amendment — 17 October 2026

Product entry uses Details, Pricing & stock, Photos and Review steps. Completed
steps remain revisitable. Validate before advancing, keep entered values in
memory, reveal the invalid step and focus its first invalid field. Without
JavaScript, all editable sections remain available in one native form. Optional
specifications use a native disclosure. Review reflects entered values; Save
product draft never submits for marketplace approval. Successful create and
edit return to the saved editor. Gallery media and variants remain attached to
that product. A cover failure after saving opens the saved product's Photos
step with retry instructions; it never asks the seller to create another draft.
Submission for review is a separate action and is disabled while edited product
fields have unsaved changes. Server validation retains values and identifies
money/upload corrections when available; session and permission errors remain
closed to unauthorized users.

Inventory filters (`q`, `stock=low|out`, `page`) are server applied before
pagination. Product stock summaries sum active variants; archived variants do
not appear as editable inventory. Adjustments use Add stock or Remove stock,
positive integer quantity and a required reason. The preview is labelled as an
estimate; only the server commits availability. Reserved units remain unchanged.
Legacy signed `delta` submissions remain supported. The selected variant's last
20 adjustments are seller scoped and show before, change, after and reason.
Adjustment success preserves list state and reloads committed quantities.

Orders use one bounded queue with URL-backed search and stage filtering. Page
counts are distinct from matching totals. Opening an order and returning retains
list state. Order details offer only the next permitted stage action, require
carrier/tracking before shipping and reveal custom carrier entry for Other.
Payment eligibility, seller ownership and revision checks remain authoritative.
Live refresh preserves focused controls and unfinished fulfilment input. Enhanced
form conflict responses retain entered details and offer an explicit reload.
Returns navigation remains available on seller order details.

The homepage crossfades outgoing and decoded incoming slides together over
450ms ease-out. The outgoing slide becomes inert immediately and is hidden when
the fade ends. Rapid navigation accumulates requested selections, cancels stale
image work and cleans up animations. Reduced motion switches immediately with
autoplay disabled. Existing four-second autoplay, visibility/focus/hover pauses,
swipe, arrows, selectors and deferred media remain. No Pause/Play button is added.

## Catalog amendment — 16 October 2026

The homepage uses one responsive five-slide hero with a four-second autoplay interval, arrows, selectors and swipe. Per the owner's follow-up, no Pause/Play buttons appear. Hover and keyboard focus pause rotation; leaving resumes a fresh interval. Manual selection/swipe restarts the interval unless paused. Hidden/offscreen pages stop timers; reduced motion disables autoplay. Image loading failures keep the current slide usable.

Admin/Super Admin own category creation, display names, stored photographs, navigation visibility, ordering and product cross-listing. Sellers choose existing primary categories. Category membership may be multiple, but a product always retains a primary category for existing reporting/commissions. Bulk operations apply only to selected products on the current bounded page and commit atomically; primary removal requires replacement. Category image failures preserve prior media and entered metadata. Changes retain server role checks, CSRF and audit reasons, and refresh category/product projections through the database revision; database-only and in-stock searches read fresh availability. The importer defaults to read-only validation and creates only drafts from actual supplied inventory; repeat runs preserve stock and unrelated records.

Customers browse, choose and track Indian craft purchases; sellers author listings and fulfil orders; operators work within their assigned role. English interface, INR currency and existing server date/time policy. Accessibility target: WCAG 2.2 AA; verification reports state coverage rather than claiming certification.

Authoritative behavior: `docs/role-workflows.md`, `docs/api/openapi.yaml`, `docs/secure-customer-update.md`, server authorization and domain lifecycle services. Visual intent: `DESIGN.md`. No payment, permission or retention rules are changed by visual work.

## Canonical UI Map

| Capability | Canonical owner | Source of truth | Allowed variants | Verification |
| --- | --- | --- | --- | --- |
| Select/Listbox | native labelled select in templ | server field contracts | OS popup accepted; boolean `selected?=` | rendered selection, keyboard and popup |
| Form | `ui-standards.js`, existing handlers | server validation | multipart editor, checkout steps, portal updates | invalid/failure/success with preserved input |
| Scrollbar | `ui-standards.css` | runtime CSS tokens | geometry-only table overflow | narrow viewport, focus, computed styles |
| Toast | existing `#ui-toast` and persistent portal notices | server result | success/error text and live region | existing UI checks |
| CRUD | existing HTTP handlers/application services | domain lifecycle, role permissions | existing redirect destinations | Go and portal workflow tests |
| Confirmation | shared native dialog in `ui-standards.js` | consequence on triggering form/button | reversible archive, media removal | Cancel, Escape, focus restoration |

## State and navigation

Actions use buttons; navigation uses links. Sold-out products cannot invoke purchase actions. A sole technical default variant is submitted invisibly, real choices retain labelled selection. Stock state updates all purchase controls. Empty cart offers shopping; checkout appears only with items. Saved address selection agrees with prefilled fields; changing fields indicates a custom address. Shipping remains an estimate until the server confirms a quote. Step transitions retain fields and focus the new heading.

Catalogue query, category, maker, availability, price, sort and page live in URL state. Removing one filter keeps all others and resets page. Facet search is local, has no submitted field name, and does not dispatch remote requests. HTMX requests replace superseded listing work. Dashboard previews are bounded and link to full queues. Table regions are labelled and focusable; cells retain explicit headers and mobile labels.

Support queues navigate to focused cases; status and internal notes belong to the case detail. Reply text remains distinct from internal notes. Only server-allowed status transitions are offered. Existing masking and CSRF remain intact.

Seller money fields show rupees with two decimals; conversion to integer paise occurs exactly on the server boundary. Invalid or over-precision values are rejected, never rounded silently. Legacy integer fields stay supported for existing clients. Create/edit success follows existing owning routes. Failed uploads keep an actionable saved-draft route rather than creating a duplicate on retry.

## Feedback and recovery

Shared form validation uses inline correction text and first-invalid focus, retaining constraint metadata and server validation. Duplicate submissions are blocked; failures keep typed values. Native selects and dates intentionally retain platform popup behavior. Destructive operations keep explicit confirmation and actual server eligibility. Search clear returns focus; no native alert/confirm product dialogs.

Modal navigation and confirmations contain focus, support Escape and restore the trigger. Native textareas retain sufficient height and shared auto-grow. Reduced-motion feedback is immediate. Titles retain the established `{page} - WeevCrafts` convention. Existing 403/access-unavailable and not-found routes remain role-safe; no resource or private data exposure is added.

The shared header's profile disclosure uses a button with an expanded state and ordinary navigation links. Opening moves focus to its first link; Escape closes it and restores the button. Outside clicks and focus leaving the disclosure close it without trapping focus. Opening the navigation drawer closes the disclosure. Wishlist and cart counters retain their existing IDs and live updates as direct header actions. Mobile account navigation collapses into an accessible native disclosure. Checkout includes Back and Edit actions that preserve fields, with focus on the destination heading. Existing private profile-photo access remains session scoped.

Customer password changes require the current password, the existing password-length policy and CSRF; their atomic commit invalidates previous sessions and recovery tokens. Emails distinguish received orders from captured-payment confirmation and include authenticated tracking links. Local payment simulations never appear in production; delivery progress reflects seller-recorded fulfilment rather than fabricated courier scans.

## Company support and account roles

Customer, Seller, Admin and Super Admin are the only assignable roles. Admin and Super Admin share the established operational shell; only Super Admin receives account, role, security, settings and audit controls. Seller navigation contains no Team workflow.

Private confirmations use the shared support shell, semantic statuses, explicit field labels, CSRF-backed forms and duplicate-safe submission keys. Customer replies, internal notes and seller confirmations are separate channels. Each seller sees only its own items and recorded fulfilment events. Closing coordination records never completes delivery or financial processing. See `docs/four-role-workflow.md` for the authoritative transition and rollout rules.


Account updates (2026-10-12): all four roles share a protected notification inbox and unread bell. Opening an update uses an ownership-scoped, CSRF-protected POST and then navigates to its case or order. Company replies notify the case owner; customer/seller replies notify active company operations accounts. Private seller coordination stays separate and never copies message text into notification summaries. Badges refresh every 30 seconds while visible; no browser push permission is required.

Checkout confirmation is an authenticated order-owner page, survives refresh, and uses persisted payment/fulfilment state. In the explicitly configured local payment mode, Place order completes the simulated payment automatically and discloses that no money was charged. Real payments still require verified provider results. Payment retries reuse the saved order; delivery steps are never inferred from order creation. SMTP QA stays isolated from the Resend production sender.

## UI-only implementation, 14 October 2026

The shared UI requirements cover all current role screens. Shopping-specific stories/controls stay in the customer journey; portal screens keep role-aware navigation and task-focused density. HTTP routes, form names, CSRF, permissions, paise arithmetic, stock reservation, payment verification and recorded fulfilment transitions are unchanged.

Search suggestions own their fragment selection (`hx-select="unset"`) rather than inheriting the submitted search form's full-page selector. The existing 300ms debounce, replace/cancellation contract, native submission and IME guards remain. Facet search is local only. Filter queries remain URL state; no authenticated HTMX page snapshots are written to local storage.

Cart rows and amounts are refreshed from existing server-rendered cart HTML after mutations. Client code may enhance the photo from the existing product response, but does not calculate the payable total or assume a variant. Multi-option and unknown-option cards navigate to the detail page. Wishlist removal/saving uses the confirmed response and synchronizes pressed state. Direct Move to cart/Undo and recents are omitted where they would add ambiguity or require unsupported behavior. Quick view is specified in the later customer reference implementation section.

Read-only startup/badge requests stop before navigation teardown and resume after a cancelled navigation or restored page. Financial and other POST requests are not cancelled by this mechanism. Notifications retain the existing visible-page 30-second polling contract and ownership-scoped POST; Read and Unread are textual states. Existing temporary checkout idempotency metadata remains part of the transaction contract, not a source for displayed prices or payment success.

Media upload feedback describes actual preparation, upload and verification phases, with bounded local previews and retained retry selection. A saved draft is not recreated after a later media failure. Unsupported ordering/cover-selection services are not invented. No new storage bucket, provider integration, analytics, email delivery or infrastructure change is part of this release.

Verification scope and outstanding physical-device, native-zoom, screen-reader, live-provider and storage integration checks are recorded in the implementation ledger. Plain server-owned HTTP error responses and download resources retain their existing transport behavior.

## Discovery and account access follow-up, 14 October 2026

The user's preview review explicitly approves browsing without sign-in. Existing public Home, catalogue, category, search, brand/maker and product-detail routes remain public; the frontend must not open an account dialog on arrival or intercept their links. Authentication, role checks, CSRF and protected server routes remain unchanged. Sign-in is requested only for a protected action/link, with Close, Keep browsing and native Escape returning focus without performing that action. Native modified links remain native.

Account screens offer a visible Continue browsing link, a compact policy/help footer, correctly associated fields, password-manager/paste support and the existing password requirements. Sign-in/sign-up links preserve only the existing validated internal `next` path. Shared inline validation runs before the intercepted auth submit; requests are duplicate-guarded, errors retain input and busy state always clears. Email/username access is primary; optional Google access appears only when ready, and its existing consent requirement is unchanged. No new identity provider, guest checkout, guest mutation capability or weaker authentication rule is introduced.

## Customer reference implementation — 15 October 2026

The customer-approved desktop reference establishes two header rows and a light footer; narrow screens keep visible search and existing protected account actions. Existing navigation, counters, profile disclosure, session rules, notification behavior and sign-out endpoints remain authoritative. Seller/Admin/Super Admin shells are outside this visual change.

Quick view uses a native dialog owned by `quick-view.js` and `storefront_reference.templ`. A GET to the existing `/products/{slug}` route with `HX-Request: true` and `HX-Target: product-quick-view-content` returns only its product fragment; ordinary GETs return the full page. Responses retain private/no-store caching. The dialog handles pending, unavailable, failed/retry, sold-out, and real variant-choice states; Close/Escape aborts pending reads and restores focus. Modified product links remain native. A real named variant requires explicit selection even if it is the only choice. Unknown choices never enable purchase. Successful cart addition closes Quick view before opening the existing cart drawer, with focus restoration to the original card. Failures retain the dialog and option and show an inline status. Protected actions keep the established sign-in prompt.

Homepage cards remain compact and listing cards retain Quick view/purchase actions after server-driven wishlist replacement through `card_actions`. The server reloads current variant data for the replacement. Catalog filter keys, price bands, sorting, paging, stale-request replacement and open-sheet preservation remain unchanged. Unsupported facets are omitted; editorial collection links use existing category or text-search queries. Footer marketing access goes to the existing notification-preferences workflow.

The homepage has five slides, automatically advancing every 4 seconds. Visible Pause/Play, previous/next and five selectors stay keyboard-accessible. Hover, focus, manual selection, tab hiding and reduced-motion changes pause playback. Reduced-motion users start paused and may explicitly play. Automatic advances do not announce through the live region. Only activated slides load their responsive photographs.

The normal local Docker preview uses existing real application services and persisted
records. Seed commands and simulated payment/delivery switches are not part of
startup. Missing provider configuration leaves its existing workflow unavailable;
it never produces invented payment or fulfilment success. Air rebuilds the real
Docker app before refreshing its local browser proxy. Internet production validation
still requires HTTPS, secure cookies and the established database/storage TLS rules.

## Storefront editor and publication

Business authority: the accepted Storefront Editor and Super Admin Publishing
plan, documented in `docs/storefront-editor.md`. Admin prepares drafts; only
Super Admin publishes or requests changes. Submitted drafts are locked.
Expected versions reject conflicting saves and publications; proposed form
values remain available and an explicit reload opens the latest saved draft.
Publication shows a comparison with the current storefront. Restoration creates
a new draft and preserves commerce history.

The editor reuses AdminDocument, native selects/dates, ui-standards validation,
confirmation, busy states, notices and unsaved-change protection. Workspace
navigation wraps into an accessible horizontal tab navigation on narrow screens;
settings retain natural document scrolling and preview has its own horizontal
scroll region. Section dragging has numbered order and Move up/down alternatives.
Category discovery uses numeric order then stable slug at every surface.

Previews use the customer renderer, require current Admin/Super Admin access,
are no-store/noindex and accept only GET. Preview scripts disable shopping and
account mutations and skip account/cart hydration. Cart/checkout previews may
show a clearly marked read-only example using an available catalog product.
No inventory, customer history or financial transactions are created for previews.

Category presentation, customer settings and promotion definitions use this
workflow. Legacy POST routes cannot publish changes. Catalog approval, product
assignment, seller fulfilment and historical customer data retain their existing
permissions. Disabled shopping actions are rejected on the server. Checkout
uses a pinned policy revision and stores the order-time policy snapshot.

## Guided editor and direct purchase amendment — 17 October 2026

This amendment supersedes earlier Add to cart success destinations. Every
confirmed product addition navigates to the cart page. Quick view closes before
navigation. The header cart trigger retains its existing drawer behavior.
Buy now checks out only the selected option and quantity through a customer-owned
24-hour context. Normal cart contents and counters remain separate. Starting a
context never reserves stock. Quotes and placement use existing policy, money,
coupon, locking and payment services. Uncertain placement retries reuse the
same context and request key; a completed context opens its saved owned order.
Sign-in retains product selection without automatically placing an order.

Storefront authoring opens an overview, groups tools and retains existing draft
URLs. The outline and preview select the same contextual inspector. Guided help
is dismissible. Live previews debounce 300 ms, ignore obsolete responses, and
never persist a draft. Save is explicit. Invalid proposals retain the last valid
preview, and save failures retain inputs. Unsaved navigation offers Save draft
and continue, Discard changes, or Keep editing through the shared confirmation.
Structural mutations require resolving pending content changes. Draft review,
publication, restoration, locking and conflict rules remain authoritative.

Image selection and upload happen within the editor; upload focus controls apply
to the new asset and do not alter retained release media. Link/content repeaters
use validated structured data with native legacy form fallback. Destination
suggestions come from actual published/draft pages, categories, approved products
and active makers. Presets remain draft presentation data. Empty merchandising
selections warn without creating inventory. Preview-frame communication checks
origin, source window and allowed targets. Customer mutations stay disabled in
previews. Native dialogs contain focus, support Escape and restore their trigger;
the admin navigation shell does not intercept Escape from an unrelated dialog.
