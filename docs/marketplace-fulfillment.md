# Marketplace fulfillment

The customer, seller, Admin and Super Admin fulfillment workspaces extend the
existing Go/templ portals. Support remains within Admin. Production uses prepaid
Razorpay orders and live Shiprocket credentials; there is no COD path.

## Workspaces

| User | Entry point | Responsibilities |
| --- | --- | --- |
| Customer | `/account/fulfillment`, `/orders/{number}/fulfillment` | Owned orders, payment/refund state, package contents/tracking, seller invoices/credit notes, cancellation before collection, exceptional return/refund requests. Existing support tickets remain under Account Support. |
| Seller | `/seller-admin/fulfillment` | Accept/reject, packing, package allocation and packing slips, ready-to-ship confirmation, carrier labels/manifests, pickup refresh/rescheduling. Existing inventory, earnings, settlements and support remain in their workspaces. |
| Admin | `/admin/fulfillment` | Delivery/NDR/RTO queues, seller escalations, exception returns, refund approval, verified manual delivery and failed-job recovery. Existing complaints and private seller coordination remain in Support. |
| Super Admin | `/super-admin/fulfillment` | Company operations plus verified pickup mappings, seller invoice profiles and product tax profiles. Existing marketplace overview, directories, commissions, payments, payouts and audit logs remain available. |

The recent-order and exception queues use bounded cursor pages. All reads and
mutations recheck current database roles and ownership. POSTs require the session
CSRF token. Browser success messages do not establish payment, refund or delivery.
Seller role removal remains blocked while fulfillment, exception returns,
refunds or company delivery cases are outstanding; existing catalog and order
records remain preserved.

## States, policy and compatibility

Migration 60 adds independent master-order and seller-order lifecycle columns,
independent package records and quantity allocations, exception return/refund
records, invoice snapshots/PDFs and durable jobs. Existing financial and manual
settlement control states remain compatible. Payment refund state is derived
from the captured amount and the independently recorded refunded amount; partial
refunds do not block fulfillment of unaffected items.

New orders use workflow version 2. Historical orders retain the older fulfillment
path, original policy snapshots and payment receipts. No historical tax profile,
invoice or delivery evidence is fabricated. Seller ownership is snapshotted on
order items; package allocations cannot exceed uncancelled quantities.

For new handmade purchases, standard returns are disabled explicitly. Zero
`return_days` continues to mean the historical eligibility behavior and is not
used as a no-returns flag. Damaged, wrong and disputed deliveries can be reviewed
by company staff. Exception returns follow Requested → Approved → Pickup pending
→ Received → Inspected → Completed, with rejection where allowed. A physical
return requires actual carrier details; received goods are restocked only after
an operator records suitable inspected condition. Completion requests a refund;
it does not approve or submit one automatically.

Paid item cancellation locks allocations against booking. Existing parcels must
be confirmed cancelled before inventory is released. Remaining items return to
Packing so the seller can confirm the revised package contents/dimensions.
After collection, complaints go to company investigation. Refunds use the actual
paid item amount after discounts, including its tax; delivery fees are refunded
only when the entire order is cancelled before any dispatch. Both company roles
can approve refunds. A provider-confirmed failed refund can receive an explicit,
audited retry approval with a fresh attempt receipt. Uncertain submissions only
reconcile. Repeated callbacks cannot duplicate ledger reversals or
restock goods twice.

Sellers receive an acceptance reminder after 24 hours and an Admin escalation
after 48 hours. There is no automatic seller-SLA cancellation. Reconciled earnings
become eligible seven days after confirmed delivery; unresolved complaints,
returns, refunds, delivery exceptions and payment reviews extend the hold.
Batch creation, approval, transfer confirmation and the database payout trigger
enforce the hold. Previously paid liabilities can be offset by verified refund
debits; an unpaid cancelled sale must not create an unrelated seller deduction.
Bank transfers still happen externally, with the existing independent operators,
recipient verification and protected transfer evidence.

## Provider calls and workers

Seller confirmation queues shipment creation, serviceable courier selection,
AWB, label, pickup and manifest work. The provider's recommended courier is
preferred; otherwise the lowest available quoted rate wins, with courier ID
breaking ties. Definitively rejected couriers are excluded on subsequent attempts.
An ambiguous response is reconciled by the saved merchant reference/provider IDs
before another business POST. Unknown carrier states create support cases.

Authenticated delivery callbacks at `/webhooks/delivery` are hints. Status changes
require an authenticated provider read; periodic synchronization provides a
fallback. NDR reattempt/RTO requests are durable, company-authorized operations.
Uncertain NDR submissions are investigated rather than blindly submitted again.
RTO stock/refund decisions require verified return-to-origin delivery and a
recorded inspection.

If courier serviceability times out at checkout, the saved manual-fallback
decision permits an operator-assisted delivery. A confirmed unserviceable route
still blocks checkout. Manual handover requires real carrier/tracking details;
company staff verify delivery evidence before marking that parcel delivered.

