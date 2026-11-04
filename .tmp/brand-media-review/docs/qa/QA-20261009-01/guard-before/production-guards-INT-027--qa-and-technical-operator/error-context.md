# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: production-guards.spec.js >> INT-027--qa-and-technical-operator
- Location: tests\e2e\production-guards.spec.js:5:1

# Error details

```
Error: Disabled development capture changed persisted order state

expect(received).toBe(expected) // Object.is equality

Expected: "pending_payment"
Received: "paid"
```

```
Error: Disabled development capture endpoint must not be registered

expect(received).toBe(expected) // Object.is equality

Expected: 404
Received: 200
```

# Page snapshot

```yaml
- generic [active] [ref=f1e1]:
  - banner [ref=f1e2]:
    - link "Skip to content" [ref=f1e3] [cursor=pointer]:
      - /url: "#page"
    - generic [ref=f1e4]:
      - generic [ref=f1e5]:
        - button "Open navigation menu" [ref=f1e6] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e7]: menu
        - link "Delivery location" [ref=f1e8] [cursor=pointer]:
          - /url: /account/addresses?notice=Delivery+location+opened
          - generic [aria-hidden] [ref=f1e9]: location_on
          - generic [ref=f1e10]: Deliver toBengaluru, Karnataka
      - link "Weevcrafts home" [ref=f1e11] [cursor=pointer]:
        - /url: /
        - generic [ref=f1e12]:
          - strong [ref=f1e13]: Weevcrafts
          - generic [ref=f1e14]: Crafted in India. Made to be cherished.
      - generic [ref=f1e15]:
        - generic [ref=f1e17]:
          - generic [ref=f1e18]: Search products
          - combobox "Search products" [ref=f1e19]
          - button "Search" [ref=f1e20] [cursor=pointer]:
            - generic [aria-hidden] [ref=f1e21]: search
        - navigation "Account actions" [ref=f1e22]:
          - link "Account" [ref=f1e23] [cursor=pointer]:
            - /url: /account
            - generic [aria-hidden] [ref=f1e24]: person
          - link [ref=f1e25] [cursor=pointer]:
            - /url: /wishlist
            - generic [aria-hidden] [ref=f1e26]: favorite
            - generic:
              - text: Wishlist
              - emphasis [ref=f1e27]: "1"
          - link [ref=f1e28] [cursor=pointer]:
            - /url: /cart
            - generic [aria-hidden] [ref=f1e29]: shopping_bag
            - generic:
              - text: Cart
              - emphasis [ref=f1e30]: "1"
          - button "Sign out" [ref=f1e32] [cursor=pointer]:
            - generic [aria-hidden] [ref=f1e33]: logout
  - main [ref=f1e34]:
    - generic [ref=f1e35]:
      - generic [ref=f1e36]:
        - paragraph [ref=f1e37]: YOUR WEEVCRAFTS HOME
        - heading [level=1] [ref=f1e38]:
          - text: Welcome back,
          - emphasis [ref=f1e39]: Priya Sharma
        - paragraph [ref=f1e40]: Glad to have you here. Together, we support artisans and keep traditions alive.
      - img "Artisan at work" [ref=f1e41]
    - generic [ref=f1e42]:
      - complementary [ref=f1e43]:
        - generic [ref=f1e44]:
          - generic [ref=f1e45]: PS
          - generic [ref=f1e46]: Priya Sharma
          - generic [ref=f1e47]: Customer · demo.customer01@weevcrafts.local
        - link "My account" [ref=f1e48] [cursor=pointer]:
          - /url: /account
          - generic [aria-hidden] [ref=f1e49]: dashboard
          - text: My account
        - link "Profile" [ref=f1e50] [cursor=pointer]:
          - /url: /account/profile
          - generic [aria-hidden] [ref=f1e51]: person
          - text: Profile
        - link "Addresses" [ref=f1e52] [cursor=pointer]:
          - /url: /account/addresses
          - generic [aria-hidden] [ref=f1e53]: location_on
          - text: Addresses
        - link "Reviews" [ref=f1e54] [cursor=pointer]:
          - /url: /account/reviews
          - generic [aria-hidden] [ref=f1e55]: reviews
          - text: Reviews
        - link "Orders" [ref=f1e56] [cursor=pointer]:
          - /url: /orders
          - generic [aria-hidden] [ref=f1e57]: inventory_2
          - text: Orders
        - link "Returns & refunds" [ref=f1e58] [cursor=pointer]:
          - /url: /returns
          - generic [aria-hidden] [ref=f1e59]: assignment_return
          - text: Returns & refunds
        - link "Payments" [ref=f1e60] [cursor=pointer]:
          - /url: /payments
          - generic [aria-hidden] [ref=f1e61]: payments
          - text: Payments
        - link "Wishlist" [ref=f1e62] [cursor=pointer]:
          - /url: /wishlist
          - generic [aria-hidden] [ref=f1e63]: favorite
          - text: Wishlist
        - link "Notifications" [ref=f1e64] [cursor=pointer]:
          - /url: /account/notifications
          - generic [aria-hidden] [ref=f1e65]: notifications
          - text: Notifications
        - link "Privacy" [ref=f1e66] [cursor=pointer]:
          - /url: /account/privacy
          - generic [aria-hidden] [ref=f1e67]: lock
          - text: Privacy
        - link "Security" [ref=f1e68] [cursor=pointer]:
          - /url: /account/security
          - generic [aria-hidden] [ref=f1e69]: shield
          - text: Security
        - link "Support" [ref=f1e70] [cursor=pointer]:
          - /url: /account/support
          - generic [aria-hidden] [ref=f1e71]: support_agent
          - text: Support
        - link "Switch workspace" [ref=f1e72] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e73]: apps
          - text: Switch workspace
      - generic [ref=f1e74]:
        - generic [ref=f1e75]:
          - generic [ref=f1e76]:
            - generic [aria-hidden] [ref=f1e77]: inventory_2
            - generic [ref=f1e78]: "1"
            - generic [ref=f1e79]: Orders placed
          - generic [ref=f1e80]:
            - generic [aria-hidden] [ref=f1e81]: favorite
            - generic [ref=f1e82]: "1"
            - generic [ref=f1e83]: Items in wishlist
          - generic [ref=f1e84]:
            - generic [aria-hidden] [ref=f1e85]: reviews
            - generic [ref=f1e86]: "0"
            - generic [ref=f1e87]: Reviews written
          - generic [ref=f1e88]:
            - generic [aria-hidden] [ref=f1e89]: volunteer_activism
            - generic [ref=f1e90]: Proud supporter
            - generic [ref=f1e91]: Indian artisans
        - generic [ref=f1e92]:
          - generic [ref=f1e93]:
            - heading "Profile information" [level=2] [ref=f1e94]
            - link "Edit" [ref=f1e95] [cursor=pointer]:
              - /url: /account/profile
              - text: Edit
              - generic [aria-hidden] [ref=f1e96]: edit
            - paragraph [ref=f1e97]: Priya Sharmademo.customer01@weevcrafts.localMember since 9 Oct 2026
          - generic [ref=f1e98]:
            - heading "Saved addresses" [level=2] [ref=f1e99]
            - link "Manage" [ref=f1e100] [cursor=pointer]:
              - /url: /account/addresses
              - text: Manage
              - generic [aria-hidden] [ref=f1e101]: arrow_forward
            - paragraph [ref=f1e102]: Demo Customer 011 Artisan Lane, Bengaluru, Karnataka, 560001, IN
          - generic [ref=f1e103]:
            - heading "Recent orders" [level=2] [ref=f1e104]
            - link "View all" [ref=f1e105] [cursor=pointer]:
              - /url: /orders
              - text: View all
              - generic [aria-hidden] [ref=f1e106]: arrow_forward
            - article [ref=f1e107]:
              - generic [ref=f1e108]:
                - generic [ref=f1e109]:
                  - paragraph [ref=f1e110]: Awaiting payment
                  - heading "WCDEMO1001" [level=2] [ref=f1e111]
                  - text: Placed on 9 Oct 2026 | Created
                - generic [ref=f1e112]:
                  - strong [ref=f1e113]: INR 2499
                  - generic [ref=f1e114]: Awaiting payment
              - generic [ref=f1e115]:
                - img "Default variant" [ref=f1e116]
                - generic [ref=f1e117]:
                  - text: Default variant
                  - generic [ref=f1e118]: 1 items
                - link "View details" [ref=f1e119] [cursor=pointer]:
                  - /url: /orders/WCDEMO1001
                  - text: View details
                  - generic [aria-hidden] [ref=f1e120]: arrow_forward
          - generic [ref=f1e121]:
            - heading "Returns & refunds" [level=2] [ref=f1e122]
            - link "View all" [ref=f1e123] [cursor=pointer]:
              - /url: /returns
              - text: View all
              - generic [aria-hidden] [ref=f1e124]: arrow_forward
            - paragraph [ref=f1e125]: View return requests linked to your orders.
          - generic [ref=f1e126]:
            - heading "Payment methods" [level=2] [ref=f1e127]
            - link "Manage" [ref=f1e128] [cursor=pointer]:
              - /url: /payments
              - text: Manage
              - generic [aria-hidden] [ref=f1e129]: arrow_forward
            - paragraph [ref=f1e130]: Payment options are shown securely during checkout. Card details are not stored in Weevcrafts.
          - generic [ref=f1e131]:
            - heading "Notification updates" [level=2] [ref=f1e132]
            - link "View updates" [ref=f1e133] [cursor=pointer]:
              - /url: /account/notifications
              - text: View updates
              - generic [aria-hidden] [ref=f1e134]: arrow_forward
            - paragraph [ref=f1e135]: Order and delivery updates appear in your order history.
          - generic [ref=f1e136]:
            - heading "Privacy preferences" [level=2] [ref=f1e137]
            - link "Manage" [ref=f1e138] [cursor=pointer]:
              - /url: /account/privacy
              - text: Manage
              - generic [aria-hidden] [ref=f1e139]: arrow_forward
            - paragraph [ref=f1e140]: Review your privacy choices and data requests.
          - generic [ref=f1e141]:
            - heading "Security & sessions" [level=2] [ref=f1e142]
            - link "Manage" [ref=f1e143] [cursor=pointer]:
              - /url: /account/security
              - text: Manage
              - generic [aria-hidden] [ref=f1e144]: arrow_forward
            - paragraph [ref=f1e145]: Change your password and protect your account.
          - generic [ref=f1e146]:
            - generic [aria-hidden] [ref=f1e147]: support_agent
            - generic [ref=f1e148]:
              - heading "Need a hand?" [level=2] [ref=f1e149]
              - paragraph [ref=f1e150]: Our customer care team is here for order, return and product questions.
            - link "Contact support" [ref=f1e151] [cursor=pointer]:
              - /url: /contact
  - generic [ref=f1e152]:
    - generic [ref=f1e153]:
      - generic [aria-hidden] [ref=f1e154]: groups
      - generic [ref=f1e155]: Crafted in India
      - generic [ref=f1e156]: Discover independent makers
    - generic [ref=f1e157]:
      - generic [aria-hidden] [ref=f1e158]: star
      - generic [ref=f1e159]: Customer reviews
      - generic [ref=f1e160]: Read published product reviews
    - generic [ref=f1e161]:
      - generic [aria-hidden] [ref=f1e162]: volunteer_activism
      - generic [ref=f1e163]: Artisan craft
      - generic [ref=f1e164]: Explore maker collections
    - generic [ref=f1e165]:
      - generic [aria-hidden] [ref=f1e166]: verified_user
      - generic [ref=f1e167]: Secure Payments
      - generic [ref=f1e168]: Status shown in your orders
    - generic [ref=f1e169]:
      - generic [aria-hidden] [ref=f1e170]: autorenew
      - generic [ref=f1e171]: Easy Returns
      - generic [ref=f1e172]: See the returns policy
    - generic [ref=f1e173]:
      - generic [aria-hidden] [ref=f1e174]: public
      - generic [ref=f1e175]: Domestic Delivery
      - generic [ref=f1e176]: To serviceable PIN codes
  - contentinfo [ref=f1e177]:
    - generic [ref=f1e178]:
      - generic [ref=f1e179]:
        - link "Weevcrafts Crafted in India. Made to be cherished." [ref=f1e180] [cursor=pointer]:
          - /url: /
          - generic [ref=f1e181]:
            - strong [ref=f1e182]: Weevcrafts
            - generic [ref=f1e183]: Crafted in India. Made to be cherished.
        - paragraph [ref=f1e184]: A marketplace for Indian arts, crafts and sarees.Supporting artisans. Preserving traditions.Building a brighter, more inclusive tomorrow.
        - generic [ref=f1e185]:
          - generic [aria-hidden] [ref=f1e186]: photo_camera
          - generic [aria-hidden] [ref=f1e187]: play_circle
          - generic [aria-hidden] [ref=f1e188]: alternate_email
          - generic [aria-hidden] [ref=f1e189]: language
      - generic [ref=f1e190]:
        - heading "Shop" [level=3] [ref=f1e191]
        - link "All Categories" [ref=f1e192] [cursor=pointer]:
          - /url: /categories
        - link "Arts" [ref=f1e193] [cursor=pointer]:
          - /url: /products?category=arts
        - link "Crafts" [ref=f1e194] [cursor=pointer]:
          - /url: /products?category=crafts
        - link "Sarees" [ref=f1e195] [cursor=pointer]:
          - /url: /products?category=sarees
        - link "Offers" [ref=f1e196] [cursor=pointer]:
          - /url: /products?category=offers
        - link "New Arrivals" [ref=f1e197] [cursor=pointer]:
          - /url: /products
      - generic [ref=f1e198]:
        - heading "Help" [level=3] [ref=f1e199]
        - link "Track Order" [ref=f1e200] [cursor=pointer]:
          - /url: /orders
        - link "Returns & Refunds" [ref=f1e201] [cursor=pointer]:
          - /url: /returns
        - link "Shipping Info" [ref=f1e202] [cursor=pointer]:
          - /url: /shipping
        - link "Size Guide" [ref=f1e203] [cursor=pointer]:
          - /url: /size-guide
        - link "FAQs" [ref=f1e204] [cursor=pointer]:
          - /url: /faq
        - link "Contact Us" [ref=f1e205] [cursor=pointer]:
          - /url: /contact
      - generic [ref=f1e206]:
        - heading "About" [level=3] [ref=f1e207]
        - link "Our Story" [ref=f1e208] [cursor=pointer]:
          - /url: /our-story
        - link "Makers" [ref=f1e209] [cursor=pointer]:
          - /url: /makers/mithila-arts
        - link "Sustainability" [ref=f1e210] [cursor=pointer]:
          - /url: /sustainability
        - link "Press" [ref=f1e211] [cursor=pointer]:
          - /url: /press
        - link "Careers" [ref=f1e212] [cursor=pointer]:
          - /url: /careers
        - link "Sell on Weevcrafts" [ref=f1e213] [cursor=pointer]:
          - /url: /seller
      - generic [ref=f1e214]:
        - heading "Email preferences" [level=3] [ref=f1e215]
        - paragraph [ref=f1e216]: Choose whether you want offers, new arrivals and maker stories.
        - link "Manage notification preferences" [ref=f1e217] [cursor=pointer]:
          - /url: /account/notifications
          - text: Manage notification preferences
          - generic [aria-hidden] [ref=f1e218]: arrow_forward
        - generic [ref=f1e219]: Sign in to save your choices. Email delivery depends on store email settings.
        - generic [ref=f1e220]: Made with care in India | A more handmade tomorrow
    - generic [ref=f1e221]:
      - generic [ref=f1e222]: Copyright 2024 Weevcrafts. All rights reserved.
      - generic [ref=f1e223]: Terms & Conditions | Privacy Policy | Cookie Policy
```

