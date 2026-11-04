# Woven Gallery sign-in

The user selected concept C, then explicitly requested username/password fields inside the mobile prompt and Continue with Google on both surfaces. The updated [reference](woven-gallery-auth-concept-2026-10-15.png) records those changes. Development stays on `main`. Local implementation and Air verification are complete; the user subsequently authorized pushing this change to GitHub and verifying Railway's automatic production deployment. The preceding production release is `523484c066f8758472554cc4b7ac4aae5dc8a926`.

## Design and implementation

- `/login` uses the approved open form at left, decorative still life at right, a restrained WeevCrafts wordmark with the large supplied woven mark, a browsing exit and Help/Privacy/Terms footer. Mobile uses a shallow photo strip and natural page scrolling.
- The protected-action prompt contains the same labelled username/password fields, password visibility control, recovery link, Sign in and Continue with Google. Mobile is a centred native modal over a translucent green backdrop; desktop retains a right-side panel. Short viewports scroll inside the prompt. Browsing alone does not open it.
- Shared templates own the credential fields and Google controls. Existing password authentication, Firebase exchange, CSRF, internal return paths, explicit provider consent and existing-account proof remain authoritative. Server authorization and business APIs are unchanged. No migrations or new dependencies.
- Closing resets form values, password visibility, validation feedback, Google consent and pending account-link proof, then returns focus. Tab/Shift+Tab wrap inside; Escape and backdrop dismissal perform no shopping action. Buttons keep existing duplicate-submit and busy handling; account linking now also guards repeats and correctly disables its submit button.
- A configured Google provider enables the button. Provider failure leaves a visible disabled button with an explanation and working password form. The closed shopping prompt does not load Firebase's browser SDK until opened.

Tokens: forest `#123F32`, hover `#0D3529`, warm white `#FFFDF8`, ivory `#F8F2E8`, terracotta `#9D4031`, gold `#906220`, ink `#20362D`, muted `#56665D`. Input boundaries use `#7D8D80` for sufficient contrast. Labels use 14px DM Sans; inputs and primary/provider buttons use 16px; controls are 50px high. The phone prompt has a 16px radius and at least 16px outer margins. Native icons are close, eye/reveal and browsing arrow; Google uses its multicolour G. The small popup image is decorative and cannot intercept controls.

Allowed copy includes Sign in; Your saved pieces and orders, in one place.; Username or email; Password; Forgot password?; Continue with Google; the existing privacy/terms agreement; New to WeevCrafts? Create account; Continue browsing; Help; Privacy; Terms. The prompt additionally uses Sign in to continue; Save your favourite pieces and keep your orders together.; Keep browsing. Existing errors, notices, account-link proof and return-context messages appear only when relevant. No unsupported provider, OTP, testimonials or trust claims were added.

## Visual comparison

The reference and rendered screenshots were inspected directly with `view_image`. Comparison covers the following concrete points:

| Point | Reference and render |
| --- | --- |
| Composition | Open desktop form, still life at right, compact brand/header and small footer; no nested form card. |
| Artwork | A standalone text-free still life reproduces the ceramic vase, burgundy weave, brass object, ivory plinths and restrained gold edge motif. It is a separately generated decorative asset rather than a screenshot used as UI. |
| Typography and identity | Playfair headings, readable DM Sans controls, supplied woven artwork, consistent WeevCrafts casing and smaller wordmark. An inherited 11px important rule was discovered in screenshot review; bounded overrides restore 14px popup labels and 16px controls, with regression assertions. |
| Palette and controls | Existing forest/ivory tokens, dark green primary button, outlined Google button, persistent labels, visible keyboard focus and 50px controls. Input borders were darkened after contrast measurement. |
| Mobile | Shallow photo strip on the full page; centred popup, visible shopping context, close control, inline fields and both sign-in methods. |
| Copy | Matches the approved C direction plus the user's explicitly requested inline fields and Google additions. Existing consent/proof/error messages are preserved. |

