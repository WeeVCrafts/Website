`flower.mp4` is the CC0 video sample served by the MDN interactive examples at:
https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4

The browser workflow uses it to verify MP4 upload, storage, customer playback markup, and media deletion.

`customer-layout/main.go` renders the real cart and checkout templates on loopback
port 18086 without a database. The layout suite stubs API responses and checks
mobile alignment, keyboard feedback, coupons, checkout navigation and cart actions.
Run from the repository root:

```sh
npx playwright test --config tests/e2e/customer-layout.config.js
```

Screenshots are written to the system temporary directory as `weev-customer-*.png`.
These fixtures do not verify database persistence or payment processing.
