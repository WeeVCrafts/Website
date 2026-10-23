# Seller responsive UI verification — 24 October 2026

The seller portal keeps its existing Go/templ implementation, branding and business
workflows. The change fixes compressed table columns and action labels, gives the
earnings ledger full width, and structures support creation, conversations and
replies with the existing seller form controls. Support field names remain stable
when shared inline validation inserts error text.

## Coverage and results

- Twelve seller routes at 320, 390, 720, 768, 820, 1024, 1280 and 1440 CSS px,
  plus saved-product and order-detail navigation. Phone records retain their labels
  and actions; desktop overflow remains inside named table regions.
- Fifteen regression cases across Chromium, Firefox and WebKit ultimately passed:
  responsive route matrix; keyboard drawer/search/disclosure; support validation,
  retained text/files after rejection, saved replies, closed tickets and filtering;
  long records/large amounts; and axe accessibility checks on four seller surfaces.
- The main cross-browser run passed 13/15. Two Firefox navigation checks timed out
  while the QA media endpoint was still unavailable after the upload-failure test.
  Both passed when media access was restored; no application change was needed.
- The existing seller workflow suite passed six cases with storage available. Its
  dedicated cover-failure case then passed with only the QA storage endpoint
  unavailable. Together these cover all seven seller cases, including product
  steps, retained server errors, no-JavaScript entry, saved-draft recovery, stock,
  reservations, order pagination/stages/conflicts and authorization boundaries.
- Targeted Go tests, generated-template consistency, CSS build, JavaScript syntax,
  strict frontend design audit (zero findings), and `git diff --check` passed.
- The Docker production image built successfully. The rebuilt local application
  at port 8080 reports PostgreSQL and Redis ready. Its served seller CSS and shared
  UI JavaScript match the workspace assets byte for byte.

Firefox retry interception stays registered throughout its test, changing only
whether the simulated request is rejected or forwarded. Reply checks wait for the
saved message before reloading. These avoid test-harness navigation races.

## Commands

```powershell
templ generate ./web/...
templ generate --check ./web/...
npm run css:build
go test ./web/selleradmin ./internal/web/handlers ./internal/transport/web
python C:/Users/Vinay/.codex/skills/frontend-design-premium/scripts/audit_project.py . --mode strict --no-write
docker compose build app
docker compose up -d --no-deps app

$env:WEEVCRAFTS_QA_DATABASE='seller_workflows_20261017'
npx playwright test --config tests/e2e/seller-responsive.config.js --workers 2
npx playwright test --config tests/e2e/seller-responsive.config.js --project firefox --grep 'all seller routes|navigation, search' --workers 1
npx playwright test --config tests/e2e/seller-workflows.config.js --project Chromium --grep-invert homepage
npx playwright test --config tests/e2e/seller-workflows.config.js --project Chromium --grep 'cover upload failure'
```

Browser tests use the existing isolated seller-workflow database and QA server at
`127.0.0.1:18080`, with its existing demo seller and workflow fixtures. The new
suite refuses other database/base-URL combinations before fixture writes. Use
working QA media for layout checks and unavailable QA media for the explicit
cover-failure scenario. Existing local application records were not reset.

## Visual evidence and limits

Reviewed screenshots: [products desktop](products-1440.png),
[products phone](products-320.png), [earnings desktop](earnings-1440.png),
[earnings phone](earnings-320.png), [support form desktop](support-create-1440.png),
[support form phone](support-create-320.png), and
[saved conversation](support-conversation-390.png).

The suite also verifies 640 × 500 reflow, equivalent in CSS viewport size to a
1280 × 1000 browser at 200% zoom. Native browser-menu zoom, physical touch devices
and screen-reader output were not manually verified. Axe results cover the tested
seller main-content surfaces and do not constitute accessibility certification.
