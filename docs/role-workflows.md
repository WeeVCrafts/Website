# Role workflows

Each role opens its own workspace after sign-in. The workspace has responsive navigation: a fixed sidebar on desktop and an accessible drawer on narrow screens.

| Role | Login username | Workspace | Main work |
| --- | --- | --- | --- |
| Customer | `customer` | `/account` | Browse products, manage addresses and cart, checkout, track orders, returns, reviews, privacy settings. |
| Seller owner | `seller` | `/seller-admin` | Build listings, upload product media, manage stock alerts, process orders, record tracking, review earnings and analytics, manage seller team. |
| Seller staff | `sellerstaff` | `/seller-admin` | Work only within assigned seller permissions: catalog, inventory, orders, fulfilment, returns, and reports. |
| Marketplace admin | `marketplaceadmin` | `/marketplace-admin` | Review seller and product queues, manage categories and brands, oversee orders, returns, support, promotions, and marketplace reporting. |
| Super admin | `superadmin` | `/super-admin` | Manage roles, security, audit records, platform settings, and all marketplace administration. |
| Support agent | `support` | `/support-portal` | Triage customer cases, link orders, record support notes, and update case status. |
| Finance operator | `finance` | `/finance-portal` | Review payments, refunds, commission records, settlements, and finance reconciliation. |
| Returns operations | `operations` | `/operations-portal` | Process return, refund, dispute, and order-operations queues. |

Workflow rules stay server-enforced. Pages show only actions allowed by role and seller assignment. Portal pages use responsive grids, horizontal scroll only for dense operational tables, and a focus-managed mobile navigation drawer.
