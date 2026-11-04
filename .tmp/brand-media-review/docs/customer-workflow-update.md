# Customer workflow update

The storefront keeps the existing Go, templ, JavaScript and shared forest/cream design. The installed frontend-design-premium workflow, frontend-design prerequisite and Emil's motion guidance informed the changes; DESIGN.md and UX-CONTRACT.md describe the shared rules.

## Customer experience

- Account navigation collapses into a keyboard-accessible menu at mobile widths. Dashboard content begins about 412px down at 390px width, compared with roughly 1,072px before.
- Wishlist and Cart are visible in the header with live counts and 44px touch targets. The shopping bag's close button is visible again.
- Checkout has Back buttons and Review edit links. Address fields and payment choices survive corrections. Counts distinguish items from makers, and summaries include the selected product option.
- Catalogue, no-match and empty-cart states explain the actual condition. Empty recommendation sections and unnecessary pagination are hidden.
- Busy buttons show a stable-size spinner and prevent repeat submission. Checkout transitions preserve keyboard focus and respect reduced motion. Failed mock payment requests recover to an enabled retry action.
- Pending payments resume the existing order. Failed payments have a clear status. Local seller controls simulate preparation, shipment and delivery through the existing fulfillment permissions.
- Order detail and tracking show progress separately for each maker, with courier details and recorded update times. `/orders/{number}/tracking` is a working authenticated destination.
- Security provides current-password verification, password confirmation and Show/Hide controls. A successful change revokes earlier sessions and unused password-recovery tokens. Device descriptions use readable browser/platform names.
- Delivered purchases support verified reviews awaiting moderation, return requests and support tickets linked to an order.

## Notifications and local integrations

Order receipt and payment confirmation are separate messages. Successful capture writes one durable `OrderPaid` event in the payment transaction. The worker creates an idempotent in-app notice and sends confirmation with an authenticated tracking link, respecting order email preferences. Existing shipment emails also link to tracking. Email retries are capped; SMTP delivery remains at least once, so a crash after sending can cause a duplicate email.

Local emails appear in [Mailpit](http://localhost:8025). Local payments and courier updates are simulations. Production needs configured SMTP, a correct `WECRATFS_PUBLIC_URL`, live payment credentials/webhooks and an actual courier integration for automatic carrier scans. Tracking currently reflects fulfillment updates recorded by the seller.

## Verification

Verified on 11 October 2026:

- All six customer workflow Playwright scenarios passed (2.9 minutes), including real local SMTP delivery and authenticated tracking URLs.
- `go test -p 1 ./...` passed with PostgreSQL integration tests against the disposable database. Tests requiring additional, unset service variables retain their existing skip behavior.
- Header regression checks passed in Chromium, Firefox and WebKit for guest/customer sessions across nine viewport widths.
- JavaScript syntax checks, CSS build, Docker image build, templ freshness and sqlc regeneration stability checks passed. The final templ refresh changed only diagnostic source paths; rendered output matches the tested version.
- The strict premium design audit and DESIGN.md lint have zero errors or warnings. Mobile account, checkout, tracking and security screenshots were inspected for wrapping and horizontal overflow.
- The main app responds successfully at `/health/ready` and retains exactly three users, zero products and zero orders.

The regression suite is `tests/e2e/customer-workflow.spec.js`. It only permits the disposable database `weev_customer_workflow_20261011` and application port `18081`; the main database is never populated with fixtures. It covers empty states, mobile navigation, keyboard/reduced motion, checkout corrections, payment loading/failure/retry, actual local SMTP messages and tracking links, seller delivery progression, reviews, returns, support, password validation, CSRF and session invalidation.

Run the suite against a fresh copy of the clean local database and a separate Redis instance:

```powershell
$env:WEEVCRAFTS_BASE_URL='http://127.0.0.1:18081'
$env:WEEVCRAFTS_QA_DATABASE='weev_customer_workflow_20261011'
npx playwright test tests/e2e/customer-workflow.spec.js
```

Use `weevcrafts-app` with local mode/payment enabled, the disposable database, a dedicated Redis container, SMTP `mailpit:1025`, and public URL `http://127.0.0.1:18081`. The final password test changes only the disposable customer's password, so repeat runs require a fresh database copy.

Generated SQL uses sqlc 1.30.0. Catalogue image selection explicitly returns non-null text so regeneration matches its Go string contract. Generated templ and CSS assets are included.
