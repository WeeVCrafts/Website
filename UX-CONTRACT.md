# WeeVCrafts UX contract

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

Modal navigation and confirmations contain focus, support Escape and restore the trigger. Native textareas retain sufficient height and shared auto-grow. Reduced-motion feedback is immediate. Titles retain the established `{page} - Weevcrafts` convention. Existing 403/access-unavailable and not-found routes remain role-safe; no resource or private data exposure is added.

The shared header's profile disclosure uses a button with an expanded state and ordinary navigation links. Opening moves focus to its first link; Escape closes it and restores the button. Outside clicks and focus leaving the disclosure close it without trapping focus. Opening the navigation drawer closes the disclosure. Wishlist and cart counters retain their existing IDs and live updates as direct header actions. Mobile account navigation collapses into an accessible native disclosure. Checkout includes Back and Edit actions that preserve fields, with focus on the destination heading. Existing private profile-photo access remains session scoped.

Customer password changes require the current password, the existing password-length policy and CSRF; their atomic commit invalidates previous sessions and recovery tokens. Emails distinguish received orders from captured-payment confirmation and include authenticated tracking links. Local payment simulations never appear in production; delivery progress reflects seller-recorded fulfilment rather than fabricated courier scans.

## Company support and account roles

Customer, Seller, Admin and Super Admin are the only assignable roles. Admin and Super Admin share the established operational shell; only Super Admin receives account, role, security, settings and audit controls. Seller navigation contains no Team workflow.

Private confirmations use the shared support shell, semantic statuses, explicit field labels, CSRF-backed forms and duplicate-safe submission keys. Customer replies, internal notes and seller confirmations are separate channels. Each seller sees only its own items and recorded fulfilment events. Closing coordination records never completes delivery or financial processing. See `docs/four-role-workflow.md` for the authoritative transition and rollout rules.


Account updates (2026-10-12): all four roles share a protected notification inbox and unread bell. Opening an update uses an ownership-scoped, CSRF-protected POST and then navigates to its case or order. Company replies notify the case owner; customer/seller replies notify active company operations accounts. Private seller coordination stays separate and never copies message text into notification summaries. Badges refresh every 30 seconds while visible; no browser push permission is required.

Checkout confirmation is an authenticated order-owner page, survives refresh, and uses persisted payment/fulfilment state. In the explicitly configured local payment mode, Place order completes the simulated payment automatically and discloses that no money was charged. Real payments still require verified provider results. Payment retries reuse the saved order; delivery steps are never inferred from order creation. SMTP QA stays isolated from the Resend production sender.
