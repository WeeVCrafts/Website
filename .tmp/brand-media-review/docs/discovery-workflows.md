# Catalog discovery and product entry

Customer suggestions, result counts, facets and listings use PostgreSQL. Optional Meilisearch configuration does not change customer results. Migration 0051 installs pg_trgm and maintains weighted search documents and indexed terms transactionally. Deployment database roles must be permitted to install pg_trgm. Existing products require no tags.

Tags are normalized to lowercase with collapsed whitespace and duplicates removed. The application accepts up to 20 tags, each at most 40 characters. Exact active SKU and product-name matches precede weighted text matches. Prefix and English typo matching require all query terms; typo matching applies only to alphabetic terms of four or more characters. Explicit price and newest sorting remain available.

`/featured-products` orders eligible Super Admin pins by existing rank, then paid-unit popularity over the previous 30 days. Popularity refreshes lazily every 15 minutes. Captured payment attempts count each purchase once; cancelled orders and fully processed refunds are excluded. Current refunds apply to whole orders. Approval, seller activity and stock eligibility are checked live. The homepage uses the same sequence, limited to six products. Existing ranks remain stored; pins default off.

Seller product entry shows basic fields together. Optional identifiers, delivery details, pricing details and variants use Advanced disclosures. Blank SKU and slug values receive random identifiers; updates preserve existing identifiers. Brand records are reused. Draft saving and review submission remain separate. Super Admin stock adjustments use the existing atomic ledger and preserve reservations.

Receipts use saved order item names, SKUs, unit prices and line totals. Owner-only PDF access sends private, no-store caching headers. Receipts are not GST tax invoices. Browser payment callbacks never substitute for verified capture.

## Isolated verification

`discovery_integration_test.go` uses DATABASE_URL and creates synthetic catalog, order and refund records. Run only against a disposable database. `discovery-workflows.spec.js` is gated by WEEVCRAFTS_DISCOVERY_QA=true and targets loopback port 18083 with the explicitly named disposable database/container. Its Razorpay protocol fixture runs on port 18088 and uses dummy credentials. It verifies signature handling and replay behavior without external charges; it does not verify the real Razorpay sandbox service. Existing razorpay-testmode.spec.js remains the provider sandbox test.

No production deployment or live payment is part of this change.
