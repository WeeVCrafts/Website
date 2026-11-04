# Secure customer experience update

## Implemented behavior

- Wishlist buttons retain the product slug when their action changes, reject invalid IDs before requests, synchronize saved state and labels, block simultaneous toggles and restore the previous state after a failed mutation. Product detail buttons now render the actual saved state.
- Customer sign-in defaults to `/products`; the API and server forms share the same destination rule and allow supported customer return paths such as checkout. Every role loses workspace switching; `/workspaces` redirects to its assigned portal.
- Accounts with missing, unknown or multiple roles receive an access-correction message and cannot access protected workflows. Existing assignments remain intact until Super Admin corrects them. Super Admin role updates replace the assigned role transactionally, revoke sessions, reconcile active staff memberships and audit the previous/new assignments. Protected Super Admin accounts retain their safeguards. Public registration grants customer only, and customer seller onboarding no longer grants a seller role.
- Customers can save name, phone and an optional private photo, preview/replace/remove it, and see save feedback. Name and phone determine completion; the notifications bell and derived reminder clear when saved details are valid. Email changes remain behind the existing verification workflow.
- Photos accept decoded JPEG, PNG or WebP up to 5 MiB and 16 megapixels, with dimension limits. The server resizes to at most 512 pixels per edge and re-encodes JPEG, removing original metadata. Opaque object keys stay private; the photo endpoint derives the owner from the session and uses `private, no-store`. Failed uploads preserve the saved profile. Replaced, removed and deleted-account photos enter durable cleanup, delayed five minutes to avoid racing a save and retried after storage failures.
- Super Admin can save optional HTTPS Instagram, Facebook, YouTube, X and WhatsApp links in Settings. Empty fields hide their links. Footer links have accessible names and safe external-link attributes. Other administrators cannot write these fields or erase them when saving unrelated settings.
- Customer screens retain the craft brand, use 16px body/form text and larger secondary text, increase touch targets and contrast, and add brief transitions that respect reduced motion. The 320px header and footer now fit without horizontal overflow.

## Security findings addressed

| Finding | Protection |
| --- | --- |
| Customer onboarding could acquire a seller role | Only already assigned seller owners can create their seller record; public customer registration remains customer |
| Additive role grants created combined privileges | Atomic replacement, membership reconciliation, database session cutoff and audit history |
| Existing ambiguous accounts could access direct workflow URLs | Shared protected-path guard plus portal authorization; correction requires Super Admin |
| New private profile data needed lifecycle handling | Owner-only retrieval, normalized images, private caching, phone clearing and durable file deletion |
| Privacy self-deletion could bypass protected Super Admin deletion rules | Protected administrators require the administrative deletion workflow |
| Account creation/deletion audits copied personal email | New audit entries retain IDs and administrative context without email copies |
| Local infrastructure published on all host interfaces | Every published Compose port binds to loopback |
| Production could inherit development configuration | Startup validation for TLS, cookies and credentials; demo flags disabled and helper routes unavailable |

Ownership and permission checks were reviewed across customer addresses, carts, wishlists, orders/receipts, returns, privacy requests, support attachments, seller catalog/staff/fulfilment, finance and administration. Existing queries scope records to the authenticated user or seller, and administrative services recheck roles. No bulk customer export endpoint is introduced. Regression coverage includes guessed identifiers and CSRF omissions. This is a review of these application paths, not a guarantee that no security defect can exist.

## Verification

Browser tests use the disposable `weev_qa_secure_20261009` PostgreSQL database and a separate Redis instance, so mutations do not change existing customer records. Screenshots contain synthetic QA data only; authentication traces are disabled so evidence does not capture credentials or session tokens.

