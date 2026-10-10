# Resend integration — 10 October 2026

Resend is the optional server-side transactional sender. Firebase Admin creates private verification and reset links; the Go application renders `internal/adapters/email/templates/action.html` as branded HTML with a text alternative. Firebase's blocked template editor is not required. Its default hosted action page applies the one-time link and returns to `https://weevcrafts.com/`. That domain still serves Shopify; no production website cutover was performed.

The user-approved Cloudflare Domain Connect action added only these three records for the existing Resend domain `mail.weevcrafts.com`:

| Type | Name | Destination |
| --- | --- | --- |
| TXT | `resend._domainkey.mail` | Public DKIM value supplied by Resend |
| CNAME | `rsend.mail` | `rsend-apne1.forge.rmta.net` |
| CNAME | `send.mail` | `send.forge.rmta.net` |

Both CNAME records are DNS only. Cloudflare's one-time authorization grants no ongoing DNS access. Resend verified the domain, DKIM and sending records. Receiving remains disabled and existing website/mailbox records were not changed. Domain click/open tracking is unconfigured; TLS is enforced, so receiving servers without TLS cannot receive these emails.

The local sender is `Weevcrafts <no-reply@mail.weevcrafts.com>` with reply-to `weevcrafts@gmail.com`. The sending-only API key is outside the checkout at `%LOCALAPPDATA%/Weevcrafts/resend-api-key`, with restricted Windows ACLs. A copy is owned by the non-root app user, mode 0400, inside the read-only external secret volume. The ignored `.env` selects `WECRATFS_RESEND_API_KEY_FILE=/run/secrets/resend-api-key` and the sender/reply address. No private key is embedded in source, images, browser configuration, logs or screenshots. Production must use an independently provisioned managed secret and HTTPS public URL.

Configuration fields: `WECRATFS_RESEND_API_KEY_FILE` (preferred), `WECRATFS_RESEND_API_KEY` (server environment alternative), `WECRATFS_RESEND_FROM` and optional `WECRATFS_RESEND_REPLY_TO`. With neither key setting, the existing SMTP adapter remains active. A configured but unreadable key or invalid sender fails startup; a delivery failure does not silently downgrade to a different sender.

Verification requires same-origin double-submit CSRF, a fresh server-verified Firebase token and an active single customer role when the email already exists locally. Client-supplied recipient fields are rejected. Forgot-password selects bound Firebase ownership server-side and retains native recovery for native accounts. Valid public requests return the same accepted response even during delivery failures; acceptance is not a delivery guarantee. Authentication rate limits include both Firebase session and verification endpoints. Provider bodies containing personal information or action links are never returned or logged; HTTP redirects carrying the bearer credential are rejected.

The Codex Resend MCP server is installed at `https://mcp.resend.com/mcp` and OAuth completed with the user's approved Full access. Its tools were not exposed to this already-open task session, so the requested `Hello from MCP` message was sent through the authorized Resend HTTP API. The dashboard confirmed it delivered to `weevcrafts@gmail.com` from `onboarding@resend.dev`; idempotency protected that send.

Focused checks cover template escaping, unsafe headers/URLs, provider-error redaction, customer ownership, role rejection, unknown identities, native recovery fallback, delivery failures, CSRF, forbidden client recipient fields and recovery non-enumeration. Opt-in live Firebase fixtures verify link generation/consumption, email verification persistence, password reset/new-password sign-in, old-password rejection and one-time reset behavior. Disposable remote identities are removed. A separately opted-in run sent the actual branded verification and reset templates to the selected test inbox; those fixture links were consumed and cannot be used as customer links.

Actual human Google popup completion and human registration/reset through the UI remain separate from automated provider checks. The browser policy requires the user to enter and submit new passwords. Do not report those human steps as passed until observed.

The dashboard confirmed all three test messages delivered: Hello from MCP, Verify your Weevcrafts email and Reset your Weevcrafts password. This is receiving-mail-server confirmation, not proof the user opened the inbox or completed the UI flow.

Evidence: `docs/qa/resend/domain-verified.png`, `docs/qa/resend/secure-delivery.png`, `docs/qa/resend/email-delivery.png`. Check delivery statuses in the Resend dashboard rather than inferring inbox receipt from an HTTP success. Mailpit remains available locally but messages use Resend while its key is configured.

References: [Firebase Admin email action links](https://firebase.google.com/docs/auth/admin/email-action-links), [Resend domain verification](https://resend.com/docs/dashboard/domains/introduction).
