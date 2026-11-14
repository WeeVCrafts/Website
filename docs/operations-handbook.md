# Current operations handbook

Reviewed against source baseline `6b2c936` on 4 November 2026. This is the current
entry point for seller and company procedures. Dated QA reports retain historical
evidence. Candidate changes and unverified boundaries appear in the
[evidence register](release-candidate-2026-11-04.md). An instruction here is not
proof that a training rehearsal or production operation has passed.

## Publication hold

Production keeps unapproved information pages and new checkout unavailable while
`WECRATFS_PUBLIC_CONTENT_APPROVED=false` (the default). Existing order history,
provider callbacks, login and account recovery remain available; adding items to
a cart does not place an order. The owner supplies approved policies, actual
contacts/support hours and permitted content. Install and review that material
on the exact candidate before recording approval and explicitly enabling the
variable. A switch alone is not approval. Isolated development previews remain
available for QA.

## Access and responsibility

| Role | Starting route | Responsibility |
| --- | --- | --- |
| Customer | `/products` | Shopping, personal account, orders, reviews, returns and customer cases. |
| Seller | `/seller-admin` | Its owned business, listings, stock, fulfilment, seller support and earnings. |
| Admin | `/admin` | Marketplace review, customer cases, delivery/returns and finance operations. |
| Super Admin | `/super-admin` | Company operations, accounts, role assignments, security and categories. |

Sign in with username or email and password. The stored account role determines
the destination. Public registration creates a Customer. An email/username alone
does not authenticate or assign a role. Role replacement ends existing sessions.
Admin and Super Admin use no MFA by the owner's 4 November decision; this remains
an explicit security exception. Use individual accounts, not shared operator
credentials. Use **Forgot password** (`/forgot-password`) for verified recovery;
inspect the saved identity after recovery and do not disclose reset links. Seller
Admin is the Seller workspace, not another role. Seller staff,
Support, Finance and Operations are not assignable separate roles.

The company must name the primary and backup people for finance, support,
account recovery and infrastructure before launch. Contact names, response hours,
queue deadlines and policy terms await owner approval; this handbook invents none.
Keep credentials, bank details, recovery secrets and private attachments out of
public reports and ordinary support messages.

## Seller quick start and product entry

Prerequisites: an individual Seller login, an owned business, approved merchant
facts, confirmed stock and permission to use photographs of the exact goods.

1. Open **Onboarding** (`/seller-admin/onboarding`) and inspect application status.
   A pending or suspended business does not become active merely by logging in.
   An active profile is required for listing changes.
2. Open **Products** (`/seller-admin/products`) and choose **Create product draft**.
   Enter Title, an existing Category, Description, Price (INR) and initial Stock
   quantity. INR prices accept two decimal places. Use Advanced specifications
   for materials, dimensions, care and technique only when verified.
3. Leave SKU blank to generate an identifier, or supply the merchant's unique SKU.
   Optional compare-at price must describe a genuine price comparison. Search tags
   accept up to 20 phrases, each at most 40 characters. Do not add unsupported
   provenance, certifications, discounts or organic claims.
4. Upload a JPEG, PNG or WebP cover of the actual product, up to 25 MB. Save the
   draft first, then review its gallery and variants. An unavailable photograph
   is not a reason to substitute an unrelated product image.
5. Confirm every variant, INR price and stock value. **Save draft** persists a
   private record; **Submit for review** is a separate action. After rejection,
   correct the stated issue and resubmit. Approval and active-seller eligibility
   determine customer visibility; saving alone does not publish.
6. Reload the saved product after success. On validation/storage failure retain
   the supplied facts, correct the indicated field, and inspect the saved record
   before retrying. Report repeated failure through Seller Support with product
   ID/SKU, timestamp and error; never publish a duplicate as a workaround.

The owner supplies real inventory and permitted media. The optional catalog CSV
importer's default is validation only; see [catalog management](catalog-management.md).
No operator should create sellable examples to satisfy a product-count target.

## Seller stock and dispatch

Open **Inventory** (`/seller-admin/inventory`) for an existing product. Editing a
listing does not change existing stock. Inspect available and reserved units,
choose the correct variant, record a truthful adjustment with its reason, and
check stock history after saving. A low-stock threshold is a warning setting,
not replenishment. Example only: 10 physical units with 2 reserved leave 8
available. A reservation is not a completed sale; cancellation/expiry and capture
must follow the recorded order lifecycle. A captured sale commits its reservation;
sold totals remain in order history and are not available units. Do not add stock
to bypass a hold.

Archive a listing with a reason to remove customer visibility while retaining
history. Restore returns an archived listing to a private draft that needs review.
Do not delete records needed by orders, returns or earnings.

For dispatch:

