# Customer media and Weevcrafts branding

Scope: responsive product photographs, video previews, product-detail reflow,
preferred `Weevcrafts` spelling and the requested introduction below Home's hero.
Social profiles are outside scope because none currently exist.

Validation used an isolated PostgreSQL database and real QA object storage.
The owner's generated category/product packs were not imported. Production
checks are read-only.

- Go package tests, templ generation and CSS/JavaScript builds pass.
- Strict premium audit: zero findings. Design-contract lint: zero errors;
  the existing unused `accent` token remains a warning.
- Branding, introduction placement and metadata pass in Chromium, Firefox and
  WebKit at 320, 390, 768, 1024 and 1536 pixels. Browser captures inspected at
  390 and 1536 pixels. Desktop hero remains 420 pixels.
- Product-detail media, keyboard gallery/zoom, long titles, narrow controls and
  WCAG A/AA axe checks pass in all three engines. Customer route checks cover
  46 public/account/order routes before and after sign-in at five widths.
- Chromium and WebKit route matrices pass. Firefox public and signed-in routes
  pass too. Its first long signed-in run encountered Windows graphics/navigation
  timeouts; opening a fresh page for each viewport released rendering surfaces
  and the complete 230-navigation signed-in rerun passed in 7.9 minutes.
- QA MP4 full and ranged response bytes match the source fixture. On Windows
  WebKit, unsupported codec decoding retains a real saved product photograph
  instead of showing a blank preview. Range, HEAD and error-cache tests pass.

Photograph derivatives are disposable and bounded; originals remain available
for zoom and are never overwritten. Saved editorial text is normalized only
for display. URLs, account identities, credentials and historical releases stay
unchanged. Payment recovery work is a separate release.

Published revisions: `6d4fa94` and follow-up `9e56f5c`. The follow-up fixes a
legacy saved About heading found by production inspection and normalizes templ
input newlines so Windows generation matches Linux CI. CI passes at `9e56f5c`.
Coolify runs that exact revision. Public Home, Our Story and About return 200
with the preferred name; readiness returns 200. The observed product cover's
320-pixel version is 32,772 bytes, versus its 3,032,247-byte original. Missing
product and storefront media return 404 with `no-store`.
