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
- Chromium and WebKit route matrices pass. Firefox public routes pass; its long
  signed-in run encountered Windows software-renderer/navigation timeouts and
  is being checked separately. This is not reported as a complete Firefox pass.
- QA MP4 full and ranged response bytes match the source fixture. On Windows
  WebKit, unsupported codec decoding retains a real saved product photograph
  instead of showing a blank preview. Range, HEAD and error-cache tests pass.

Photograph derivatives are disposable and bounded; originals remain available
for zoom and are never overwritten. Saved editorial text is normalized only
for display. URLs, account identities, credentials and historical releases stay
unchanged. Payment recovery work is a separate release.