1. Open **Orders** (`/seller-admin/orders`) and the specific order. Confirm seller
   items/variants, recorded payment, holds and the permitted next stage. A pending
   or held payment must not be treated as dispatch authorization.
2. Inspect, pick and pack the actual items. For a configured Shiprocket pickup
   location, enter the packed weight and dimensions and create the seller's
   shipment. Assign a courier, schedule pickup and open the provider label.
   Retain the courier handover record.
3. For manual dispatch, book with the approved courier and save the real carrier
   and tracking details. For a live Shiprocket parcel, the app updates fulfilment
   from verified Shiprocket status. A waybill alone leaves the order at Packing.
4. Use the provider label for a Shiprocket parcel. The standard order printout
   contains parcel/address information for manual dispatch.
5. After a conflict, reload before retrying. After a timeout, inspect the saved
   stage and tracking before submitting again. Escalate lost parcels, RTO, damage,
   held payments or invalid transitions to company support; do not invent tracking
   events, deliver an order through support closure or repeatedly advance stages.

Shiprocket is disabled by default. Use a private staging deployment and synthetic
buyers, addresses and parcels. In the deployment secret settings, supply
`WECRATFS_SHIPROCKET_MODE=sandbox`, `SHIPROCKET_API_EMAIL` and
`SHIPROCKET_API_PASSWORD` for a dedicated Shiprocket API user. Configure every
participating seller using its WeevCrafts seller ID. For example

```dotenv
SHIPROCKET_PICKUP_LOCATIONS={"12":"Seller Depot"}
SHIPROCKET_PARCEL_PROFILES={"12":{"postal_code":"110030","packing_grams":50,"packing_cm":2}}
WECRATFS_SHIPROCKET_BOOKINGS_ENABLED=true
```

Use the exact Shiprocket pickup name and PIN. Weigh and measure each sellable
variant in the seller or company product editor. Enter all four shipping fields
in grams and whole centimetres. Checkout estimates one parcel per seller by
summing weight, stacking item heights and adding the configured packing
allowance. Check estimates against representative multi-item parcels. The seller
enters the actual packed measurements when booking.

Run `wecratfs check-shiprocket` in the staging container or
`go run ./cmd/website check-shiprocket` from an equivalent environment. The command
checks the provider account and pickup names and PINs. With bookings enabled it
also checks active seller mappings and approved active variant dimensions in
PostgreSQL. It prints affected seller IDs and creates no shipments. An HTTP 498
during login requires Shiprocket to enable API access for the staging host.

Configure the HTTPS `/webhooks/delivery` endpoint in Shiprocket. Set the same
dedicated secret in Shiprocket and `SHIPROCKET_WEBHOOK_SECRET`. Use at least 32
characters. The receiver queues a status read after matching a saved parcel.
The worker also polls the provider. Sandbox bookings do not advance live
fulfilment. Production uses `WECRATFS_SHIPROCKET_MODE=live`, a separate API user,
production pickup profiles and the webhook secret.

### Verify a release candidate

1. Run `wecratfs check-shiprocket` on the exact private staging image and record
   its revision, migration level and readiness. At checkout use a known
   serviceable PIN and a confirmed unserviceable PIN. Check that the first can
   proceed and the second gets a PIN error before payment. Positive and negative
   eligibility checks are cached for five minutes per app process and parcel route.
2. Simulate a courier API timeout on staging. Complete a synthetic payment and
   confirm **Delivery exceptions** under Admin Orders shows a checkout fallback.
   The seller should be able to record a manual courier. Confirm
   `shipping.courier_errors` rises on protected `/health/metrics`. Then induce a
   pending provider operation and check that `pending_operations` and
   `oldest_pending_seconds` reflect it. `backlog_available` signals whether the
   database snapshot succeeded.
3. Pay for a synthetic two-seller order. At Packing, create the parcel for one
   seller, check available couriers, assign an AWB, schedule pickup, and open the
   label and pickup manifest. Reload after each step and compare the saved
   references with Shiprocket. Confirm callbacks receive HTTP 200 with the
   configured token and HTTP 401 with an invalid token. Confirm that the second
   seller cannot open or change the first seller’s parcel.
4. With a separate prepaid parcel, request cancellation under **Delivery
   exceptions** before the provider reaches Out for Pickup. Until verification
   confirms cancellation, the seller cannot manually dispatch the live parcel.
   After confirmation, assign the case to an operator and record a resolution
   with **Allow manual dispatch**. Verify that both a second cancellation and
   an after-pickup cancellation are refused. The original parcel remains in the
   provider and local audit record. A cancellation does not issue a refund.
5. Record the staging evidence for checkout, real provider label, live-status
   mapping, cancellation, alerts and rollback. The sandbox tracking playground
   can check callback delivery, while advancement requires an authenticated status
   read for the same saved parcel. Use the provider-supported scan or simulation
   to verify that one seller’s parcel advances independently and that the shopper
   gets the same tracking information. Wait for the owner’s activation approval
   before enabling bookings for the mapped sellers in production.

