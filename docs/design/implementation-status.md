# WeeVCrafts implementation tracker

Updated 15 October 2026. Scope: finish the approved customer reference redesign,
preserve existing marketplace workflows, replace old WeeVCrafts Docker resources,
and show a local real-data preview. The user subsequently authorized a safe Git
push; the changes are being committed to a review branch. Deployment remains out
of scope.

## Completed and retained

- [x] Approved warm customer palette and shared two-row header/light footer.
- [x] Homepage sections and Home & Living listing composition.
- [x] Five distinct editorial hero banners, 4-second autoplay, Pause/Play and reduced motion.
- [x] Supplied logo across customer and operational branding/favicons.
- [x] Shared server-rendered Quick view, genuine options, existing cart feedback.
- [x] Filters, URL history, wishlist fragment presentation and existing account flows.
- [x] Air v1.67.4 installed and browser reload verified after source edits.
- [x] Existing Go/template, focused browser, keyboard and responsive checks passed.

## Current completion pass

- [x] Remove demo/simulated defaults and automatic seed commands from normal startup.
- [x] Preserve database, media and credentials before replacing Docker resources.
- [x] Remove only WeeVCrafts containers, project images and identifiable project build cache.
- [x] Build fresh images from current source, including regenerated assets/templates and PNG logo.
- [x] Start one real-data Docker stack and check persistence, health and media.
- [x] Complete source/handler review of customer route families and fix remaining listing drift.
- [ ] Final-stack visual review by user; automated browser access is policy-blocked.
- [x] Verify five-slide images, fonts and logo in the final image.
- [ ] Reconfirm narrow layouts manually; earlier responsive checks passed before the final Docker rebuild.
- [x] Open the final local preview and report remaining external configuration dependencies.

## Data and provider rules

No demo/QA catalog or accounts are inserted. Missing catalog/portrait/review data
stays missing. Existing users/orders/media are retained. The previous isolated
verification containers are removed rather than being presented as the app.
Real Razorpay credentials are currently absent; payments must remain unavailable
until configured, rather than report simulated success. Existing private Firebase
and transactional-email configuration is retained without printing credentials.

Live provider sends, real charges and public deployment are outside this local
rebuild. The subsequent Git push is authorized separately. See the implementation
report for earlier verification evidence.

## Current pass findings and fixes

- Normal startup had still invoked local-account provisioning and simulated payment
  defaults. Both are removed; real provider readiness continues to gate checkout.
- Docker excluded every PNG, which would omit the supplied logo. Maintained asset
  PNGs are now explicitly included.
- The image had relied on host-generated assets. It now builds responsive images,
  Tailwind, minified CSS/JS and templ inside the Docker build.
- Listing active chips sat below an extra divider and filter rows inherited excess
  spacing. Chips now share the desktop toolbar; desktop facet rows are denser while
  the mobile sheet retains touch targets.
- Backed up both database volumes and both media stores before cleanup. The main
  retained database has four existing accounts and no products/orders. The temporary
  website database was empty. No records were copied from the isolated QA database.
- Removed 31 WeeVCrafts containers and their project images. Three unrelated
  Debian/ISO containers remain. All volumes are retained. The scoped old application
  build-cache cleanup reclaimed 27.42 GB; unidentifiable/shared cache is retained.

Go checks on the final source pass: `go test ./web/... ./internal/web/handlers
./internal/transport/web ./internal/config`. Compose validates without starting
seed/demo/QA services. No Git operations have pushed or published this work.

## Final-stack verification status

The fresh application, PostgreSQL and Redis containers are healthy. Garage setup
completed successfully. The application reports its existing Resend sender configured
and starts with real Razorpay mode, local simulation disabled. Account/product/order
counts remain 4/0/0. The supplied logo and icon are included in the container build.

Automatic final-stack browser inspection is blocked by the in-app browser URL
security policy. Its getTab action was rejected; no alternate browser/control path
is used to bypass it. Earlier rendered comparison and keyboard/reflow evidence
remains recorded, but the small final toolbar-spacing change and real-data empty
states require manual visual review in the opened preview. This is a verification
limitation, not a claim that every screen has been visually reapproved.

Final image HTTP verification: all five editorial banners and both self-hosted fonts
return 200. The logo and favicon bytes exactly match their maintained files. Both
shared brand CSS and reference CSS match the latest local source build. Readiness
and liveness return 200; `/api/v1/local/` and `/dev/payments/` return 404. Runtime
UID is 100 (`app`). The in-app preview pane was opened via the app's file/browser
panel tool without attempting to observe blocked browser content.

Air's real Docker path is verified: a watched source change rebuilt the Compose
application, waited for healthy status, and emitted its SSE browser-refresh event.
Both ports 8080 and 8090 return readiness 200 afterward. No reload loop followed.
No demo/QA containers remain. The final image retains the latest source styles.

Ready for manual review: **http://localhost:8090**. Direct container URL:
**http://localhost:8080**. Start/restart live reload with `npm run dev:docker`.
The current runtime is local HTTP, not an internet production deployment. Full
production validation/TLS requirements remain unchanged. Real payment activation
still requires the user's provider credentials and webhook configuration.
