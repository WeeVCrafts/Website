# Local tool integration verification

Verified on 9 October 2026 against the local Docker Compose stack.

- Adminer `6.1.1-standalone` and Uptime Kuma `2.5.5-slim` are running.
- Web bindings are `127.0.0.1:8082` and `127.0.0.1:3001`; each returned HTTP 200.
- Browser sign-in to Adminer showed PostgreSQL's `public` schema and `users` table.
- Browser sign-in to Kuma showed the configured dashboard.
- All nine configured monitors had Up heartbeats.
- A temporary monitor detected a missing HTTP path as Down, then the app's live
  health URL as Up. The temporary monitor was removed afterward.
- Re-running provisioning retained all nine monitors without duplicate IDs.
- Recreating the Kuma container and re-running setup retained the same account,
  monitor IDs and heartbeat history; the nine monitors remained Up.
- Compose configuration, both Node script syntax checks, Bash startup syntax and
  `git diff --check` passed. The entire Windows/Linux one-click build/seed sequence
  was not run because the existing application/data was already running; its
  constituent Docker startup and setup commands were exercised directly.

Evidence: [browser login](browser-verification.log),
[monitor transitions](monitor-verification.log),
[idempotent setup](provision-rerun.log),
[persistence](persistence-verification.log), and
[dashboard screenshot](screenshots/uptime-kuma-dashboard-1440.png).

The generated password was exported to `.env.kuma.local`, which is Git-ignored.
No credential appears in these evidence files. External alert delivery is not
configured. The same-host checks do not establish external availability when the
entire Docker host is down.
