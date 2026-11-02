# Seller workspace reference implementation — 2 November 2026

The ten source images in `seller-admin-screens/` define the seller content redesign.
The expanded sidebar retains its existing visual rules. Desktop navigation adds a
64px rail; phones use the existing modal drawer through an explicit 960px breakpoint.
The Go/templ stack, existing forms, permission gates and business data remain authoritative.

## Implemented behavior

Products, inventory, orders and returns have full-dataset seller summaries.
Returns filter status and period before pagination. Seller notifications filter
all/unread/order/support updates and preserve filters through cursor pagination;
read actions remain recipient-owned. Support retains private confirmations,
attachments and saved conversations. Profile changes preview locally and use
shared validation, unsaved-change protection and failure recovery.

Seller settlement creation remains unavailable: the reference action opens support.
This preserves the existing finance boundary. No migration or JSON API is added.
Nine optimized imagegen illustrations and four native SVG illustrations are
separate from native controls, tables and text; prompt briefs are recorded with
the production assets. Generated art is never catalog data or a maker identity.

## Verification

- `go test ./...` passed, followed by focused handler/template/admin checks after refinements.
- Isolated PostgreSQL projection tests passed for summaries, seller isolation,
  return filters, pagination and inventory history. The helper creates disposable
  schemas only in `weevcrafts_projections_20261009`.
- Seller responsive Playwright suite: **15 passed**, Chromium/Firefox/WebKit,
  320, 390, 720, 768, 820, 1024, 1280 and 1440px; includes record reflow,
  long names/amounts, support failure/replies, keyboard and axe checks.
- Seller reference Playwright suite: **12 passed**, all three browsers;
  covers all ten empty reference screens, additional 960px checks, desktop rail
  persistence/tooltips, mobile drawer independence, profile recovery and notification
  recipient isolation. Navigation regression rerun: **3 passed**.
- `templ generate`, `npm run css:build`, JavaScript minification and strict premium
  audit passed. Design lint returned zero errors and one existing orphan-accent warning.
- In-app browser initialization timed out; Playwright supplied browser verification.
- Test writes use only the guarded `seller_workflows_20261017` database at a local
  QA port. Production receives read-only smoke checks.

The prior GitHub application check failed because an Admin inline script was
embedded with Windows CRLF literals. Its generated output now matches the LF
source and Linux generation; the script behavior is unchanged.

## Visual comparison

The accepted references and browser captures were inspected with `view_image`.
Desktop captures use each source PNG's native viewport dimensions. Mobile captures
use 390×844, with automated reflow checks down to 320px.

| Comparison | Result |
| --- | --- |
| Layout and hierarchy | Reference summaries, queues, support columns and profile preview are present. |
| Palette and surfaces | Seller-scoped cream, forest, tinted summaries and outlined panels retain shared branding. |
| Artwork | Page-specific craft banners and empty illustrations are separate assets; objects remain visible. |
| Controls and copy | Search, period, filter, upload and save controls remain native and connected to existing endpoints. |
| Navigation | Existing expanded sidebar styling remains; rail names/tooltips and mobile focus behavior were verified. |
| Reflow | Whole controls wrap, short actions remain intact and semantic tables become labelled records. |

Intentional differences are live identities/data, existing permission gates,
retained shared storefront chrome for notifications, platform-owned selects/file
pickers, and additional saved support history when records exist. Decorative
miniature growth charts are omitted rather than implying unmeasured growth.

Local QA object storage at port 13900 was unavailable. File selection, validation,
retention on rejected submissions and existing attachment service tests were checked;
no production uploads or business-data fixtures were created for verification.

Release target: GitHub `main`, Coolify application `vy2hpttvmehhauvskvilgoju`,
`https://weevcrafts.com`. The prior revision is `d0f43ed58ff6ee083db52cf1ab891d1e49a6cbd6`.
