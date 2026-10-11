# Four-role operations

This contract supersedes the role descriptions in dated audit reports. Historical audit evidence and message authorship remain unchanged.

| Account role | Destination | Responsibility |
|---|---|---|
| `customer` | `/products` | Shopping, account, orders, reviews, returns and customer support. Public registration always assigns this role. |
| `seller` | `/seller-admin` | One login per independent business, owning its catalogue, media, inventory, fulfilment, recorded tracking, earnings and support. Profile approval and suspension still apply. |
| `admin` | `/admin` | Company marketplace, support, delivery follow-up, returns and existing finance operations. |
| `super_admin` | `/super-admin` | Company operations plus account creation, all four role assignments, platform security, settings and audit access. |

Accounts have one role. Assignments replace the previous role and invalidate existing sessions. Account creation and role replacement require explicit confirmation and an audit reason. The last active Super Admin cannot be demoted, suspended or deleted. A transaction-wide administration lock serializes these operations and rechecks the acting account after acquiring the lock.

Seller role replacement and deletion lock the seller record and check unfinished orders and unresolved returns. Checkout acquires that same seller lock before reading availability. Eligible demotion pauses the storefront while keeping products, media, earnings and order history. Reassigning Seller does not reactivate a suspended profile.

## Private seller confirmations

Customer case → inspect the linked order → request seller confirmation → seller replies → company records delivery issue/follow-up outcome → customer receives a separate reply → company resolves the case.

Company support selects only sellers represented in the case's order. Each seller receives only its own request, items, tracking and fulfilment history. Requests and replies live in separate private tables; customer messages and internal notes never enter the seller conversation. Attachments from the customer case are not copied into private requests.

Requests move from Open to Answered to Closed. Replies after closure are rejected. Another follow-up creates a new request. Create/reply keys make exact retries idempotent and reject reuse for different payloads. All mutations require authentication, CSRF and server validation; coordination actions are audited. Queue status and pagination use URL parameters (`coordination_status`, `coordination_before`).

Carrier, tracking number and fulfilment history reflect seller-entered records. Company support records a follow-up outcome without manufacturing carrier scans. Closing requests or cases cannot deliver orders, capture payments or process refunds.

## Migration and rollout

Migrations 42–44 map valid legacy assignments, remove retired assignable roles, enforce a single assignment, add private confirmation records and support unique account usernames. Migration 42 aborts with affected user IDs for staff assignments, active team memberships, conflicting assignments or active accounts without roles. Review those identities explicitly; do not give staff broader access automatically. The entire migration rolls back on failure. Historical author roles remain permitted only as message evidence.

Retired company GET bookmarks redirect to the current authorized workspace. Legacy POST aliases dispatch to the same handlers and validation; no POST is redirected into a different mutation. Seller Team screens and endpoints return 404.

Before rollout:

1. Back up PostgreSQL with `pg_dump -Fc` and verify that the dump is readable.
2. Run integration tests against an isolated database (`DATABASE_URL`), plus Redis/S3 integration dependencies where configured. Never run fixture-creating QA against business data.
3. Run Go tests, templ generation, `npm run css:build`, the strict premium audit and relevant Playwright checks. Exercise multi-seller privacy, duplicate retries, keyboard navigation, 390px layout and reduced motion.
4. Deploy migrations and application together after staging verification. The startup migrator applies migrations; keep the previous application stopped while the role model changes. Existing sessions will require sign-in again.
5. Verify preserved account/catalogue/order counts and canonical assignments. Monitor 403/409 responses and support persistence errors. Roll back by restoring the verified backup and matching application version; do not recreate retired role assignments manually.

The owner subsequently requested four designated accounts in local and Railway: Customer, Seller, Admin and Super Admin, with matching usernames and emails at `weevcrafts.com`. Existing designated accounts are renamed in place to preserve IDs and seller ownership; the missing local Admin is created explicitly. Other accounts and business records remain. Passwords are supplied privately to the opt-in `cmd/sync-designated-accounts` tool, hashed with Argon2id and never committed. Credential changes revoke prior sessions. Usernames are unique account identities and remain stable when Super Admin changes a role. Public registration and password changes retain the 12-character policy; the operator tool supports the owner-approved bootstrap credentials independently.

Real courier integrations, live payouts, refund reconciliation and privileged MFA remain separate release work. This consolidation does not establish production readiness.