| Check | Result |
| --- | --- |
| `go test ./...` and `go vet ./...` | Passed |
| Real PostgreSQL adapter and web-handler suites | Passed; includes photo-storage failure/retry, privacy deletion, minimal audits, role replacement and protected role correction |
| Chromium, Firefox and WebKit | 27 scenarios passed: 24 in the main run and 3 in a focused boundary retest after correcting a test URL |
| Wishlist | Add/remove/re-add, card/detail states, reload persistence, wishlist removal, Move to cart, rapid clicks, injected failure and no `undefined` requests passed |
| Profile and settings | Private upload/preview/replacement/removal, invalid/oversized files, missing CSRF, cross-account denial, reminder clearance, social persistence/validation and administrator boundaries passed |
| Roles and sessions | All eight single-role portals, legacy redirects, ambiguous account blocking, role replacement, suspension, password replacement and deletion revocation passed |
| Ownership | Two customers/two sellers, guessed orders/receipts, addresses, wishlists, private support attachments, seller writes, staff team permissions and finance boundaries passed |
| Customer layout/accessibility checks | Chromium/Firefox/WebKit at 1440, 720, 390 and 320 CSS pixels; no horizontal overflow, 16px body text, keyboard focus and reduced-motion configuration passed. 720px exercises the reflow available at 200% of a 1440px display; native browser zoom remains useful in manual review |
| Generated/static checks | Templates, preview CSS, JavaScript syntax, API YAML parsing, Compose configuration and `git diff --check` passed |

The database concurrency suite initially timed out while competing with a Docker build, then passed when rerun alone. Browser testing exposed the detail-heart state, WebKit file MIME handling and narrow-screen footer/header defects; those were corrected and retested. A foreign receipt now returns 404 instead of a misleading 503. The expanded boundary test also needed a valid archive reason and the actual finance route; its corrected scenarios passed in all three browsers.

Raw results: [main browser run](qa/secure-customer/final-browser-results.txt), [boundary retest](qa/secure-customer/boundary-retest.txt). The first file retains the test-URL failures rather than hiding them; the second records the successful focused rerun. These checks supplement the earlier UI audit and do not rewrite the larger manual campaign's execution tracker.

The original application database contained 96 users, 28 orders and 72 products before recreation. Its 12 active accounts with multiple roles and 4 active accounts without roles remain for Super Admin correction; no migration silently rewrites their assignments.

After the final Docker recreation those counts remained 96 / 28 / 72 / 12 / 4. The app readiness endpoint returned HTTP 200, and read-only smoke checks verified customer, seller-owner and Super Admin destinations, profile-photo controls, social settings and absence of workspace links in the main preview. All published ports are loopback-bound. Adminer still presents its password form; Kuma denied anonymous monitor access, authenticated successfully and reported all 9 monitors up. No volume reset or production deployment was performed.

The live QA cleanup worker removed the remaining queued test photos; the QA database ended with zero avatar references and zero cleanup entries. The two isolated QA containers are stopped to free resources, while their database/containers remain available for a repeat run on this machine:

```powershell
docker start weev-secure-qa-redis weev-secure-qa-app
$env:WEEVCRAFTS_BASE_URL='http://127.0.0.1:18082'
$env:WEEVCRAFTS_QA_DATABASE='weev_qa_secure_20261009'
npx playwright test -c tests/e2e/secure-customer.config.js
docker stop weev-secure-qa-app weev-secure-qa-redis
```

The guard in the browser suite refuses mutations against the main preview or any other database. Tests on a fresh machine require provisioning the same isolated seeded database and application/Redis instances first.

## Customer checklist

1. Sign in as a customer: the product listing should open. A checkout return path should still return to checkout.
2. Click a product heart twice, then add again. Reload and check the product detail heart, wishlist count, removal and Move to cart.
3. Open the notifications bell. If name or phone is missing, follow Complete your profile. Save name/phone and preview a photo; the reminder should disappear.
4. Replace and remove the photo, reload to verify persistence, and confirm no workspace-switch control appears.
5. Check products, profile, cart and notifications on mobile and with enlarged text/keyboard navigation. In Super Admin Settings, save a social link and verify the customer footer updates.

Production TLS ingress, private infrastructure access, encryption at rest, secret rotation, retention and encrypted backup restore verification remain deployment responsibilities. See [deployment security](deployment-security.md).
