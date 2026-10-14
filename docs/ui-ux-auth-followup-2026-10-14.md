# Public discovery and account access follow-up

The user's preview review requested exploration before login and improved sign-in/sign-up UX. This supersedes the initial release's preserved, mandatory frontend login gate. The server already exposes public catalogue routes; no API/service/authorization changes are required or included.

## Implemented

- Home, catalogue, categories, search, offers, brands/makers and detail pages no longer open a login dialog on arrival or intercept public navigation.
- Existing protected actions still request sign-in. Close, Keep browsing, Escape and backdrop dismissal return focus without performing a mutation. Direct protected server routes and API authorization remain enforced.
- Account screens retain the accepted palette/type, a compact single-column card, visible browsing exit and small Help/Privacy/Terms footer. No new photos, animation framework, identity provider or package dependency.
- Shared password fields use explicit labels, accessible reveal controls, appropriate autocomplete and the existing password requirements. Labels remain stable when hints/errors appear.
- Login/register switching keeps the existing validated internal return path. Account forms use shared inline validation before the intercepted request, duplicate guards, stable busy controls, retained input and focused persistent error feedback.
- Optional Google controls remain hidden until configured and ready; existing Google consent/account-link proof remains unchanged. Native account forms remain available without JavaScript.

## Verification and scope

Dedicated preview: `http://127.0.0.1:18080`, database `weev_qa_ui_20261014` on host port 15432, Redis on 16380. Active application/data at 8080 were not reset or seeded. Provider/email credentials remain disabled in the isolated runner; the happy signup test creates one fictional local QA customer only. No real payment or external email is sent.

`npm run test:ui-ux:auth` exercises Chromium/Firefox/WebKit guest navigation, filtering, protected-action dismissal/zero writes, private-route rejection, return paths, empty/invalid/failed/pending form states, reveal/autofill, valid customer login and responsive keyboard operation. Four auth routes at seven widths produce 84 viewport checks. Chromium additionally runs the four-route axe audit, native signup success and no-JavaScript discovery/forms. Final results: 23 checks passed; zero reported automated accessibility violations or uncaught page errors.

Generated templates, CSS/JavaScript builds and `go test ./web/... ./internal/web/handlers ./internal/transport/web` pass. Strict premium audit and DESIGN.md lint report zero errors/warnings. The first browser run exposed changing field names and test synchronization/scoping issues; that report is preserved alongside the corrected passing run, not described as a pass.

Existing header checks pass in all three engines for guest/customer at nine widths, including the new public-discovery contract. Existing provider-fallback checks pass in all three engines; the configured Google provider path is not claimed from those disabled-provider checks. All four current-role landing/boundary/responsive regression tests pass. Final native preview executable SHA-256: `9737367ff82a94bb02bb352dc53af878b6950cfc8868d38a46bc31ea2cca3726`.

Chosen-option shopping, confirmed Wishlist retrieval and server-authoritative Cart checks pass in all three engines. One earlier WebKit Wishlist interaction timed out; its failed report is retained and the repeat passed on the same UI build. Local checkout duplicate activation, confirmation/refresh and same-order failed-payment recovery also pass. These tests create only fictional isolated orders and charge no money.

See `qa/UI-UX-auth-20261014/` for reports and desktop/mobile captures. Earlier release/device/provider gates still apply: desktop engine tests are not physical-device or screen-reader certification; configured Google/provider/storage paths require their existing external sign-off. Backend domain rules, roles, sessions, CSRF and password policy remain unchanged.

Repeat checks only against the dedicated setup:

```text
templ generate -check
npm run css:build
go test ./web/... ./internal/web/handlers ./internal/transport/web
npm run test:ui-ux:auth
```

Set `WEEVCRAFTS_BASE_URL=http://127.0.0.1:18080` for the existing header/provider/role checks. The auth script refuses other endpoints. Artifact output uses `WEEVCRAFTS_UI_ARTIFACTS` or the existing temporary-directory default.
