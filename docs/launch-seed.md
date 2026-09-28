# Launch catalog and role accounts

`cmd/seed-launch` creates ten photo-matched catalog listings and provisions six QA accounts. It is safe to rerun for the same account emails and product slugs. Passwords are read from environment variables at runtime; they are not in Git history. The command changes credentials for the six listed emails, so run it only when credential rotation is intended.

| Username | Login email | Role | Password variable |
| --- | --- | --- | --- |
| customer | customer@weevcrafts.test | Customer | `WECRATFS_CUSTOMER_PASSWORD` |
| seller | seller@weevcrafts.test | Seller owner | `WECRATFS_SELLER_PASSWORD` |
| superadmin | superadmin@weevcrafts.test | Super admin | `WECRATFS_SUPERADMIN_PASSWORD` |
| support | support@weevcrafts.test | Support agent | `WECRATFS_SUPPORT_PASSWORD` |
| operations | operations@weevcrafts.test | Returns operations | `WECRATFS_OPERATIONS_PASSWORD` |
| finance | finance@weevcrafts.test | Finance operator | `WECRATFS_FINANCE_PASSWORD` |

Set `DATABASE_URL`, all six password variables, and `WECRATFS_SEED_LAUNCH=true`, then run `go run ./cmd/seed-launch`. The command does not print passwords. The seller account owns the seeded products.

The ten listings use images already in `web/assets/images/customer`. Names and descriptions identify only what those images show. Prices are starter catalog values and require merchant approval. New stock defaults to zero; set `WECRATFS_SEED_VERIFIED_STOCK` only after confirming the physical quantities. Repeated runs preserve existing stock counts. Supplier identity, item specifications, image rights, and fulfillment terms need confirmation before treating these as live merchandise.
