# Railway release procedure

The Website service deploys `main` from `WeeVCrafts/Website`. Its Dockerfile cache mounts require IDs scoped to that service. A build failure before compilation was traced to mounts without those IDs and corrected.

Production database connections require `sslmode=verify-full`. Set the managed Postgres public root CA in `DATABASE_CA_CERT`; do not copy private certificate keys. The app configures pgx trust while preserving hostname verification. Set the canonical app URL explicitly when Railway advertises a custom domain whose DNS points to another site.

## Release checks

1. Export a custom-format database backup and restore it to an isolated PostgreSQL instance matching the server major version.
2. Review migration preflight failures. The owner approved converting the five legacy Seller Staff accounts to Customer and revoking team memberships. That preparation is transactional and audited; no accounts, products or orders are deleted.
3. Rehearse migrations and verify preserved record counts. Run isolated integration tests, template and CSS builds, and browser checks.
4. Configure connection variables with `--skip-deploys`, then push the tested commit. Do not apply an entire IaC plan merely to change a connection variable. Review unrelated deletions and preserve existing credentials.
5. Follow the deployment associated with the exact pushed Git SHA through Railway build, startup migrations and readiness. Check both public health endpoints and authenticated portal access before declaring success.
6. Verify the four canonical roles and the preserved account, product and order IDs after rollout. Keep the database backup outside the repository.

## Designated accounts

The operator-only `weevcrafts-sync-accounts` command accepts exactly four account records as private JSON on stdin after `WECRATFS_SYNC_DESIGNATED_ACCOUNTS=true` is set. Records contain `username`, `email`, `role`, `password` and optionally `existing_email`. Existing active accounts must already have the requested canonical role. A supplied previous email must resolve to an existing account. Seller ownership and profile approval remain intact. This command is never run at application startup.

Run it after migrations on local and Railway databases to apply the owner-requested credentials. Existing identities are updated in place; a missing local Admin can be created. Passwords use independent Argon2id salts; prior sessions are revoked and each update is audited without recording credentials. Do not put private JSON, password hashes, variable exports or database dumps into Git.

Username and email sign-in use the same identity, lockout counters and role routing. Registration always creates Customer server-side. Only Super Admin can replace assignments through the application, with confirmation and an audit reason.

The current payment configuration is simulated. Live payments, courier integration, payout/refund reconciliation and privileged MFA require separate release verification.