If new bookings must stop, set `WECRATFS_SHIPROCKET_BOOKINGS_ENABLED=false` and
keep the existing mode and credentials so polling, cancellations and document
recovery can continue. Checkout resumes the standard delivery policy for new
orders. A live parcel with a saved shipment record stays provider-managed until
its terminal status or approved manual handoff. In a pending case, check the
merchant reference in Shiprocket before taking any replacement action. Returns
and post-pickup exceptions use the existing support process.

The 14 November 2026 playground rehearsal authenticated the sandbox API user,
registered a synthetic depot and created one prepaid order. Order details and
courier serviceability returned successfully. Blue Dart, Xpressbees Surface,
DTDC 500g and India Post returned no waybill. A repeat check used the saved parcel's
actual pickup and delivery PIN codes. Shiprocket listed Shadowfax Surface as
serviceable, but assignment returned HTTP 200 with `awb_assign_status: 0` and
`Awb Assignment Failed`. Ekart Logistics Surface subsequently issued waybill
`SRTP2190434957` for sandbox shipment `352594652`. The playground confirmed pickup
and returned a manifest PDF URL. The label endpoint failed both before and after
manifest generation, and the combined label and invoice request reported one
failed document. The waybill lookup returned no tracking scans. Direct
authentication from the local app host returned HTTP 498. Label generation,
delivery scans and hosted app-to-sandbox access still require verification.
The local callback rehearsal verified rejection of invalid credentials, exact
matching of a known parcel, and replay-safe delivery updates in the shopper's
order. No hosted callback URL has been registered for this candidate.

## Seller returns, private confirmations and earnings

Open **Returns** (`/seller-admin/returns`) to inspect status. The Seller list is
read-only; company staff decide customer-facing returns and refunds. Open
**Support** (`/seller-admin/support`) for a seller case and its existing replies.
Private confirmation requests from company staff contain only this seller's order
context. Reply to the correct request, include factual evidence, and inspect the
saved response. Do not paste unrelated customer/private information into it.
Closed requests reject replies; ask company support to open a follow-up. Customer
attachments and internal company notes are not automatically shared with sellers.

Open **Earnings** (`/seller-admin/earnings`). Reconcile merchandise value, saved
item discounts, commission, refund reversals, open balance and settlements to order
references. Shipping is recorded separately; it is not automatically payable as
seller merchandise earnings. A refund reverses the original saved sale amounts,
not a new calculation using today's price or commission rule. Ordinary receipts
are not GST tax invoices. An approved settlement is not evidence of a bank credit.
Compare completed-transfer reference/date/amount with the actual bank credit and
report mismatches with the settlement ID through a private company channel.

Payout cadence, eligibility, reserves, fees and negative-balance handling await
approved policy. Do not promise a date or fee absent that policy. Legacy paid
records missing bank evidence require reconciliation; never create a fictitious UTR.

## Company accounts, review and publication

Super Admin opens **Users** (`/super-admin/users`) to create/search/manage accounts.
Confirm identity, choose one role, provide the required confirmation/audit reason,
save, and inspect the account. Password/role changes revoke sessions; changing an
email clears its verification. Suspension prevents access. Protected references
block physical deletion, and the last active Super Admin cannot be removed.

If a Seller has its role but no owned business, Super Admin opens **Roles**
(`/super-admin/roles`), reapplies Seller with confirmation and an audit reason,
then asks the seller to sign in again. The repair creates a pending business when
missing; it does not approve it or attach another seller's business by name/email.
Inspect unfinished orders/returns before role changes or deletion. See
[four-role operations](four-role-workflow.md) for transaction and migration rules.

Admin/Super Admin use their **Sellers**, **Products** and **Reviews** queues to
inspect submitted facts, photographs, stock and policy compliance. Record the
review reason; after saving inspect the exact object's state and public visibility.
Reject unsupported claims and request correction rather than inventing evidence.
Appeal/escalation ownership and prohibited categories require owner-approved policy.

Super Admin opens **Products > Add category / Manage categories**. Confirm name,
stable slug, photograph, display order and visibility. **Save category publishes
immediately.** Hiding removes discovery links without deleting history. Assign
primary/additional categories through the product editor or category product
management. Removing referenced categories is blocked. A primary membership needs
a replacement before removal.

The general Super Admin Storefront editor is retired. Retained Admin drafts do
not provide a current publishing path. Preserve drafts/releases. Obtain owner
authorization for other homepage, policy or navigation changes and use a reviewed
code/content change until a replacement publication workflow is approved.

## Company payment, refund and payout procedures

