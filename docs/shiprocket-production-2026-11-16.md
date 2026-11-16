# Shiprocket production activation — 16 November 2026

## Running release and recovery

The existing production application was restarted through Coolify on `main`,
pinned to `e6fdb0f354f8b3ca614afa32854a60ae4be196ca`. The image is unchanged:
`sha256:166827c295d66723a6258a1b72298fe9d56f8de5352a326ffe0498f806c1019d`.
PostgreSQL remains at migration 59. The larger fulfillment candidate in this
checkout has not been promoted by these configuration rollouts.

Before changing runtime configuration, an age-encrypted PostgreSQL custom archive
and encrypted container/environment snapshot were created. The archive decrypts
and `pg_restore --list` reports 715 entries. The encrypted dump is 357,044 bytes.
A recovery image tag was retained. The restricted recovery directory, encrypted
archives and recovery identity were copied to the owner's private local storage,
outside this repository. No database migration or object-storage change occurred.

## Verified production configuration

- Live Shiprocket production authentication succeeds from the application server.
- The dedicated API user has Orders, Settings, Shipments and Courier access;
  customer-data access is restricted to the application server IP.
- Credentials are encrypted in Coolify, never browser application configuration.
- `WECRATFS_SHIPROCKET_MODE=live` is active.
- `WECRATFS_SHIPROCKET_BOOKINGS_ENABLED=false` remains active during readiness.
- `WECRATFS_SMS_ENABLED=false` remains active; MSG91 is deferred by the owner.
- Homepage and `/health/ready` return HTTP 200.
- `/webhooks/delivery` exists: GET returns 405, an unauthenticated POST returns
  401, and a correctly authenticated connectivity probe returns 200.

The probe uses a deliberately unmatched reference and never applies a delivery
status or creates a business order. The application continues to obtain actual
shipment state from authenticated Shiprocket reads.

## Registered provider webhook

The owner-authorized in-app Browser session registered
`https://weevcrafts.com/webhooks/delivery` using `x-api-key` authentication.
Shiprocket displayed **Endpoint saved successfully**, followed by its successful
test notice: **Tracking information will be sent to the url given below**.
The saved proof screenshot excludes the private token field.

Earlier HTTP 404/save failures were caused by the inactive shipping callback on
the previous runtime configuration. The configuration rollouts resolved that
boundary; they do not establish a completed courier delivery.

## Seller pickup mappings

| Seller | Registered nickname | PIN | Verification |
| --- | --- | --- | --- |
| 3 — WeevCrafts | `Primary` | 500084 | Verified in the provider API (`phone_verified=1`) and dashboard. Contact name is WeevCrafts, using the owner-supplied shared courier phone. |
| 2 — Kamala | `Kamala500079` | 500079 | Created through the production pickup API, ID 151546829. Owner completed OTP; API (`phone_verified=1`) and refreshed dashboard both confirm verified. |

Kamala's address and contact came from the owner and are retained privately in
Shiprocket. No guessed street, telephone number, coordinates or seller tax data
was substituted. The platform's existing business inbox receives pickup notices.
Both nickname/PIN mappings are saved in Coolify. Seller 3's existing pickup
contact was updated through Shiprocket using the owner's supplied name and
phone, plus the platform's existing business inbox for required pickup notices.
Shiprocket acknowledged the save and recognized the already verified phone;
no second OTP was required. Fresh API reads confirm both expected contact
names, phone matches and PINs. Both dashboard rows show VERIFIED.

## Remaining genuine fulfillment evidence

Production contains ten variants with zero shipping weight/dimensions; eight are
currently orderable. An exported input CSV lists those eight existing variants
with blank measurement fields. The owner/seller must supply actual measurements.
No default weight was inserted to bypass readiness.

The Shiprocket wallet remains ₹0. Funding is an owner action. The only captured
production order is already delivered; no unshipped captured order is available
for a genuine booking. Do not rebook that completed order as an integration test.

After pickup verification and parcel data are complete, verify serviceability
and the actual quote, fund the wallet, and select a genuine captured prepaid
order. Generate its AWB/label, schedule pickup, retain actual handover evidence,
and verify tracking through physical delivery. Keep booking paused until these
prerequisites are satisfied. No AWB, shipment pickup or delivery is claimed here.

The extended fulfillment release also requires verified seller invoice/legal
and product tax profiles before its new-order checkout is promoted.

## Candidate fixes and checks

The checkout candidate now checks all shipping dimensions as well as weight in
the Shiprocket readiness command. NDR parsing recognizes explicit boolean and
documented string acknowledgements; null, missing and unknown outcomes remain
uncertain. Those changes remain local to the next application candidate.

Targeted Go tests passed: 51 tests across Shiprocket, webhook and application
entry-point packages. Targeted `go vet` and `git diff --check` passed. Earlier
marketplace/database/browser checks are recorded in the fulfillment notes.