Intentional practical differences: mobile full login scrolls naturally to keep fields and consent readable; short or keyboard-sized popups scroll internally. The generated decorative photograph is recreated as a standalone asset, so individual floral patterns differ. The gallery layout and customer controls retain the established site fonts rather than reproducing accidental serif body text in generated images. Default fields remain empty; reference example credentials are not prefilled. Desktop keeps the existing drawer pattern requested earlier.

## Verification

- `templ generate -path . -w 2`, `templ generate -path . -check`, `npm run assets:images`, `npm run css:build`, JavaScript syntax checks and `git diff --check` passed.
- `go test ./web/... ./internal/web/handlers/... ./internal/transport/web/...` passed. Page/handler/transport checks passed again after final template cleanup.
- Existing authentication discovery regressions passed in Chromium, Firefox and WebKit: public browsing, intent-only protected prompt, dismissal without shopping writes, login/register return paths, empty/invalid/pending/failed inputs, reveal, native fixture sign-in, provider fallback and reflow. Chromium also checked no-JavaScript forms and isolated fixture signup. One Firefox navigation timed out on the first run; the targeted engine rerun passed.
- New `tests/e2e/auth-gallery.spec.js`: **12 passed** across Chromium, Firefox and WebKit. Tests cover actual native fixture sign-in from the prompt, zero cart/wishlist mutations, close/reset/focus, short-screen scrolling, Google consent/return paths and scoped account linking. Google SDK/exchange responses are intercepted for these provider-flow tests; they do not prove a live Google account sign-in.
- Shared storefront header checks passed for guest and customer in all three engines at nine widths. An existing fractional-pixel comparison (43.999996px versus 44px) was corrected to round bounding boxes to hundredths before checking the intended 44px target.
- Air at `http://localhost:8090`: seven widths (320, 390, 768, 800, 801, 1120, 1365), actual enabled Google control and unchecked-consent guard, responsive artwork, phone prompt centring, short viewport (390×400), reduced motion, no browser errors and **zero data writes**. Three axe audits (phone login, desktop login, phone popup) reported zero violations. [Machine-readable report](woven-gallery-verification-2026-10-15.json).
- Codex's in-app browser displayed the live Air login and its actual provider controls. Responsive screenshots and the automated matrix use the existing Playwright tooling. Browser-tool telemetry produced unrelated network errors; application-page errors were tracked separately.

All account/session test writes were confined to the existing isolated UI QA services on ports 18080/15432/16380. Main Air volumes and catalogue were preserved. The Air catalogue remains empty; storefront background screenshots reflect that rather than inserting products.

Not verified locally: real Google account completion, physical mobile keyboard behaviour, screen readers and native browser zoom. Reduced-height and 320px checks provide supporting evidence, not device certification. Production deployment status and read-only smoke checks will be reported after the authorized GitHub push; this report records the completed local checks.

## Saved evidence

- [Desktop login, 1120×844](woven-gallery-login-1120-2026-10-15.png), [desktop login, 1365px](woven-gallery-login-1365-2026-10-15.png).
- [Mobile login, 390px](woven-gallery-login-390-2026-10-15.png), [mobile login, 320px](woven-gallery-login-320-2026-10-15.png).
- [Mobile popup, 390px](woven-gallery-popup-390-2026-10-15.png), [mobile popup, 320px](woven-gallery-popup-320-2026-10-15.png), [desktop prompt](woven-gallery-popup-1365-2026-10-15.png).

## Image production

Built-in Image Gen produced the updated concept and the standalone artwork. Artwork is stored at `web/assets/images/customer/woven-gallery-auth.webp` with proportional responsive variants. Original output: `C:/Users/Vinay/.codex/generated_images/01a13f63-b70e-7610-b988-c8534035ac52/exec-c5f76c24-d781-4b1d-96e4-60df1e36cd56.png`.

Final artwork brief: reproduce the approved gallery's still life as a single text-free portrait photograph: indigo floral ceramic vase, burgundy handloom cloth, small perforated aged brass vessel, copper bowl with airy stems, stepped ivory plinths, warm sunlight from upper left and a restrained double woven gold line at the right edge. Keep the left wall mostly plain warm white. No people, words, logos, UI, product labels, watermark or commercial claims. The concept-update brief preserved C's composition and added the user's inline username/password and Google controls with the existing unchecked consent step.