Open **Payment reviews** (`/admin/payment-reviews` or
`/super-admin/payment-reviews`). Before approving fulfilment or refunding, compare
app order, provider order/payment IDs, captured amount/currency, reservation,
recorded exception, refund status and seller ledger. A browser message is not
payment evidence. Record the decision reason and inspect the saved case after
success. An uncertain provider outcome needs reconciliation, not another charge.

Do not retry a refund POST after timeout, process failure or unknown outcome.
Review its durable submission/recovery state and provider reference. Known
references are reconciled by the existing recovery path. Retry only through the
authorized reviewed action and its expected refund identifier when eligible.
Escalate unmatched references/amounts or duplicate candidates to the finance owner.
Support closure and seller confirmation never capture or refund money.

The release candidate introduces the following manual payout workflow. Its exact
field labels and runtime verification remain pending in the evidence register;
do not use an older **Record mock payout** button operationally.

1. The preparer opens **Finance > Settlements** (`/admin/finance?workspace=settlements`
   or `/super-admin/finance?workspace=settlements`) and chooses **Prepare settlement
   batches** from eligible open ledger entries.
2. Expand **Prepare recipient for approval**. Record **Account holder**, **Bank
   name**, **Last four account digits**, **IFSC** and **Recipient verification
   instructions**. Attach **Private recipient evidence** (PDF/PNG/JPEG, up to
   5 MiB), then **Save recipient version**. Keep full banking information in the
   protected evidence channel; do not add it to public notes.
3. A different operator downloads the private evidence, independently verifies
   seller/bank/amount and chooses **Approve recipient and batch**. New recipient
   preparation before transfer resets approval and requires another review.
4. The original preparer makes the bank transfer externally. Expand **Record
   completed bank transfer**. Enter **Transferred amount in paise** (100 paise =
   INR 1), **Transfer date**, **Bank UTR/reference** and **Private transfer
   evidence**, then **Submit transfer evidence**. No button sends money.
5. The original independent approver checks recipient version, amount, reference,
   protected proof, holds/refunds and actual bank reconciliation, then selects
   **Confirm transfer paid**. Never approve one's own preparation or use a
   generated settlement ID as a bank reference.
6. After timeout/conflict, inspect the batch, bank and ledger before retrying.
   Submitted transfer evidence cannot be replaced or cancelled through this
   workflow; mismatches require external reconciliation. Do not create a second
   batch or transfer to hide an uncertain outcome. Historical paid rows without
   proof display **Historical paid status: transfer evidence requires
   reconciliation**; investigate rather than manufacturing evidence.

At daily close compare captured orders, provider payments, refunds, bank credits,
manual transfers and ledger/settlement balances. Track each mismatch with its ID,
age, owner, next action and private evidence. Review older unsettled/uncertain
items at handover. At month end reconcile totals to the accounting system; do not
recompute saved historical amounts. Accounting treatment awaits owner approval.

## Daily operations, incidents and glossary

The opening operator checks paid-unprocessed orders, dispatch delays, delivery
issues, open returns/cases/private confirmations, payment reviews, uncertain
refunds, unpaid settlements and dependency/worker alerts. Assign each case to a
person and record next action; the owner must approve deadlines and backup cover.
Weekly review access, outstanding reconciliation and recurring delivery failures.
Monthly review close, retention, capacity and recovery evidence. Restore frequency
and targets require explicit owner approval, not a claimed completed drill.

For payment inconsistency, hold affected fulfilment/payout, preserve IDs and
provider evidence, and involve the finance owner. For storage outage, preserve
saved metadata and queued retries; do not substitute sample images. For access
loss, use verified account recovery or an authorized individual Super Admin;
never demote the last administrator or share a password. For failed deployment,
database pressure or suspected breach, involve the infrastructure owner and follow
the [release/recovery checklist](release-checklist.md). Record detection time,
affected operations, containment, owner, recovery evidence and follow-up. No
live incident rehearsal is claimed by this handbook.

| Term | Meaning |
| --- | --- |
| Seller status | Business approval/availability; independent from account role and product review. |
| Product status | Draft/review/approved/archived state; independent from stock and seller availability. |
| Payment status | Provider-confirmed financial state; independent from fulfilment and support case state. |
| Fulfilment status | Seller-recorded progress for manual parcels and verified courier progress for live Shiprocket parcels. |
| Return status | Company return decision/receipt workflow; not a completed refund. |
| Refund status | Durable submission/provider/recovery state; unknown does not mean failed. |
| Settlement status | Batch preparation/approval/transfer confirmation; approved does not mean paid. |

Before accepting a new release, a representative Seller and both company roles
must perform their tasks in isolated staging using this handbook without developer
coaching. Record route/label differences, errors, keyboard/narrow-screen results,
operator names and tested release. This training remains unverified until recorded.
