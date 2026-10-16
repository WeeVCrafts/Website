# Firebase customer integration — 10 October 2026

The local Go/Docker preview uses Firebase project `weevcrafts-708af` on its existing Spark plan. No billing upgrade, production website deployment, database reset or bulk customer import was performed. The selected public return address is `https://weevcrafts.com/`; that domain currently serves a separate Shopify site, so the Firebase-hosted email action handler remains in use.

## Connected features

| Feature | Implementation and limits |
| --- | --- |
| Google / Gmail | Google sign-in is enabled and the login/register pages expose an explicit consent button. The server verifies Firebase signatures, revocation, provider, email and fresh authentication before issuing its own session. Gmail addresses also work with email/password. |
| Email/password | New customer registrations use Firebase. The project enforces a 12-character minimum password. Browser Firebase authentication uses memory persistence; the application session is a first-party HttpOnly cookie. Existing native operational logins remain supported. |
| Existing customers | Explicitly confirm the existing password after verified Google/Firebase email sign-in. Matching email alone never merges accounts. Linking preserves the customer ID and commerce history, consumes native recovery links and removes the old native password. Existing customer passwords were not exported or imported into Firebase. |
| Password recovery | Firebase Admin generates links for Firebase accounts; the application sends branded HTML through Resend. Native accounts retain native action tokens and use the same sender. Recovery responses do not disclose which service contains an address. Return links use the public homepage selected by the user. |
| Verification | Firebase email registrations request a branded Resend verification message from the server using fresh identity proof. Registration can be retried with the same credentials if the local account save was interrupted. Firebase verification is reflected locally on the next Firebase sign-in. Account verification controls select Firebase or native action links by saved ownership. |
| Browser push | A web VAPID key pair was generated with user approval; only its public key is supplied to browsers. Customer notification controls provide enable, disable and test actions. Delivery targets the authenticated subscription and current server session. Only new order updates are automatically dispatched; preferences are respected. Real browser/device receipt still needs a manual check. |
| Performance | Optional Firebase web performance diagnostics can be enabled for one page visit from Account → Privacy. Collection is off by default. No custom customer identifiers or commerce data are supplied. Dashboard receipt has not been independently verified. |
| Crashlytics | Not available for this Go/browser application: Firebase Crashlytics supports native Android, Apple, Flutter and Unity apps. Do not present web Performance Monitoring as crash reporting. |
| Other services | No unnecessary database, storage, functions, hosting, phone/SMS or billing services were introduced. Existing PostgreSQL and private object storage remain authoritative. |

Firebase currently rejects email-template changes for this project with `EMAIL_TEMPLATE_UPDATE_NOT_ALLOWED`, and the console explicitly directs the project owner to Firebase Support. Firebase-native branding has not been saved. The application now bypasses that editor using Admin-generated links and its own branded Resend HTML. `mail.weevcrafts.com` is verified; see [Resend integration](resend-integration.md) for sender, DNS, secrets and test evidence. No billing upgrade was required.

## Identity and data protection

Firebase access is limited to one active customer role. Operational and ambiguous-role accounts cannot enter through this endpoint. Registration records explicit terms/privacy consent. Firebase UID associations are private and never appear in public configuration. All Firebase session exchanges require same-origin CSRF proof. The server checks both local session cutoffs and Firebase disabled/deleted/revoked state; Firebase outages fail protected Firebase sessions closed.

Role replacement, suspension, native password changes and deletion retain the existing local revocation safeguards. Logout/all-session revocation removes relevant push registrations. Push tokens stay in private PostgreSQL, bound to a user and session, and expire with that session. A token cannot be transferred to a different account. Payloads contain a generic account-update notice, not names, addresses, order contents or financial information. Invalid subscriptions are removed. A Firebase service worker does not cache account pages.

Privacy/admin deletion removes Firebase bindings and push subscriptions transactionally and queues Firebase identity deletion for retry. Historical commerce records keep their existing retention rules. The queue must be monitored and drained with a working Firebase credential; disabling the integration pauses remote deletion. User-initiated performance collection may transmit technical device/network/page data to Firebase and remains optional.

Customers whose role is changed to an operational role need an operational credential established through the existing Super Admin user controls. Their removed native customer password is not silently restored.

## Local configuration

`firebase.json` declares the email/password and Google providers. Only Authentication configuration was deployed. Authorized domains include localhost, 127.0.0.1, weevcrafts.com and www.weevcrafts.com alongside the default Firebase domains. No OAuth client secret is shipped to the browser. The web app's public identifiers are in `compose.firebase.yaml`.

The local ignored `.env` selects `compose.yaml;compose.firebase.yaml`, using `COMPOSE_PATH_SEPARATOR=;`, and supplies the public VAPID key. Normal `docker compose up -d --build app` therefore includes Firebase on this machine. Other checkouts opt into the overlay explicitly after provisioning credentials.

