# Razorpay test payments

The application uses Standard Checkout, server-created Orders API orders and
HMAC-verified callbacks. Only verified capture webhooks mark orders paid.
Credentials determine the mode: `rzp_test_` keys use the sandbox. Changing the
Dashboard toggle alone does not change the application's mode.

## Configuration

Keep these values in the ignored local `.env` or Railway service variables:

| Variable | Test configuration |
| --- | --- |
| `WECRATFS_PAYMENT_MODE` | `razorpay` |
| `WECRATFS_LOCAL_MODE` | `false` |
| `WECRATFS_NO_PAYMENT_DEMO` | `false` |
| `RAZORPAY_KEY_ID` | Test key beginning with `rzp_test_` |
| `RAZORPAY_KEY_SECRET` | Matching private API secret |
| `RAZORPAY_WEBHOOK_SECRET` | Private webhook signing secret |
| `RAZORPAY_BASE_URL` | `https://api.razorpay.com` |

Checkout, recovery and confirmation identify test payments. Confirmation emails
also identify payments made without real money. Private secrets stay server-side.

The account was verified in Test Mode on 28 October 2026. Its existing automatic
capture setting is used; order creation no longer overrides it with manual
capture. The test API key was regenerated with user authorization and a 24-hour
transition. The previous test key expires on 29 October 2026 at 07:50 IST.
The new credentials and a random webhook secret are saved in the local `.env`.

An enabled **test** webhook was saved for
`https://weevcrafts.com/webhooks/razorpay`, subscribing to `payment.captured`,
`payment.authorized` and `payment.failed`. Its secret matches the local config.
At the time of the local checks below, hosted delivery had not been verified.
It requires the updated app and matching variables on Railway. Localhost cannot
receive that public webhook directly.

The handler verifies the original bytes using `X-Razorpay-Signature` and uses
`X-Razorpay-Event-Id` for idempotency. A verified body digest is the fallback when
a proxy omits the event ID. Late authorization/failure events cannot downgrade a
captured payment.

## Verification on 28 October 2026

- Razorpay's real Orders API accepted a test order for INR 1.00.
- The full Go suite passed against the isolated PostgreSQL QA service.
- Focused PostgreSQL checks passed for signed capture, stock commitment,
  duplicate delivery and late events. `go vet ./...` passed.
- Race checks passed for the Razorpay adapter, payment application, webhook
  transport and PostgreSQL capture regression. The final Docker build passed;
  the rebuilt normal local app reports healthy with test credentials configured.
- The opt-in Playwright test completed a real sandbox netbanking flow using
  Razorpay's demo bank. It verified the callback and fetched the captured payment
  from Razorpay's API. The order stayed pending until a signed capture was
  replayed locally, then displayed payment completed. An altered body returned
  401; a duplicate returned `processed: false`.
- Browser checks covered 390px and 1440px widths, keyboard activation, test-mode
  notices and absence of page errors. No real money was charged.

The browser test uses a dedicated local database with synthetic accounts/products
and separate Redis on port 16381. It does not seed the normal local database or
the hosted marketplace. Against the explicitly prepared fixture on port 18083:

```powershell
$env:WEEVCRAFTS_BASE_URL='http://localhost:18083'
$env:WEEVCRAFTS_RAZORPAY_QA='true'
npx playwright test tests/e2e/razorpay-testmode.spec.js
```

It refuses other hosts/ports and live keys, and is skipped by normal test runs.
Provider-to-hosted-app delivery must be checked after deployment; the local
signed replay does not prove it.

Screenshots: [desktop confirmation](qa/razorpay-seo-20261028/payment-confirmed-1440.png),
[mobile confirmation](qa/razorpay-seo-20261028/payment-confirmed-390.png), and
[enabled test webhook](qa/razorpay-seo-20261028/razorpay-test-webhook.png).

Provider references: [webhook validation](https://razorpay.com/docs/webhooks/validate-test/),
[Orders API](https://razorpay.com/docs/api/orders/create/), and
[capture settings](https://razorpay.com/docs/payments/payments/capture-settings/).
