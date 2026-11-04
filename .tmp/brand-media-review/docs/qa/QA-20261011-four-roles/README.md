# Four-role verification — 11 October 2026

This report records the first local four-role rollout, before the later designated credential sync and Railway release. The later sync explicitly creates the local Admin and preserves the production accounts. See `docs/railway-rollout.md` for the release procedure.

The local application on port 8080 now uses Customer, Seller, Admin and Super Admin. Migrations 42–43 were verified against an isolated restore before local rollout. The existing three accounts remain; no Admin account was automatically created.

| Business data | Before | After |
|---|---:|---:|
| Accounts | 3 | 3 |
| Products | 0 | 0 |
| Orders | 0 | 0 |
| Migration version | 41 | 43 |

The existing seller assignment changed from `seller_owner` to `seller`. Customer and Super Admin assignments stayed canonical. Local login, portal access, forbidden company access for Customer/Seller, the four-role account selector and logout were smoke-tested after deployment.

## Checks performed

- `go test -p 1 ./...` passed with isolated PostgreSQL and Redis dependencies before final cleanup. The final full run passed every package except the 100-user inventory fixture, which exceeded its setup deadline during concurrent build/browser work. Its standalone retry passed; all other final tests, including private coordination and changed handler/template packages, passed.
- PostgreSQL/S3 integration checks passed with `S3_INTEGRATION=1`, covering media authorization/lifecycle, legacy migration mapping and rejection, concurrent Super Admin protection, seller obligations and private multi-seller requests.
- Nine isolated workflow scenarios passed, including confirmed creation and login for all four roles and pending Seller onboarding. Four additional portal tests passed across 360, 390, 768, 1024 and 1440px. All 13 scenarios passed together against the final build.
- Templ generation, `npm run css:build` and `git diff --check` passed.
- Strict premium UI audit reported zero errors, warnings and violations.
- Desktop and 390px screenshots were inspected. Keyboard opening, focus trapping, Escape restoration, visible logout and reduced-motion operation passed.

The Playwright run used only `weevcrafts_roles_browser` on port 8081. Its fixtures included a second seller and a multi-seller order. Tests proved exact submission retries do not duplicate messages, conflicting retries fail, foreign seller replies fail, an Open request cannot close before the Seller answers, private replies/internal notes do not enter the customer conversation, and resolving the case leaves the order paid.

Use the current isolated role suite explicitly:

```powershell
$env:WEEVCRAFTS_QA_DATABASE = 'weevcrafts_roles_browser'
$env:WEEVCRAFTS_BASE_URL = 'http://127.0.0.1:8081'
npx playwright test tests/e2e/four-role-workflow.spec.js
```

Provision that disposable deployment with the explicit QA seed before running it. The larger demo portal regression suite was updated to canonical roles and syntax-checked. Its four portal layout tests were also run with isolated launch accounts; the remaining historical demo tests were not rerun. These results apply to the current four-role scenarios and Go suite.

## Rollout and limits

PostgreSQL custom-format backups were checked with `pg_restore --list`. An isolated restore retained all three accounts after migration. A fresh backup and the previous image tag `weevcrafts-four-roles:pre-upgrade` were retained before applying the tested image locally. Existing pre-migration sessions were invalidated.

Backup and raw verification output are retained under `%TEMP%\weevcrafts-four-roles`. Disposable QA services were removed after verification. No external production environment was deployed or certified. Courier integration, live payouts, refund reconciliation and privileged MFA remain separate release work.
