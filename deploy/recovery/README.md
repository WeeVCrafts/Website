# WeevCrafts host recovery

The existing weevcrafts-backup timer runs daily at 02:30 IST. Keep Coolify's
database-only schedule as an additional recovery option. The host recovery
service uses the ops-recovery command to export a PostgreSQL snapshot and
collect the media referenced by that snapshot, including product and storefront
media, private claim evidence, support attachments and settlement documents.

- Build the Linux helper with `CGO_ENABLED=0 go build -trimpath ./cmd/ops-recovery`.
  Keep the existing scheduler script as `weev-backup-base.py` alongside
  `host_runner.py`, which is installed as `weev-backup-run.py`. The Docker
  collector uses the application network, configured PostgreSQL client image,
  and inherited database and storage TLS settings.
- The host config points to the private R2 destination and the Coolify server
  that holds `BACKUP_S3_ACCESS_KEY` and `BACKUP_S3_SECRET_KEY`. Use a
  bucket-scoped Object Read & Write token. Both configured age recipients
  are public keys. Store the private identities away from the VPS.
- The job marks an archive complete after successful collection, encryption,
  and full R2 readback with a matching SHA-256. Use the `daily/` object prefix
  with the 30-day bucket lock. Apply any expiration policy after that lock
  period and preserve known-good recovery points.
- The host monitor compares the public readiness endpoint with the local proxy.
  The notifier appends sanitized detail from `WEEVCRAFTS_ALERT_DETAIL`. Audit
  stale snapshots, alert delivery, and release revisions after maintenance.

For a restore drill, use the recorded archive name and digest with
`ops-recovery download ARCHIVE SHA256`. Decrypt outside the VPS with an
authorized age identity. Restore `database.dump` to isolated PostgreSQL with
live integrations disconnected. Verify each entry under `objects/` against
the manifest inside the encrypted archive. To repopulate the recovery object
store, use the original keys and content types in that manifest. Check that
sample objects can be retrieved with the intended access controls. Record the
recovery point and elapsed time in the release checklist.

The Production readiness GitHub workflow probes the public endpoint from
outside the VPS every five minutes. Failed workflow notifications use the
repository owner's GitHub notification settings. Scheduled workflow timing
depends on GitHub runner availability.
