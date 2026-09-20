# Local demo accounts

Run the seed workflow before signing in:

```sh
docker compose --profile demo run --rm seed-demo
```

Every seeded account uses the local-only password `WeeVCrafts-Demo-2026!`.
Sign in with the full email address using the `demo.*@weevcrafts.local` pattern.
The accounts use the `@weevcrafts.local` domain and are not production
credentials. The seed is insert-only: it does not reset passwords, roles, or
business records that already exist.

| Account range | Role |
| --- | --- |
| `demo.customer01` – `demo.customer08` | Customer |
| `demo.owner01` – `demo.owner03` | Seller owner |
| `demo.staff01` – `demo.staff03` | Seller staff |
| `demo.marketplace01` – `demo.marketplace03` | Marketplace admin |
| `demo.super01` – `demo.super02` | Super Admin |
| `demo.support01` – `demo.support02` | Support agent |
| `demo.finance01` – `demo.finance02` | Finance operator |
| `demo.operations01` | Operations and returns |

Use `http://localhost:8025` to inspect local verification and recovery mail.
Local payment scenarios are available only in development mode:

```sh
curl -X POST http://localhost:8080/dev/payments/WCDEMO1001/capture
curl -X POST http://localhost:8080/dev/payments/WCDEMO1001/fail
curl -X POST http://localhost:8080/dev/payments/WCDEMO1001/pending
```

Additional local-only PostgreSQL APIs are available below `/api/v1/local/`
for addresses, notifications, promotions, reviews, seller finance, support
tickets, and Super Admin settings. Mutating requests require the signed-in
session's `X-CSRF-Token`; the routes are not registered when local mode is off.
