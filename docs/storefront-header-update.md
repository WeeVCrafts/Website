# Customer header and navigation update

The Menu button sits at the left of the top row, with the logo and branding centered. The lower row is ordered delivery location, search, wishlist, cart, then profile. Narrow screens place delivery location above the search and shopping shortcuts. The hero and category cards retain their existing design.

Shop all, Categories, Offers and Our story are reached through Menu. Wishlist and cart are directly accessible in the header, with live counters and readable accessible names. The profile dropdown contains sign-in or account links. An uploaded customer photo replaces the profile icon. Opening the dropdown focuses its first link; Escape restores the profile button. Outside clicks and focus leaving close it. Shopping shortcuts, profile links and buttons have targets of at least 44px.

The navigation drawer groups shopping, account and help links, highlights the current destination, and uses a native modal dialog to isolate the background. Its links use 16px text, and delivery/menu labels use at least 14px. Keyboard focus cycles through its controls, returns to the trigger on closing, and resets the drawer scroll when reopening. Escape, the close button and backdrop clicks close it. The opening transition respects reduced motion.

## Verification

- `go test ./web/components ./web/pages` passed; generated components passed the templ freshness check. Preview styles were regenerated.
- `npm run test:storefront-header` passed against the rebuilt Docker application in Chromium, Firefox and WebKit for guest and customer sessions.
- Browser checks cover nine viewport widths from 320px to 1440px, zero/multi-digit/unavailable counts, clipped text, header overlap, touch targets, keyboard focus, scroll locking, closing controls, destination navigation and reduced motion. Sample count values are changed only in the test browser DOM.
- The application was rebuilt and recreated without resetting preview data. The local preview at <http://localhost:8080> has one customer, one seller owner and one superadmin. Private profile-photo upload and removal are tested against a separate QA database.

The repeatable browser check uses the local seeded customer account. `WEEVCRAFTS_BASE_URL`, `WEEVCRAFTS_CUSTOMER_EMAIL` and `WEEVCRAFTS_DEMO_PASSWORD` can override its connection and login. Set `WEEVCRAFTS_HEADER_SCREENSHOTS` to a directory to save optional screenshots; authentication requests are not recorded as traces.

## Quick customer check

1. Check the wishlist/cart numbers in the header. Upload a profile photo and check that it appears in the profile button after saving.
2. Use Menu to reach the shopping links. On mobile, check search and profile without horizontal scrolling.
3. Open each menu section, tab through its controls, and close it using Escape, the close button and the shaded backdrop. Reopen after scrolling: its heading should be visible again.
