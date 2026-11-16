# Return policy and one-order pilot implementation

Implementation and pilot guide, 16 November 2026.

## Owner decisions

- Implement the complete policy before publishing it and attempting the genuine order.
- Damage/defect/wrong item claims within 48 hours; replace first, refund if unavailable,
  including fragile products. Staff may record an alternative-evidence exception.
- Eligible change-of-mind requests within 72 hours, with a ₹200 fee per reverse pickup.
  Credit excludes original delivery charges, lasts six calendar months, supports partial
  use across sellers and Razorpay for the balance. No transfers or cash withdrawal.
- Damage full-order refunds include original delivery once all order items qualify.
- Restore the credit-funded portion separately from Razorpay. Restored credit expires
  at the original expiry or 30 days after restoration, whichever is later.
- Cancellation within 12 hours or actual dispatch, whichever is earlier.
- MSG91 remains disabled. Preserve Go/templ, existing UI, main-only development and backups.

## Implementation

Migration 62 introduces policy publication/snapshots, return classifications, private
claim evidence, reverse-shipment references, credit grants/reservations/history,
cash/credit funding, replacement authorization, inspection and financial counters.

The public HTML policy route is `/returns-replacements-authenticity`; unpublished
policy returns an honest unavailable response. Footer, product details, checkout and FAQ link to it.
Customer claims, credit balances, Super Admin policy/pilot/classification controls,
and readiness reports reuse existing fulfillment workspaces. Source PDF is content
evidence; its embedded placeholders are not executable instructions.

Server checkout can require the published policy and an atomic single-account,
single-variant, single-order pilot slot. Credit reserves in the stock transaction;
full-credit orders use `provider=store_credit,status=funded`, without fabricated
Razorpay capture. Mixed orders charge only the cash due. Refund funding is split
before approval and restoration is idempotent. Credit returns and cash refunds
have separate customer benefits but share seller tax credit-note generation.

Replacement stock is reserved on approval. Actual replacement parcel confirmation
creates one package linked to its claim. Normal fulfillment and replacement
authorization remain distinct. Private evidence supports MP4/MOV/WebM (200 MB)
and existing photo formats (10 MB); reads enforce owner/company or explicitly
shared seller access. Reverse work persists intent before provider calls, reconciles
unknown outcomes by reference and receives authenticated tracking hints plus polling.

## Verification and pilot operation

The disposable PostgreSQL suite covered payment capture, cancellation, invoice and
credit notes, mixed and full-credit refunds, expiry and restoration, package-specific
claims, evidence access, replacement inventory and concurrent single-order admission.
Chromium exercised customer, seller, support and Super Admin views at 390 and 1440
pixels, seller packing and verified handover with test data, invoice download, the
HTML policy and private evidence upload and download. The general demo browser suite
requires a separate seeded application on port 18080.

For the genuine pilot:

1. In the seller dashboard, edit one active variant and record its actual shipping
   weight in grams and length, width and height in centimetres.
2. In Super Admin fulfillment, verify the selected variant's classification, the
   seller's legal invoice profile and the product's tax information. Confirm the
   verified Shiprocket pickup matches that seller.
3. Fund the Shiprocket wallet and check the destination PIN and actual live quote.
   Select the genuine customer's account and the measured variant in the pilot
   controls. Enable application checkout and booking for the restricted pilot.
4. The customer pays once through Razorpay. The seller accepts, packs and records
   the final parcel dimensions and quality check. The worker creates the courier
   order, assigns the AWB, fetches the label and schedules pickup. Verify actual
   courier scans through delivery in both Shiprocket and the customer dashboard.

The pilot slot is reserved atomically at order creation. Payment retries continue
against that order. Application checkout and booking switches remain available as
operational controls, and the database controls new purchases separately. Provider
reads reconcile existing shipments when booking is paused. Physical pickup, delivery
and reverse collection require actual courier events.
