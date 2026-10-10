# Role workflows

Each account has one assigned role and opens its portal after sign-in. Customer sign-in opens the product listing. Operational portals have responsive navigation: a fixed sidebar on desktop and an accessible drawer on narrow screens. There is no workspace chooser or switch control.

| Role | Login username | Workspace | Main work |
| --- | --- | --- | --- |
| Customer | `customer` | `/products` | Browse products, manage profile/photos, addresses and cart, checkout, track orders, returns, reviews, privacy settings. |
| Seller owner | `seller` | `/seller-admin` | Build listings, upload product media, manage stock alerts, process orders, record tracking, review earnings and analytics, manage seller team. |
| Seller staff | `sellerstaff` | `/seller-admin` | Work only within assigned seller permissions: catalog, inventory, orders, fulfilment, returns, and reports. |
| Marketplace admin | `marketplaceadmin` | `/marketplace-admin` | Review seller and product queues, manage categories and brands, oversee orders, returns, support, promotions, and marketplace reporting. |
| Super admin | `superadmin` | `/super-admin` | Manage roles, security, audit records, platform settings, and all marketplace administration. |
| Support agent | `support` | `/support-portal` | Triage customer cases, link orders, record support notes, and update case status. |
| Finance operator | `finance` | `/finance-portal` | Review payments, refunds, commission records, settlements, and finance reconciliation. |
| Returns operations | `operations` | `/operations-portal` | Process return, refund, dispute, and order-operations queues. |

Workflow rules stay server-enforced. Pages show only actions allowed by role and seller assignment. Portal pages use responsive grids, horizontal scroll only for dense operational tables, and a focus-managed mobile navigation drawer.

Super Admin alone replaces an account's assigned role through Security & Roles. Replacement revokes existing sessions and reconciles seller staff memberships while retaining historical records. Accounts with multiple or missing roles show an access message until Super Admin corrects their assignment; the update does not silently remove old roles. Legacy `/workspaces` requests redirect to the assigned portal.

## Manual workflow testing

Use [the plain-language test cases](manual-test-cases.md) to test every role, guest access, permissions, complete commerce journeys, integrations, and release requirements. Record each role's observed results, evidence, defects, and retests in [the execution tracker](test-execution-tracker.csv). All tracker rows start as Not run; preparing the suite does not establish application readiness.
