# Account updates verification — 12 October 2026

The changes preserve the four roles, credentials, catalogue and order history.

- Full Go tests passed with PostgreSQL and Redis using `go test ./... -p=1`. An earlier parallel run exhausted the inventory concurrency test's deadline; that test and the complete serial suite passed.
- Template generation, CSS build and strict premium UI audit passed (zero findings).
- Storefront header checks passed in Chromium, Firefox and WebKit across nine widths from 320px to 1440px, for guests and customers, including keyboard focus, drawers, reduced motion and sticky geometry.
- The isolated account workflow check exercised all four sidebars at 1440px, 390px and 320px, automatic local payment capture, confirmation refresh, failed capture followed by payment retry, order-owner authorization, Super Admin replies to Seller, Seller replies to Admin, notification ownership, CSRF rejection and customer isolation.
- The confirmation email reached Mailpit with the saved order number and an authenticated delivery tracking URL. QA used a cloned database, separate Redis and SMTP; it did not send to production customers.
- PostgreSQL integration checks covered transactional support notifications, private order coordination, duplicate protection and unchanged order state.

Production sender verification used Resend MCP. Railway CLI configured the existing verified sender and securely staged the sending-only key. A PostgreSQL backup was made on the production persistent volume before migration.

Automatic payment applies only to the configured local simulator and says that no money was charged. Live payment capture remains dependent on verified gateway results. Delivery events continue to reflect seller-entered information; order creation does not fabricate shipment progress.
