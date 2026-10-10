# Customer header and navigation update

The count badges previously had a 17px height and inherited a 22.4px text line height, clipping the numbers. They now use a 24px minimum width and height, explicit 14px text with a unit line height, tabular numerals, and rounded ends for larger counts. Account actions have consistent 44px-wide touch targets. Search has more room on desktop and a separate row on smaller screens.

The Menu button has a visible label. Desktop customers can reach Shop all, Categories, Offers and Our story directly. The drawer groups shopping, account and help links, highlights the current destination, and uses a native modal dialog to isolate the background. Its links use 16px text, desktop navigation uses 15px, and delivery/menu labels use at least 14px, overriding a legacy global 11px rule. Keyboard focus cycles through its controls, returns to the trigger on closing, and resets the drawer scroll when reopening. Escape, the close button and backdrop clicks close it. The opening transition respects reduced motion.

## Verification

- `go test ./web/components ./web/pages` passed; generated components passed the templ freshness check. Preview styles were regenerated.
- `npm run test:storefront-header` passed against the rebuilt Docker application in Chromium, Firefox and WebKit for guest and customer sessions.
- Browser checks cover nine viewport widths from 320px to 1440px, zero/multi-digit/unavailable counts, clipped text, header overlap, touch targets, keyboard focus, scroll locking, closing controls, destination navigation and reduced motion. Sample count values are changed only in the test browser DOM.
- The application was rebuilt and recreated without recreating backend services or resetting volumes. It is healthy at <http://localhost:8080/products>. Database counts remained 96 users, 28 orders and 72 products.

The repeatable browser check uses the local seeded customer account. `WEEVCRAFTS_BASE_URL`, `WEEVCRAFTS_CUSTOMER_EMAIL` and `WEEVCRAFTS_DEMO_PASSWORD` can override its connection and login. Set `WEEVCRAFTS_HEADER_SCREENSHOTS` to a directory to save optional screenshots; authentication requests are not recorded as traces.

## Quick customer check

1. Check that wishlist/cart numbers are fully visible and comfortably separated from the icons.
2. Try the desktop shopping links and the labelled Menu button. On mobile, check search and account actions without horizontal scrolling.
3. Open each menu section, tab through its controls, and close it using Escape, the close button and the shaded backdrop. Reopen after scrolling: its heading should be visible again.