Business records, durable jobs and outbox events commit together in PostgreSQL.
Redis Streams transport job IDs to two consumers. PostgreSQL claims, expired
leases, retry backoff and operator-visible failed jobs preserve work across
Redis loss/restarts. Payment/refund reconciliation, confirmations, dispatch
notifications, indexing, reservation expiry and legacy shipment synchronization
use this transport. Existing record-level idempotency remains necessary because
delivery is at least once. Metrics/maintenance observations retain their bounded
periodic scheduling.

## Invoices and live activation

Super Admin verifies each seller's legal name, address, registration, state,
invoice prefix and each variant's HSN/tax rate. Catalog prices are tax-inclusive.
The platform issues separate invoices on the seller's behalf using immutable
snapshots, transactional numbering and stored private PDF bytes/checksums.
Processed refunds create separate credit notes. Receipts remain separate.
Production checkout refuses unverified seller/product invoice configuration
before reserving stock or creating an order. Approved legal/tax classifications
must be supplied; do not invent merchant facts or tax rates.

Live activation requires private Coolify settings:

```text
WECRATFS_SHIPROCKET_MODE=live
WECRATFS_SHIPROCKET_BOOKINGS_ENABLED=true
SHIPROCKET_API_EMAIL=<dedicated API user>
SHIPROCKET_API_PASSWORD=<private password>
SHIPROCKET_WEBHOOK_SECRET=<private token of at least 32 characters>
SHIPROCKET_PICKUP_LOCATIONS=<seller ID to registered pickup name JSON>
SHIPROCKET_PARCEL_PROFILES=<verified pickup PIN and packing allowances JSON>
```

Run `check-shiprocket` against the deployed candidate. Additional pickup mappings
can be verified against Shiprocket from Super Admin; checkout reads verified
live mappings from PostgreSQL. Pausing new bookings preserves reconciliation.

At the initial audit, live Razorpay credentials were configured on the running
Coolify service, but Shiprocket settings were absent from both saved and running
configuration. Requests to the public homepage/readiness endpoint from this
environment returned HTTP 403. These observations require a fresh launch check;
they do not establish current credentials, public access or provider readiness.

Integration setup on 15–16 November 2026 created a dedicated Shiprocket API user
with Orders, Settings, Shipments and Courier access, excluding Listings. PII
access is restricted to the application server IP. Production authentication and
the pickup-list API succeeded from that server. Credentials are encrypted in
Coolify and kept outside source; no shipment, label or pickup was created.

The owner confirmed seller 3 uses PIN 500084 and seller 2 uses PIN 500079. The
account currently exposes only `Primary` at 500084. Seller 3's mapping is staged;
seller 2 still needs an actual registered pickup address/name and contact details.
The PIN alone is insufficient to register a courier collection address. The
Shiprocket dashboard showed KYC complete and a zero shipping-wallet balance.

On 16 November the owner requested full production activation. Coolify restarted
the existing `main` image with live provider mode and bookings paused, following
an encrypted recovery backup. The public callback is now active and authenticated;
Shiprocket saved the webhook and its endpoint test passed. Kamala's owner-supplied
pickup was registered as `Kamala500079`. Pickup verification, real parcel data,
wallet funding and an unshipped captured order remain prerequisites for booking.
See the [production activation record](shiprocket-production-2026-11-16.md) for
the exact image, evidence and remaining physical fulfillment steps. Migration 60
and the extended fulfillment candidate have not yet been deployed.

Transactional SMS is implemented separately; see [MSG91 setup](msg91-integration.md).

Before activation, publish the matching approved return/refund policy, verify
seller/legal/product/pickup data, take the recovery backup, and record the exact
main commit, image digest and migration level. Complete a controlled genuine
prepaid order, real label/pickup, tracking and delivery verification, and separate
refund evidence. Automated fixtures do not establish live provider success.

Deploy migration 60 and the compatible application together, stopping the older
writer during the change. Ledger uniqueness rules now support multiple item
refunds. A binary-only rollback to an older writer is unsupported: keep checkout
and finance closed until a compatible version is restored, or restore the
verified database backup with its matching application. Retain provider
references, refund attempts, transfer proof and pending job claims.

## Verification

Focused domain/adapter/application tests cover lifecycle transitions and refund
rounding. PostgreSQL integration tests cover unpaid denial, split packages,
over-allocation, lost creation responses, customer ownership, item cancellation,
refund approval/idempotency, stock restoration, invoice idempotency, exception
returns, delivery holds and missing-invoice checkout rejection. Legacy payment,
return and shipping fixtures explicitly exercise historical behavior.

`tests/e2e/marketplace.config.js` uses opt-in fixtures and an authenticated
localhost preview. It checks all four role surfaces at desktop/390px, keyboard
access, seller fulfillment, manual-fallback delivery, private PDF downloads and
role denial. The opt-in preview has no checkout or provider worker. Its fixture
helpers refuse business databases and use dedicated disposable PostgreSQL/Redis.
The existing manual-finance integration suite uses its separately guarded
database and schema isolation.