The dedicated local service account `weevcrafts-customer-web@weevcrafts-708af.iam.gserviceaccount.com` has Firebase Authentication and Cloud Messaging administration permissions. Its private JSON remains outside the repository at `%LOCALAPPDATA%/WeevCrafts/firebase-admin.json`. Docker Desktop's file sharing did not expose that Windows file reliably, so the application uses the external Docker volume `weevcrafts_firebase_credentials`, mounted read-only at `/run/secrets`. The credential file is owned by the non-root application user with mode 0400. Protect both the Windows original and Docker data directory.

To provision that volume on another development machine, obtain a separately authorized development credential outside the repository, create the named Docker volume, pipe the file into a one-off container's standard input, write it with umask 077, set owner `app:app` and mode 0400, then remove the helper container. Do not print the JSON, embed it in shell arguments or generate another cloud key without authorization. Public Firebase config and VAPID values are not substitutes for that private server credential.

Firebase emulators are rejected in production. The application already rejects insecure production cookie/database/object-storage settings and disables demo flags. For production, prefer workload identity or a managed secret store, TLS, restricted infrastructure access and encrypted backups with verified restores; see `deployment-security.md`. Include Firebase deletion backlogs when planning restores. Firebase dashboard/privacy/retention controls and production domain cutover remain deployment-owner work.

The browser SDK is pinned to 13.0.0. This integration uses the still-supported `getToken()` registration interface with Admin SDK FCM v1 delivery. Firebase now deprecates that registration interface; plan a coordinated client/server migration rather than mixing the new FID flow into existing token delivery.

## Verification and customer checklist

Automated checks include the complete Go suite, PostgreSQL identity/linking/deletion tests, forged/stale/provider/role/CSRF rejection, and real Firebase disposable-account tests. The live tests verify ID-token exchange, an HttpOnly application session, a checkout return path and remote revocation, then delete the fixtures. Authentication traces are disabled and credentials/tokens are excluded from evidence. Existing customer header/navigation tests cover Chromium, Firefox and WebKit at nine viewport widths.

| Check | Result |
| --- | --- |
| `go test ./...` and focused checks after edits | Passed |
| PostgreSQL Firebase/privacy/role tests | Passed on isolated `weev_qa_20261009_unit` |
| Real Firebase provider and application-session tests | Passed; disposable remote identities removed |
| Push subscription ownership, private status and owner-only test/unsubscribe | Passed against two isolated customer fixtures |
| `test:firebase-ui` | Passed in Chromium, Firefox and WebKit |
| `test:storefront-header` | Passed for guest/customer, three browsers and nine widths |
| Secure customer Playwright suite | 27 passed; isolated database/Redis, authentication traces off |
| Template generation/check and JavaScript syntax | Passed |
| Main data preservation | 96 users, 28 orders, 72 products; volumes retained |

A temporary project-inspection diagnostic exposed Firebase's private password-hash configuration in tool output before it was corrected to emit service status only. This was disclosed during the work. No plaintext customer passwords or customer password-hash records were exported; the private configuration value is not copied into this repository or test evidence. Treat the tool history containing that diagnostic as restricted information.

Use the running preview at `http://localhost:8080`:

1. Open Login, accept the terms/privacy notice and select Continue with Google. A new customer should land on Products. An existing customer is asked to confirm their old password before linking.
2. Register a customer with email and a password of at least 12 characters. Verify the received WeevCrafts email, sign out and sign in again. Check Forgot password with that account; the branded email uses the Firebase-hosted action and returns to weevcrafts.com after the Firebase-hosted action.
3. In Account → Notifications, explicitly allow notifications, enable them, background the tab and send a test update. Confirm the operating system/browser receives it, then disable notifications and test logout. Browser/OS permissions and delivery cannot be guaranteed by server tests.
4. Check a real order update while notifications and Order updates are enabled. Disabling the preference should stop automatic order pushes.
5. Recheck wishlist add/remove, profile reminder/photo save, readable counters and the menu. Optional Privacy diagnostics should stay off until clicked and reset on reload.

The production Shopify site is unchanged. Firebase-native template editing remains blocked, while application-owned Resend branding is connected. Actual push receipt and performance-dashboard telemetry require the manual checks above, and there is no web Crashlytics integration.

References: [Google sign-in](https://firebase.google.com/docs/auth/web/google-signin), [server token verification](https://firebase.google.com/docs/auth/admin/verify-id-tokens), [web messaging](https://firebase.google.com/docs/cloud-messaging/web/get-started), [web performance](https://firebase.google.com/docs/perf-mon/get-started-web), [Crashlytics platforms](https://firebase.google.com/docs/crashlytics), [Firebase pricing](https://firebase.google.com/pricing).
