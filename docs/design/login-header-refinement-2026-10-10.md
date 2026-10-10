# Login and sticky storefront header refinement — 10 October 2026

## Behavior

- `localhost:8080` is served by the current local Docker application. Its Firebase configuration is intentionally disabled because the local Firebase settings are empty.
- The login screen now keeps the Google sign-in option visible. In local mode, the button is disabled with an explicit explanation; configured Firebase environments retain the terms-gated popup flow.
- The first-party email path is explicitly labelled `Continue with email` and remains the working local sign-in path.
- The storefront header, search bar, actions, and navigation remain visible while scrolling. The layout remains overflow-free at a 390px viewport.

## Verification

- Chromium-only authentication regression passed: Google controls, terms guard, login errors, memory persistence, notification consent, and diagnostic opt-in.
- Chromium-only storefront regression passed: responsive widths, cart states, location modal keyboard/backdrop/navigation behavior, and reduced-motion behavior.
- Manual browser verification passed for the three local roles' sign-in surface, sticky header behavior, and customer email sign-in/sign-out.

Firefox and WebKit were not claimed because those local Playwright browser binaries are not installed in this workspace.

