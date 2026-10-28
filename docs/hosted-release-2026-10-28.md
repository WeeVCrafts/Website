# Hosted payment and SEO release — 28 October 2026

## Deployment

The user authorized deployment and requested a direct GitHub push. Commit
`a198a7ce00fa234c4eea0884f2a625e346a07121` was pushed to `WeeVCrafts/Website`
on `main`. Railway automatically deployed it to `https://weevcrafts.com`.
Deployment `a630131c-ebb7-4675-b7ea-94cc56c0425f` reached `SUCCESS`.

Razorpay test credentials and the webhook secret were copied into the private
Website service variables without printing them. `WECRATFS_PAYMENT_MODE` is
`razorpay`; local/demo switches are off. The key starts with `rzp_test_`, and
the deployed configuration matches the verified local sandbox configuration.

## Hosted verification

- `/health/ready` returned 200 with PostgreSQL and Redis ready.
- All **45 sitemap URLs** returned 200 with `index,follow`, a matching canonical
  URL and a nonempty description.
- Four legacy redirects returned 301 with the intended destinations. Unknown
  brands returned a real 404.
- A signed webhook deployment probe returned 200 and `processed: true` once,
  then `processed: false` for the duplicate. An altered raw body returned 401.
  Its provider order deliberately matched no customer order, so the probe
  changed no customer order or inventory.
- A fresh Razorpay sandbox netbanking payment completed through the isolated
  QA storefront. Razorpay reported `captured` for INR 1,398.00. Railway's HTTP
  logs recorded both provider webhook deliveries to the hosted endpoint with
  200 responses at 03:38:08 and 03:38:09 UTC (09:08 IST).
- The provider test created no hosted customer order. Settlement, stock and
  replay behavior were verified in the isolated QA database; hosted evidence
  verifies provider delivery and signature handling, not a production customer
  checkout. No real money was charged.
- Both Playwright sandbox/sitemap checks passed again after deployment.

## Search Console

The updated sitemap was submitted successfully on 28 October. Search Console
fetched it and reported **Success, 45 discovered pages**. The homepage was
already indexed before this release.

An individual indexing request for `/size-guide` failed with Google's message
that its reCAPTCHA service could not connect. The accepted sitemap remains the
bulk discovery path. A live URL inspection was also started. Search indexing
and ranking are controlled by Google and do not change immediately after a
deployment.

Private login/wishlist exclusions and genuinely removed products are intentional.
Do not remove private-page `noindex` directives or redirect unrelated retired
products merely to eliminate every excluded URL in the report.

## WWW alias repair

The release audit found `www.weevcrafts.com` returned NXDOMAIN. The domain uses
Cloudflare nameservers. Railway's plan allows only one custom domain on this
service, so the alias is handled at Cloudflare without a plan upgrade.

- Created a proxied `A` record for `www` using Cloudflare's redirect-only
  documentation address `192.0.2.1`.
- Enabled the Single Redirect rule `WeevCrafts www canonical redirect`.
- Match: `(http.host eq "www.weevcrafts.com" and http.request.method in {"GET" "HEAD"})`.
- Target: `concat("https://weevcrafts.com", http.request.uri.path)`.
- Status: 301; preserve query string: enabled.

Authoritative DNS, public recursive DNS and the local resolver confirmed the new
record. HTTP and HTTPS requests for `/categories/textiles?sort=newest` redirected
to `https://weevcrafts.com/categories/textiles?sort=newest`, with TLS verification
enabled. The main hostname's DNS and origin routing were retained.

## Evidence

- [Hosted verification data](qa/razorpay-seo-20261028/hosted-verification.json)
- [Google sitemap submission](qa/razorpay-seo-20261028/google-sitemap-submitted.png)
- [Enabled www redirect](qa/razorpay-seo-20261028/cloudflare-www-redirect.png)
- [Original payment integration checks](razorpay-integration.md)
- [Search Console findings](seo-audit-2026-10-28.md)

The existing S3 bucket probe signature warning was also present in the prior
deployment's logs. Storage configuration was outside this payment/SEO release.
