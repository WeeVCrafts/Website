# Cross-role order integration

The existing Go, templ, PostgreSQL and Redis application remains the owner of
checkout, payment, fulfillment and support. No additional frontend application or
provider-side storefront/order-sync channel is required.

## Entry points

| Actor | Order workspace | Setup and requests |
| --- | --- | --- |
| Customer | `/orders/{number}` | `/returns` |
| Seller | `/seller-admin/orders/{number}` | `/seller-admin/setup`, `/seller-admin/returns` |
| Admin | `/admin/orders/{number}` | `/admin/returns` |
| Super Admin | `/super-admin/orders/{number}` | `/super-admin/fulfillment/reviews`, `/super-admin/returns` |

Order sections are `?tab=overview`, `packages`, `documents`, `returns` and
`support`. Existing fulfillment order links redirect to Packages. Authorized
invoice, credit-note, packing-slip and private-evidence downloads keep their
existing routes. Historical orders retain their recorded policy and validated
request/fulfillment actions.

## Common facts, independent states

The order workspace reads its summary, fulfillment data and requests from one
read-only repeatable-read transaction. Role and ownership checks use current
database records. Sellers receive their own item/package/document/request data;
customer-wide provider charge and refund amounts are excluded from seller
financial summaries.

Financial summaries retain original totals, captured provider payments,
provider-confirmed refunds, unresolved refunds, applied/restored credit and net
provider payments. Item quantities retain original, cancelled and refunded
counts independently; refunded quantities can overlap cancelled quantities.
Cancellation and refund are separate processes.

`commerce_request_summary` combines typed historical returns/refunds and modern
returns, claims and refunds. Keys include the record kind to avoid ID collisions.
The common buckets are Needs review, In progress, Completed and Rejected.
Counts describe process records; a claim and its resulting refund remain
independent records. Historical whole-order amounts and complaint text are not
presented as a seller's item outcome.

Owned-order API responses preserve existing fields and add financial/item
progress plus bounded package, document and request collections. Overflow flags
indicate additional records; the normal order workspace remains available.

## Seller details and verification

1. Seller opens Complete product setup from Products or Store profile.
2. Seller records legal invoice identity once per business, and tax/return
   details for each option. Sellers can assign reusable measured parcel profiles in Fulfillment Setup. Existing variant measurements override profiles. Stock stays in Inventory and pickup verification stays with Super Admin.
3. Save draft preserves work without changing approved profiles or checkout.
4. Submit for review records an immutable version and places a hold on new
   purchases for that seller (invoice) or option (tax).
5. Super Admin compares submitted and verified values, records verification
   notes, and approves or requests changes. Requested changes retain the hold.
6. Approval publishes only that reviewed version and clears its review hold.
   Other stock, parcel, pickup, tax, policy and launch checks still apply.

Revisions reject stale edits/reviews. Identical submitted/approved retries are
idempotent. Audit history and account notifications are recorded transactionally;
the existing worker sends email and retains provider delivery outcomes.
Invoice numbering remains platform-managed. New orders freeze seller invoice
identity when items are created, alongside recorded item tax data. Profile edits
never rewrite an existing order's invoice identity or issued PDF.

## Delivery and rollout verification

Captured Razorpay payment remains the fulfillment gate. Seller acceptance,
packing, actual parcel allocation and ready-to-ship confirmation precede courier
booking. Genuine carrier labels require a recorded AWB and provider label URL.
The existing worker, provider reconciliation and callback validation remain
responsible for real shipping states. A browser POST does not prove delivery.

The mandatory `authenticated-marketplace` CI job runs real role handlers against
disposable PostgreSQL/Redis without provider credentials, real messages, payments
or bookings. It checks role isolation, responsive/accessibility behavior, seller
draft/review, independent two-seller packages, invoice downloads and recorded
partial refunds. Provider adapters, duplicate callbacks, capture gates, uncertain
booking recovery and refund/settlement conditions retain backend coverage.

Local simulations are not a live carrier pilot. Production activation requires
the owner-controlled customer account, intended price, verified assortment,
funded shipping wallet and a genuine shipment. The delivered-pilot and verified
assortment gates must not be satisfied by fabricated provider events or business
data.

Migrations 72–75 are additive. Take a current recovery point before deployment;
roll back application code using a reviewed compatible release if needed. Keep
the new audit, submission and order snapshot records. Do not drop financial or
historical evidence as a rollback shortcut.