# Test source

```ts
  1  | const { test, expect } = require('@playwright/test');
  2  | const { execFileSync } = require('node:child_process');
  3  | 
  4  | const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
  5  | test('INT-027--qa-and-technical-operator', async ({ page }) => {
  6  |   test.skip(target.hostname !== '127.0.0.1' || target.port !== '18081', 'Requires the disposable production-configured QA guard instance.');
  7  |   await page.goto('/login');
  8  |   await page.locator('#page input[name="email"]').fill('demo.customer01@weevcrafts.local');
  9  |   await page.locator('#page input[name="password"]').fill('WeeVCrafts-Demo-2026!');
  10 |   await page.getByRole('button', { name: /sign in/i }).click();
  11 |   await page.waitForURL(/\/account\/?$/);
  12 |   const csrf = await page.locator('input[name="csrf_token"]').first().inputValue();
  13 |   const state = () => execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', 'weev_qa_20261009_guard', '-tAc', "SELECT status FROM orders WHERE order_number='WCDEMO1001'"], { encoding: 'utf8' }).trim();
  14 |   const before = state();
  15 |   for (const route of ['/api/v1/local/addresses', '/api/v1/local/settings']) {
  16 |     expect((await page.request.get(route)).status(), route).toBe(404);
  17 |   }
  18 |   const response = await page.request.post('/dev/payments/WCDEMO1001/capture', { headers: { 'X-CSRF-Token': csrf } });
  19 |   // Report the business effect as well as route status when the guard fails.
  20 |   expect.soft(state(), 'Disabled development capture changed persisted order state').toBe(before);
> 21 |   expect(response.status(), 'Disabled development capture endpoint must not be registered').toBe(404);
     |                                                                                             ^ Error: Disabled development capture endpoint must not be registered
  22 | });
  23 | 
```