# Deployment security requirements

This local update implements application safeguards. It does not deploy or certify production infrastructure. `compose.yaml` is a development stack with loopback ports and development credentials; do not use it as a production manifest.

## Required configuration

Set `WECRATFS_ENV=production`, `WECRATFS_PUBLIC_URL=https://your-store-domain`, and `WECRATFS_SECURE_COOKIES=true`. Terminate TLS at a controlled ingress, redirect HTTP to HTTPS, and configure HSTS there after verifying all required subdomains support HTTPS. Restrict direct access to the application port and health/metrics endpoints.

The application fails startup in production if the public URL/cookies are insecure, the database uses missing/development credentials, `DATABASE_URL` does not include `sslmode=verify-full`, object storage uses missing/development credentials, or `S3_SECURE` is false. Configure the trusted database CA as required by your provider. Local/demo feature flags are forcibly disabled in production and local payment helper endpoints are not registered. Use the real payment provider configuration for production; never seed demo users there.

Use separate, least-privilege credentials for the database, private object bucket, Redis, search and email. Keep secrets in the deployment secret store, rotate them, and restrict who can read them. Do not copy `.env`, `.env.kuma.local`, development passwords, customer uploads or session material into source control or build logs. Restrict Redis and search to private infrastructure; use authentication and encrypted transport supported by their adapters/providers. The search index must contain public catalog data only.

Keep Adminer, Uptime Kuma, Mailpit and the Garage admin API off the public internet. Local ports bind to `127.0.0.1`; remote administration needs a private network or authenticated tunnel. Adminer additionally requires database login; Kuma has its own persisted administrator login. Neither tool inherits marketplace roles. Disable public bucket access and anonymous listing; avatars are served by the application only after owner authentication.

## Data and recovery

Enable provider-managed encryption at rest for PostgreSQL, object storage and backup storage. Use encrypted backups with separate access permissions and key management; include PostgreSQL, the private bucket and the `product_uploads` volume. Redis sessions and the search index are disposable and must not be restored as authoritative customer records. Rebuild the public search index after restoring PostgreSQL; invalidate sessions after a recovery.

Before launch, perform a restore into an isolated environment: restore a database snapshot and the matching files, run migrations, verify representative order/payment totals and media, verify account ownership and role access with synthetic users, and record restore duration and recovery point. Repeat on a schedule and after backup or schema changes. Monitor backup failures and the avatar cleanup backlog; storage outages leave queued deletion work for retry.

Define retention periods for orders, tax/payment records, support correspondence, audits and backups with the responsible business owner. Account deletion clears profile phone and avatar metadata and queues the private photo for deletion, while historical commerce records remain available under their existing retention rules. Expired backups must be removed according to the retention policy; restored data must not reactivate previously deleted accounts. An access/privacy request is recorded by the existing privacy workflow; there is no new bulk personal-data export endpoint in this update.

## Operational checks

Review operator access, production cookie flags, CSRF denials, authentication throttling and session revocation after role changes, password replacement, suspension and deletion. Keep request bodies, tokens, passwords, uploaded bytes and unnecessary personal details out of telemetry. Audit entries describe the actor, operation, affected record and administrative reason. Restrict audit access and instruct operators not to enter sensitive data into free-text reasons.

Maintain supported dependencies and scan built images before release. Application and integration tests provide regression evidence for tested paths; they do not replace an independent penetration test or verification of the deployed ingress, storage permissions, encryption and backup recovery.
