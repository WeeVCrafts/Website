# Clean local application

The local app starts with exactly three active accounts, one seller studio and no products, addresses or orders.

| Role | Email |
| --- | --- |
| Customer | customer@weevcrafts.local |
| Seller owner | seller@weevcrafts.local |
| Super admin | superadmin@weevcrafts.local |

All three local accounts use `WeeVCrafts-Local-2026!`. `WECRATFS_RESET_PASSWORD` overrides this development password (minimum 12 characters).

Normal `start.bat` / `start.sh` uses the `clean` Compose profile. It provisions only an empty database and preserves existing accounts on subsequent starts. The old `demo` profile is optional and adds demo accounts/products; do not run it for the clean presentation.

Super Admin has a dedicated **User Management** navigation entry at `/super-admin/users`. Search by name, email or role; expand **Manage account** to edit account details, suspend/reactivate, reset passwords, change roles or delete non-protected accounts. Changes require an audit reason. Password/role changes revoke existing sessions; changing an email clears email verification. Deletion retains order/audit history. Super Admin access remains bootstrap-controlled.

Authenticated customer, seller and admin headers expose a visible top-right **Log out** action at desktop and mobile widths. It submits POST with a session CSRF token.

## Explicit destructive local reset

Back up PostgreSQL, the uploaded image volume and the configured storage bucket before resetting. Stop the app to prevent writes during reset. From this repository in PowerShell:

```powershell
docker compose stop app
$env:WECRATFS_LOCAL_MODE='true'
$env:WECRATFS_ENV='development'
$env:WECRATFS_RESET_LOCAL='true'
go run ./cmd/reset-local
Remove-Item Env:WECRATFS_RESET_LOCAL
```

The reset accepts only the local `wecratfs` PostgreSQL database and local Compose service endpoints. It clears application tables, Redis sessions/cache, the product search index, the configured `wecratfs-media` bucket and the local Mailpit inbox. Service cleanup failures are reported; finish cleanup before restarting the app. This command does not delete remote Firebase identities.

The separately mounted upload volume must also be cleared while the app is stopped, after backing it up:

```powershell
docker compose run --rm --no-deps --entrypoint sh app -c 'find /app/web/assets/images/uploads -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +'
docker compose up -d app
```

The app upload path above is a fixed container path; keep it unchanged. The reset preserves schema migrations and re-creates role definitions, categories and the seller's brand so the product editor works without sample catalogue records.

## Verification (11 October 2026)

- Targeted Go checks passed for reset guards, authentication, PostgreSQL, portal handlers and admin rendering.
- CSS build/minification, JavaScript syntax and Compose validation passed.
- Four clean-start Playwright tests passed: working login/logout for all three roles at 1440, 960, 720, 390 and 320px; keyboard logout; customer/account management; protected Super Admin controls; seller access rejection; account creation/search/edit; suspension/reactivation; password visibility/reset (new password authenticated successfully); role replacement; anonymizing deletion; and CSRF rejection.
- The storefront header passed its nine-width interaction checks in Chromium, Firefox and WebKit for guest and authenticated customer sessions.
- An isolated PostgreSQL container verified a fresh three-account bootstrap, preservation of existing accounts on repeat startup and preservation of existing categories when the user table is empty. The check removed its container afterward.
- The strict frontend design audit reported zero findings. Desktop and narrow screenshots were visually inspected.
- Temporary verification accounts and their audit/session data were removed with a final explicit reset. Final PostgreSQL counts: users 3, sellers 1, products 0, variants 0, orders 0, addresses 0, support tickets 0 and Firebase links 0. Uploaded files, product search documents and local development messages were cleared. The app health endpoint returned 200.

Before-reset backups are stored outside the repository under `C:/Users/Vinay/.codex/visualizations/2026/10/10/01a1244d-2dbf-77f0-9e44-50278f80c429/before-clean-start` and `before-clean-continuation` (PostgreSQL, upload files and storage). Screenshots and the strict audit report are in the parent directory.

To repeat the focused browser verification on a fresh local dataset:

```powershell
npx playwright test tests/e2e/clean-start.spec.js
./scripts/check-clean-bootstrap.ps1
```

The browser test creates a temporary account and verifies deletion/anonymization; perform the explicit local reset afterward to return to exactly three stored accounts.
