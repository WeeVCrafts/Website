# Uniform customer header and login destination

Customer screens share the supplied icon-above-label layout for Wishlist, Cart
and Sign in. Authenticated customers see Account in place of Sign in.
Notifications, its live unread count, and Sign out are available inside Account.
All three actions retain visible 14px labels, 24px icons and targets at least
48px wide and high, including phone layouts. Search occupies its own row at
800px and below. At 600px and below, the brand uses its logo to retain space for
all actions. Initial rendering and cart/wishlist counter updates share markup.

The shared customer header also appears on sign-in, registration and recovery.
Sign-in keeps its existing form and artwork, with Continue browsing inside the
form content area. The new customer-header stylesheet owns action geometry;
conflicting homepage action overrides were removed. Operational portal headers
and the seller notification header retain their existing presentation.

Ordinary customer authentication defaults to `/`. Validated customer return
paths, including selected product variants and checkout intent parameters, remain
authoritative when login started for a task. Native forms, JSON password login,
Firebase exchange and account linking use the same server redirect policy.
Browser fallbacks also use `/`. Registration that starts an authenticated
customer session follows the shared policy. Account settings still use `/account`.
Operational role destinations and ambiguous/unassigned-role rejection remain.

## Verification

- Go tests passed for application auth, API transport, web transport, shared
  components and pages. Redirect tests cover homepage defaults, safe task paths,
  rejected external/invalid destinations and operational role destinations.
- All 12 new browser checks passed across Chromium, Firefox and WebKit. The
  matrix covers 12 guest routes and six authenticated routes at 320, 390, 768,
  800, 801, 1024, 1440 and 1920px. It checks visible labels, stacked alignment,
  three actions, 48px targets, counts 0/12/123, no page overflow, Account keyboard
  operation and notification/sign-out access.
- Password login passed with and without JavaScript, both with and without a
  return destination. Mocked Google exchange and existing-account linking passed,
  including missing-response-destination fallback to the homepage.
- Existing storefront header, authentication/discovery and shopping regressions
  passed in all three engines. The auth suite checks form errors, duplicate
  submission protection, service failure recovery, unavailable Google fallback,
  keyboard operation, axe checks and native account forms.
- Updated stale auth regression expectations to use the existing Arts category,
  preserve variant/quantity continuation parameters, and expect the QA checkout
  release gate's HTTP 503. Checkout configuration was not changed.
- Affected templ outputs and minified CSS/JS were regenerated. Asset regression,
  JavaScript syntax and whitespace checks passed. Desktop/mobile homepage,
  listing, sign-in and Account-menu screenshots were inspected.

The new matrix can be rerun against the isolated preview with:

```powershell
npx playwright test -c tests/e2e/customer-header.config.js
```

Live Google authentication, physical devices, native browser zoom and an actual
screen-reader session remain unverified. Google tests use intercepted provider
modules and exchange responses; they do not connect real identities. Screenshots
and routine test artifacts are under the existing temporary UI QA directory.
No production deployment or push is part of this change.
