# Weevcrafts manual test cases

Prepared on 9 October 2026 from source revision `e853611798973f9197efc007357f19b1bbc5e261`.

This suite contains **373 distinct test cases** and **754 execution rows**. It covers all eight application roles, guest access, shared controls, complete journeys across roles, integrations, and release requirements. Cases use normal sentences and state what to do and what result to expect.

**Execution status: Partially executed on 9 October 2026.** See the [execution report](qa/QA-20261009-01/report.md) and [execution tracker](test-execution-tracker.csv) for observed results, build identity, defects, blockers, and unexecuted rows. Related automated coverage does not establish that every manual procedure passed. This document remains the procedure.

The expected results are acceptance criteria, not a claim that the current code already meets them. An unavailable page, a decorative control, a simulated integration, or an unsaved change must not be marked Passed merely because it looks complete.

## Contents

- [Preparation and fixtures](#preparation-and-fixtures).
- [Role coverage and permissions](#role-coverage-and-permissions).
- [Execution and evidence rules](#execution-and-evidence-rules).
- [Case inventory](#case-inventory).
- [Sign-in, account recovery, and sessions](#auth-cases).
- [Workspace navigation and access boundaries](#nav-cases).
- [Guest and public storefront](#gst-cases).
- [Customer shopping and account workflows](#cus-cases).
- [Seller owner workflows](#own-cases).
- [Seller staff permissions](#stf-cases).
- [Marketplace administration](#mkt-cases).
- [Super Admin workflows](#sup-cases).
- [Finance operator workflows](#fin-cases).
- [Returns and operations workflows](#ops-cases).
- [Support agent workflows](#agt-cases).
- [Usability, accessibility, and browser checks](#ui-cases).
- [Complete workflows across roles](#e2e-cases).
- [Service, payment, persistence, and recovery checks](#int-cases).
- [Requirements and release gaps](#gap-cases).
- [Execution order and acceptance](#execution-order-and-acceptance).
- [Existing automated coverage](#existing-automated-coverage).

## Preparation and fixtures

### Prepare the environment

1. Use a disposable local or staging environment for state changes, deletion, concurrent checkout, dependency interruption, and restoration. Give each run a unique name such as QA-20261009-01.
2. Record the application URL, deployed revision, payment mode, local-mode setting, database migration version, and test date before testing. The reviewed revision above is not evidence that a deployed build was tested.
3. Start the documented local stack and seed accounts using [README.md](../README.md) and [demo-accounts.md](demo-accounts.md). The seed is insert-only; it does not reset existing passwords, permissions, or business records.
4. Use full email addresses, such as `demo.customer01@weevcrafts.local`. The username shorthand in [role-workflows.md](role-workflows.md) is not a complete local sign-in email.
5. Keep PostgreSQL and Redis available. Configure persistent product-cover storage, additional-media object storage, SMTP, and workers for the workflows that need them. Use the local mailbox at `http://localhost:8025` when running the documented local stack.
6. For payment integration cases, use the actual payment provider sandbox, verified callback or webhook configuration, and evidence access to provider records. A local mock capture may prepare local workflows but cannot pass a provider integration case.
7. Keep every participant in a separate browser profile or private context. Do not reuse one signed-in tab for multiple actors during ownership tests.
8. Ask the test environment owner to provide read-only database and operational evidence when a case checks persistence, stock reservations, ledger entries, audits, or worker events. A tester can perform ordinary workflows through the interface; a technical operator can assist with altered requests, controlled clocks, and dependency failures.
9. Record the configured password limits, session expiry, lockout thresholds, reservation duration, accepted report periods, browser support, and performance budgets. Do not invent values when the environment has not supplied them.
10. If a prerequisite is unavailable, mark the affected execution Blocked, state what is missing, and identify the next action. Do not replace an integrated result with a mock result.

### Prepare accounts

Use disposable copies when changing credentials, suspending an account, revoking roles, or deleting data.

| Fixture | Account example or preparation | Purpose |
| --- | --- | --- |
| Customer A | `demo.customer01@weevcrafts.local` | Primary shopping and account workflows. |
| Customer B | `demo.customer02@weevcrafts.local` | Foreign ownership checks. |
| Seller A owner | `demo.owner01@weevcrafts.local` | Primary seller workflows. |
| Seller B owner | `demo.owner02@weevcrafts.local` | Seller isolation checks. |
| Seller A staff | `demo.staff01@weevcrafts.local` | Permission checks after confirming its seller membership. |
| Marketplace admin | `demo.marketplace01@weevcrafts.local` | Seller, product, directory, campaign, and review decisions. |
| Super Admin A and B | `demo.super01@weevcrafts.local` and `demo.super02@weevcrafts.local` | Account management and administrator protection. |
| Support agent | `demo.support01@weevcrafts.local` | Customer and seller conversations. |
| Finance operator | `demo.finance01@weevcrafts.local` | Financial records and local settlement workflows. |
| Returns operations | `demo.operations01@weevcrafts.local` | Return decisions and receipt. |
| Multiple-role account | Grant two supported roles to a disposable account. | Workspace selection and role revocation. |
| No-workspace account | Prepare a disposable account without recognized assignments. | Safe handling without implicit privileges. |
| Pending and suspended users | Create dedicated accounts and sellers for these states. | Onboarding, suspension, and denial checks. |

The local-only password is documented in [demo-accounts.md](demo-accounts.md). Confirm each account's current assignments instead of assuming the seed reset them. Seller staff tests require several disposable accounts or deliberate permission changes between runs.

### Prepare business records

| Fixture | Required preparation |
| --- | --- |
| Products | Create Seller A and Seller B products in draft, pending review, approved, rejected, and archived states. Give each record a unique run-prefixed slug and SKU. |
| Variants | Include two distinct prices and SKUs, an archived variant, a zero-stock variant, a low-stock variant, and a dedicated variant with exactly ten units for concurrency tests. |
| Directories | Prepare used and unused categories and brands, including enough rows to exercise pagination. |
| Addresses | Prepare separate Customer A and Customer B addresses. A valid example is a named recipient at 12 Test Lane, Pune, Maharashtra, 411001, India, with a test phone containing ten digits. |
| Coupons | Prepare fixed and percentage campaigns that are draft, active, paused, archived, expired, scheduled for the future, exhausted, and limited to one usage. Use controlled dates for boundary checks. |
| Orders | Create independent pending, paid, processing, shipped, delivered, cancelled, and refunded orders. Include an order owned by Customer B and an order with items from both sellers. |
| Returns | Prepare pending, approved, rejected, and received returns. A current return requires the owning customer's delivered order. |
| Payments | Prepare genuine sandbox capture, failure, pending, invalid-signature, duplicate-event, and mismatched-event scenarios. Keep local simulated payment evidence separate. |
| Refunds | Prepare received-return orders with captured payments for processed, pending, failed, and retry scenarios. The current refund endpoint uses the owning customer, not an operator impersonating that customer. |
| Finance | Prepare open seller ledger entries and pending, approved, paid, and failed settlements. Keep a simple goods-only price example of INR 500.00 with a 10 percent commission for calculation checks when that base matches the approved rule. |
| Support | Prepare customer and seller cases, one foreign case, public replies, internal notes, order links, and image attachments. |
| Reviews | Prepare delivered purchases without reviews, pending reviews, published reviews, rejected reviews, and foreign purchases. |
| Files | Prepare genuine PNG, JPEG, WebP, MP4, and WebM files, unsupported files, empty files, renamed non-images, and files just below, at, and above upload limits. |
| Privacy | Prepare disposable accounts with addresses, cart items, wishlist entries, consents, sessions, and orders. Create separate access, correction, deletion, and grievance requests. |

Create prerequisite states through supported workflows whenever possible. Technical fixtures may set controlled states in an isolated environment, but record that setup. Never count a manually changed database row as proof that the corresponding user workflow works.

Product covers and additional media have a 25 MiB limit in the reviewed code. Covers accept PNG, JPEG, and WebP. Additional media also accepts MP4 and WebM. Support attachments accept up to three PNG, JPEG, or WebP images, each up to 10 MiB. Test real content as well as file extensions.

Use separate records for mutually exclusive paths. For example, do not reuse one delivered order for approved, rejected, and received-return journeys. Preserve the failing record for diagnosis before removing run-specific fixtures.

### Browser and accessibility matrix

Run critical journeys at widths of 360, 390, 768, 1024, and 1440 pixels. Test Chromium, Firefox, and WebKit unless the release owner supplies a different approved support list. Record actual versions and devices.

The existing Playwright configuration runs Chromium only. It does not establish Firefox, WebKit, screen-reader, zoom, or physical-device coverage. Run those checks separately and record missing coverage.

## Role coverage and permissions

The application has eight signed-in roles. Guest access is an additional public testing context. An account can have multiple explicit assignments; test each assignment separately.

| Role | Workspace | Workflow checklist |
| --- | --- | --- |
| Customer | `/account` | Profile, addresses, notifications, security and sessions, privacy, reviews, support, storefront, cart, wishlist, checkout, order details, receipts, returns, return documents, and customer-owned refunds. |
| Seller owner | `/seller-admin` | Dashboard, onboarding, settings, products and variants, cover and additional media, inventory and low-stock thresholds, order details and fulfillment, returns visibility, earnings tabs, analytics, team, owner support, and terms. |
| Seller staff | `/seller-admin` | Seller shell plus the precise catalog, inventory, order, return, and report grants. Test every grant alone, useful combinations, no grants, revocation, and foreign seller records. |
| Marketplace admin | `/marketplace-admin` | Seller applications and status, product moderation, inventory oversight, categories, brands, customer directory, orders, returns, support oversight where authorized, reviews, campaigns, analytics, audit, and permitted public settings. |
| Super Admin | `/super-admin` | All permitted marketplace administration through this workspace, operational users and roles, account suspension, protected administrator state, platform settings, audit and security records, dependency health, product creation for sellers, commissions, and finance oversight. |
| Support agent | `/support-portal` | Dashboard, cases, customer and seller context, linked orders, public replies, private notes, statuses, attachments, history, filters, and denied financial or security writes. |
| Finance operator | `/finance-portal` | Payments, failed payments, refunds, reconciliation, commission rules, settlement batches and decisions, ledger consistency, reports where implemented, and denied catalog or security changes. |
| Returns operations | `/operations-portal` | Dashboard, orders, returns, approval, rejection, receipt, refund visibility, dispute views where implemented, and denied finance or security changes. |

For each checklist item, open the page through navigation and directly, use every exposed meaningful control, refresh it, and confirm the durable result. If a page uses a tab or query-selected workspace instead of a dedicated URL, test that selection as well. Record every unsupported or ineffective control against its owning case.

A single-role account must be denied other workspace roots unless it receives another explicit assignment. For example, Super Admin administers finance under its own workspace; that assignment alone does not grant `/finance-portal`, `/support-portal`, or `/seller-admin`.

The expected page boundary is:

| Account assignment | Customer account | Seller workspace | Marketplace workspace | Super Admin workspace | Support workspace | Finance workspace | Operations workspace |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Customer only | Allow | Deny | Deny | Deny | Deny | Deny | Deny |
| Seller owner only | Deny | Allow | Deny | Deny | Deny | Deny | Deny |
| Seller staff only | Deny | Scope by permission | Deny | Deny | Deny | Deny | Deny |
| Marketplace admin only | Deny | Deny | Allow | Deny | Deny | Deny | Deny |
| Super Admin only | Deny | Deny | Deny | Allow | Deny | Deny | Deny |
| Support agent only | Deny | Deny | Deny | Deny | Allow | Deny | Deny |
| Finance operator only | Deny | Deny | Deny | Deny | Deny | Allow | Deny |
| Returns operations only | Deny | Deny | Deny | Deny | Deny | Deny | Allow |

Run denial checks against direct writes and older routes as well as page reads. The documented boundary is an acceptance target; any observed bypass is a defect.

### Seller staff permission matrix

| Exact permission | Positive check | Required denial check |
| --- | --- | --- |
| `PRODUCT_READ` | Read assigned seller products. | Deny catalog writes without `PRODUCT_WRITE`. |
| `PRODUCT_WRITE` | Create and change assigned seller products, variants, and media. | Deny foreign catalog writes and marketplace approval. |
| `INVENTORY_READ` | Read assigned seller stock. | Deny stock changes without `INVENTORY_WRITE`. |
| `INVENTORY_WRITE` | Change assigned seller stock through supported controls. | Deny foreign stock and invalid negative availability. |
| `ORDER_READ` | Read assigned seller orders and details. | Deny fulfillment writes without `ORDER_FULFILL`. |
| `ORDER_FULFILL` | Update eligible assigned seller fulfillment. | Deny foreign and invalid order transitions. |
| `RETURN_READ` | Read relevant seller returns. | Deny approval, receipt, and financial refunds through this grant. |
| `REPORT_READ` | Read permitted assigned seller reports. | Deny commission changes and payout decisions. |

The reviewed permission list does not include `RETURN_PROCESS`, although the PRD gives it as an example. Use GAP-004 if return processing by staff is part of the accepted release scope. Team management, owner settings mutations, and owner support remain owner-only.

### Super Admin reuse checklist

Run MKT-001 through MKT-009 and MKT-011 through MKT-031 again as Super Admin through `/super-admin`. Do not reuse MKT-010 or MKT-032, because their marketplace-specific restrictions differ.

Run FIN-001 through FIN-018 and FIN-020 again through Super Admin's own finance controls. Do not reuse FIN-019, because Super Admin has catalog and security administration.

When reusing a case, substitute the tested role's session and its permitted workspace root. Preserve all business, ownership, validation, and audit expectations. The tracker contains separate Super Admin rows for these repeats. SUP-017 and SUP-019 are overall workflow checks, not substitutes for these individual rows.

## Execution and evidence rules

- **P0** covers access boundaries, payment and inventory integrity, critical order journeys, privileged changes, privacy deletion, and essential release integrations.
- **P1** covers complete business workflows, validation, recovery, accessibility, and important compatibility checks.
- **P2** covers lower-risk presentation and policy navigation checks.

Each case gives preconditions, a normal-sentence procedure, and an expected result. For a case containing several input values or transitions, test each listed variation and record the observed outcomes in the Actual result field. Mark the case Failed if any required variation fails. If variations or browsers need independent evidence, duplicate the tracker row and extend its Execution ID with the variation or browser.

For AUTH, NAV, and UI, record a separate result for every listed role. Do not mark the remaining roles Passed from one account's result. The tracker already expands those shared cases by role.

Use these result values:

| Status | Meaning |
| --- | --- |
| Not run | No execution evidence exists yet. |
| Passed | Every required assertion was observed on the recorded build and environment. |
| Failed | One or more expected results were not met. Link the defect and observed evidence. |
| Blocked | A prerequisite prevents completion. Record the missing prerequisite and next action. |
| Not applicable | The release owner has explicitly excluded the requirement or variation. Record the decision and reason. |

An unavailable required feature is Failed or Blocked, not automatically Not applicable. An optional PRD feature can be excluded only by a recorded scope decision. A blocked or failed result never counts as Passed.

Record the tested account alias, record identifiers, browser or client, build, environment, test date, tester, actual result, evidence location, and defect reference in the CSV. Do not store passwords, active session cookies, recovery links, provider secrets, or sensitive personal data in evidence. If an environment failure interrupts a case after a write, inspect the stored result before retrying.

For every meaningful saved change, refresh the page and reopen it in a fresh authorized session. Where a case names multiple roles, inspect the affected record from each role. For financial, inventory, and audit assertions, retain authoritative evidence rather than relying only on a success message.

After a fix, rerun the failed variation, the affected role workflow, and its relevant cross-role journey. Record retest evidence against the same case and defect. Keep the prior failure evidence.

## Case inventory

| Group | Distinct cases | Execution rows |
| --- | --- | --- |
| AUTH: Sign-in, account recovery, and sessions | 16 | 128 |
| NAV: Workspace navigation and access boundaries | 12 | 96 |
| GST: Guest and public storefront | 20 | 20 |
| CUS: Customer shopping and account workflows | 61 | 61 |
| OWN: Seller owner workflows | 53 | 53 |
| STF: Seller staff permissions | 22 | 22 |
| MKT: Marketplace administration | 32 | 62 |
| SUP: Super Admin workflows | 24 | 24 |
| FIN: Finance operator workflows | 20 | 39 |
| OPS: Returns and operations workflows | 16 | 16 |
| AGT: Support agent workflows | 22 | 22 |
| UI: Usability, accessibility, and browser checks | 17 | 153 |
| E2E: Complete workflows across roles | 15 | 15 |
| INT: Service, payment, persistence, and recovery checks | 33 | 33 |
| GAP: Requirements and release gaps | 10 | 10 |
| Total | 373 | 754 |

Shared checks cover all assigned roles. Marketplace and finance groups include the explicitly listed Super Admin repeats. Application checks use the running product; Integrated environment checks require real configured boundaries; Release requirement checks close accepted scope and provider gaps.


<a id="auth-cases"></a>

## Sign-in, account recovery, and sessions

Default context: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

Before this group: Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

Source references: [internal/application/auth/](../internal/application/auth/), [internal/transport/web/auth_recovery.go](../internal/transport/web/auth_recovery.go), [internal/transport/web/handler.go](../internal/transport/web/handler.go), [tests/e2e/portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js).

### AUTH-001: Sign in with valid credentials

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Open the sign-in page, enter the role account's email and password, and submit the form. Refresh the destination page.

**Expected result:** The account reaches its assigned workspace and remains signed in after refresh. The displayed identity and role match the account.

### AUTH-002: Reject an incorrect password

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Enter the correct email with an incorrect password and submit the form. Try opening the protected workspace directly.

**Expected result:** Sign-in fails without revealing whether the email exists. The protected workspace remains inaccessible.

### AUTH-003: Reject an unknown account

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Sign in with an email that has never been registered and a plausible password.

**Expected result:** The application shows a generic authentication failure. It does not create an account or session.

### AUTH-004: Validate empty sign-in fields

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Submit the sign-in form with both fields empty, then with each field empty separately.

**Expected result:** Required fields receive clear validation feedback. No authenticated session is created.

### AUTH-005: Handle repeated failed sign-in attempts

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** In an isolated test environment, submit incorrect passwords until the configured rate limit or lockout applies. Retry after the configured recovery period.

**Expected result:** Attempts are limited according to the configuration. The account recovers as designed, and failed attempts leave security evidence.

### AUTH-006: Block a suspended account

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Have Super Admin suspend a disposable role account, then attempt sign-in and access from an existing session.

**Expected result:** The suspended account cannot start or continue protected work. No business change is accepted.

### AUTH-007: Keep the redirect destination safe

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Open sign-in with a permitted internal destination, then repeat with an external URL, a double-slash URL, and a backslash URL.

**Expected result:** Permitted internal destinations work. Unsafe destinations are ignored, and sign-in never redirects to an external site.

### AUTH-008: Request password recovery

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Request password recovery for the role account and inspect its test mailbox. Open the recovery link.

**Expected result:** The account receives a usable recovery link without exposing its token in ordinary pages or logs. The recovery page identifies the required action.

### AUTH-009: Prevent account enumeration during recovery

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Request recovery for an existing address and an unknown address. Compare the visible responses.

**Expected result:** Both requests receive equivalent generic feedback. Only the existing account receives an email.

### AUTH-010: Reset a password and revoke sessions

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Keep two sessions open, reset the disposable account's password through its email link, and retry both sessions. Sign in with the old and new passwords.

**Expected result:** Existing sessions lose access. The old password fails, and the new password signs in successfully.

### AUTH-011: Reject invalid recovery links

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Use an altered recovery token, an expired token, and a previously used token. Attempt a reset with each token.

**Expected result:** Every invalid token is rejected without changing credentials. A successful recovery link cannot be used twice.

### AUTH-012: Validate a new password

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Attempt a password reset below the configured minimum, at the accepted minimum, and above the configured maximum. Repeat with mismatched confirmation fields where provided.

**Expected result:** Invalid values receive clear feedback. A valid password works, and rejected submissions do not consume a usable token unexpectedly.

### AUTH-013: Sign out securely

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Sign out from the role workspace, refresh it, use the browser Back button, and attempt a protected action.

**Expected result:** The server session is invalidated. Cached page content cannot be used to read fresh protected data or submit changes.

### AUTH-014: Expire an inactive session

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Use the test configuration to let a role session expire. Submit an already open form and navigate to another protected page.

**Expected result:** The application requires sign-in again. The expired session cannot authorize the form submission.

### AUTH-015: Isolate simultaneous accounts

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Sign in as two different role accounts in separate browser profiles. Change data in one profile and inspect both identities.

**Expected result:** Each session retains its own identity and permissions. One account cannot acquire the other account's session or records.

### AUTH-016: Reject missing or incorrect request tokens

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use a separate account and browser session for each of the eight roles. Keep a second session for revocation tests and use disposable accounts for password changes.

**Test steps:** Submit a representative protected form without its request token, with another session's token, and with an altered token.

**Expected result:** Every invalid submission is denied. The target record and financial state remain unchanged.

<a id="nav-cases"></a>

## Workspace navigation and access boundaries

Default context: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

Before this group: Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

Source references: [internal/application/auth/portal.go](../internal/application/auth/portal.go), [internal/web/handlers/portal_routes.go](../internal/web/handlers/portal_routes.go), [web/workspaces/workspaces.templ](../web/workspaces/workspaces.templ), [tests/e2e/role-portals.spec.js](../tests/e2e/role-portals.spec.js).

### NAV-001: Open every permitted workspace page

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Open every permitted page listed in the role's coverage checklist. Follow each visible navigation item and primary action.

**Expected result:** Every permitted page loads real data or an honest empty state. Links and actions reach the correct workflow.

### NAV-002: Deny another role's workspace

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Enter each other workspace root directly, using a role account that has only one assignment.

**Expected result:** Unauthorized workspaces return an access denial. Super Admin does not inherit separate seller, support, or finance workspace assignments merely through its name.

### NAV-003: Deny protected actions directly

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Copy an action request from an authorized account and submit it with the tested role's session. Repeat through canonical and legacy routes.

**Expected result:** The server rejects every action outside the role's permissions. Hiding a button is not the only access control.

### NAV-004: Protect query-selected workspaces

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Add workspace query parameters for roles, settlements, commissions, and security to pages available to the tested role.

**Expected result:** Query parameters cannot bypass permissions or expose restricted data. The server enforces the selected workflow's authorization.

### NAV-005: Use the workspace chooser

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Sign in with the account that has two roles, open the workspace chooser, and select each assigned workspace.

**Expected result:** The chooser lists only assigned workspaces. Each choice opens the correct workspace with its own navigation.

### NAV-006: Avoid duplicate seller workspaces

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Use an account with both seller owner and seller staff assignments. Open the workspace chooser.

**Expected result:** The seller workspace appears once with owner access. Other assigned workspaces remain available.

### NAV-007: Handle an account without a workspace

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Sign in with a disposable account that has no recognized role assignment.

**Expected result:** The application shows an honest workspace selection or access state. It does not grant a default privileged role.

### NAV-008: Preserve permitted legacy links

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Open permitted older seller, admin, support, and finance links with search or page parameters.

**Expected result:** The application redirects to the appropriate current workspace while preserving useful parameters. Restricted legacy links remain denied.

### NAV-009: Retain page context after refresh

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Apply a supported search, filter, tab, or page selection and refresh the page. Use browser Back and Forward.

**Expected result:** The visible state agrees with the URL and saved data. Navigation does not repeat a previous write.

### NAV-010: Handle unavailable records

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Open a detail URL for a nonexistent record, a removed record, and a malformed identifier.

**Expected result:** The application shows a controlled not-found or validation response. It does not expose internal errors or another account's record.

### NAV-011: Remove access after a role change

Priority: P0. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Keep the role account signed in while Super Admin revokes its assignment. Retry an open action and sign in again.

**Expected result:** Revoked access cannot continue. New sign-in and workspace choices reflect the current assignments.

### NAV-012: Show honest dashboard totals

Priority: P1. Test mode: Application. Run as: Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Sign in separately as each listed role. Prepare an account with two assigned roles and a seller staff account with limited permissions.

**Test steps:** Create a known record in the role's scope and refresh the dashboard and its linked list.

**Expected result:** Totals reflect the authoritative records and the role's scope. A failed data load does not appear as a healthy zero.

<a id="gst-cases"></a>

## Guest and public storefront

Default context: Guest.

Before this group: Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

Source references: [internal/web/handlers/live_storefront.go](../internal/web/handlers/live_storefront.go), [internal/transport/web/discovery_handlers.go](../internal/transport/web/discovery_handlers.go), [web/pages/](../web/pages/), [internal/application/catalog/](../internal/application/catalog/).

### GST-001: Load the storefront

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open the home page in a fresh browser and follow its product and category links.

**Expected result:** The home page loads usable navigation and approved catalog data. Product images and primary links work.

### GST-002: Browse a category

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open two category pages and inspect their product lists. Clear the category selection.

**Expected result:** Each category shows matching approved products. Clearing the selection returns to the broader catalog.

### GST-003: Search for an existing product

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Search using a product name, a partial name, and a word from its description.

**Expected result:** Relevant approved products appear. Search does not expose drafts, rejected listings, or archived products.

### GST-004: Use search suggestions

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Type a partial product name and select a displayed suggestion. Repeat with no matching text.

**Expected result:** Suggestions lead to the intended product or search result. No-match input produces a clear state without stale suggestions.

### GST-005: Handle an empty search

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Submit an empty search and a search containing only spaces.

**Expected result:** The application handles the input consistently and shows a useful catalog or validation state.

### GST-006: Handle no search results

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Search for a unique term that matches no product.

**Expected result:** A clear no-results message appears. Navigation and search remain usable.

### GST-007: Filter and sort results

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Apply every visible category and price filter. Select each visible sort option and clear the filters.

**Expected result:** Results satisfy the selected filters and the displayed ordering. Unsupported controls are recorded as a gap instead of assumed to work.

### GST-008: Navigate result pages

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Use a catalog with more than one page. Move forward and backward, then open the URL in another browser.

**Expected result:** Pages contain the correct records without unintended duplicates. The copied URL preserves the supported search state.

### GST-009: Open product details

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open an approved product and compare its name, variant, price, stock, seller, delivery information, and return information with the listing.

**Expected result:** Details match the stored catalog. The selected variant determines the price and availability shown for purchase.

### GST-010: Use product media

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Select each image thumbnail and play an available product video.

**Expected result:** The selected media opens correctly. Broken media shows an honest failure state instead of an unrelated image.

### GST-011: Browse a seller store

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open a maker or seller link, inspect its approved products, and apply its available sort options.

**Expected result:** The page identifies the correct seller. Its catalog contains that seller's public products.

### GST-012: Browse brands and deals

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open the brand directory, a brand page, and the deals page. Compare their products with the configured catalog and active campaigns.

**Expected result:** Directories and deals show matching public records. Expired, paused, or unpublished data does not appear as an active offer.

### GST-013: Create a guest cart

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Add an in-stock variant, refresh the cart, and add a second product.

**Expected result:** The guest cart retains both selected variants and correct quantities. Totals agree with current server prices.

### GST-014: Change a guest cart

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Increase and decrease a quantity, remove one line containing two units, and remove the final line.

**Expected result:** Quantities and totals update correctly. Remove deletes the entire selected line, and the empty cart shows a useful message.

### GST-015: Reject invalid guest quantities

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Attempt zero, negative, nonnumeric, and excessive quantities through the cart form or its request.

**Expected result:** Invalid quantities are rejected. Stock and cart totals cannot become negative or exceed allowed limits.

### GST-016: Prevent unavailable purchases

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Try adding an out-of-stock variant and opening draft, archived, and rejected product URLs.

**Expected result:** Unavailable items cannot be purchased. Nonpublic details are not disclosed through guessed URLs.

### GST-017: Continue a guest cart after sign-in

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Create a guest cart, sign in as a customer with an existing cart, and inspect the merged cart. Repeat sign-in without adding new items.

**Expected result:** The merge preserves valid items according to stock limits. Repeating the merge does not duplicate quantities.

### GST-018: Require an account for protected actions

Priority: P0. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Attempt final checkout, a review, a wishlist change, and protected account pages while signed out.

**Expected result:** Protected actions require the appropriate account. Guest requests do not create unauthorized orders or personal records.

### GST-019: Read information and policy pages

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open shipping, returns, privacy, contact, help, and every public footer link.

**Expected result:** Pages load meaningful current content. Displayed contact and shipping information agree with marketplace settings.

### GST-020: Reject unknown public URLs

Priority: P1. Test mode: Application. Run as: Guest.

**Before you start:** Use a browser with no authenticated session. Prepare approved products from two active sellers, unpublished products, an out-of-stock variant, and an archived listing.

**Test steps:** Open nonexistent product, category, brand, and general page URLs.

**Expected result:** The application returns a controlled not-found response. It does not substitute an unrelated product as success.

<a id="cus-cases"></a>

## Customer shopping and account workflows

Default context: Customer.

Before this group: Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

Source references: [internal/web/handlers/live_storefront.go](../internal/web/handlers/live_storefront.go), [internal/transport/web/commerce_handlers.go](../internal/transport/web/commerce_handlers.go), [internal/transport/web/privacy_handlers.go](../internal/transport/web/privacy_handlers.go), [internal/application/commerce/](../internal/application/commerce/), [tests/e2e/portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js).

### CUS-001: Register a customer account

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Sign out and register a new disposable email with valid account details. Sign in with the new credentials.

**Expected result:** Exactly one customer account is created. The account receives customer access without a privileged role.

### CUS-002: Validate registration data

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Register with an invalid email, missing display name, and a password outside the configured limits. Repeat with an existing email.

**Expected result:** Invalid submissions do not create accounts. Duplicate registration does not overwrite the existing credentials or roles.

### CUS-003: Verify an email address

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Request verification for an unverified disposable customer, open its email link, and complete verification.

**Expected result:** The verified state persists after sign-in and refresh. Verification does not create another account.

### CUS-004: Reject invalid verification links

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit an altered verification link, an expired link, and a link that was already used.

**Expected result:** Invalid links do not change verification state. The application explains how to obtain a fresh link.

### CUS-005: Save the customer profile

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Change the display name, save it, refresh the account, and sign in from another browser.

**Expected result:** The new name persists and appears consistently. Customer B's profile remains unchanged.

### CUS-006: Validate profile changes

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit a blank name, whitespace-only text, and text above the configured maximum.

**Expected result:** Invalid input receives clear feedback. The previous valid name remains stored.

### CUS-007: Create an address

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Create an Indian address with recipient name, phone, street, city, state, and six-digit postal code. Reopen the address list.

**Expected result:** The address is saved once and belongs to Customer A. Its fields are available for checkout.

### CUS-008: Edit an address

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Change a saved address and reopen checkout using that address.

**Expected result:** The updated address appears in the list and future checkout. Existing orders retain their original address snapshot.

### CUS-009: Delete an address

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Delete a disposable address and refresh the list and checkout options.

**Expected result:** The address disappears from future choices. Historical orders remain readable with their original snapshot.

### CUS-010: Validate address boundaries

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit missing required fields, invalid postal codes, an unsupported country, and phone values below seven or above fifteen digits. Retry with valid boundaries.

**Expected result:** Invalid addresses are rejected without partial saves. Accepted phone and postal values follow the application's Indian address rules.

### CUS-011: Protect another customer's address

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use Customer B's address identifier in Customer A's edit, delete, and checkout requests.

**Expected result:** Foreign addresses remain inaccessible and unchanged. Checkout cannot use another customer's saved address.

### CUS-012: Add and remove wishlist products

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Add a product to the wishlist twice, reload it, and remove the product.

**Expected result:** The wishlist contains one matching entry. Removal persists without changing another customer's wishlist.

### CUS-013: Move items between wishlist and cart

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use each available move or add action between the cart and wishlist. Repeat with an unavailable variant.

**Expected result:** Supported actions preserve the intended product and quantity. Unavailable products receive clear feedback without a misleading cart success.

### CUS-014: Keep an authenticated cart

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Add two variants, sign out, sign in again, and inspect the cart.

**Expected result:** The account cart persists with the correct variants and quantities. Another customer does not see its contents.

### CUS-015: Update and remove cart lines

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Change a line to two units and use Remove. Refresh the page and inspect totals.

**Expected result:** The complete selected line disappears. Remaining totals are correct and persist after refresh.

### CUS-016: Enforce stock during cart changes

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Request more units than available and submit negative or malformed quantities.

**Expected result:** The server rejects invalid changes or reports the accepted stock limit clearly. Inventory is not oversold.

### CUS-017: Open checkout with a saved address

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Select an in-stock cart and a valid saved address, then open the order review.

**Expected result:** The review shows correct items, quantities, address, discount, shipping charges, and total for the configured environment.

### CUS-018: Handle an empty checkout

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Remove all cart items and open checkout or submit an order request.

**Expected result:** No order or payment intent is created. The customer receives a clear empty-cart message.

### CUS-019: Apply and remove a valid coupon

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Apply an active coupon to an eligible cart, compare the discount with its rule, and remove it.

**Expected result:** The server recalculates the total correctly. Removing the coupon restores the undiscounted total.

### CUS-020: Reject an invalid coupon

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Try an unknown code, an expired code, a paused campaign, a future campaign, and an exhausted usage limit.

**Expected result:** Ineligible coupons do not reduce the total or consume usage. The customer receives understandable feedback.

### CUS-021: Honor coupon date boundaries

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use controlled test dates at campaign start and end boundaries, including midnight in India.

**Expected result:** Coupon eligibility follows the configured India date boundaries. The displayed offer and checkout agree.

### CUS-022: Handle a changed checkout quote

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open checkout, change the product price or coupon availability from an authorized account, and submit the old review.

**Expected result:** The server rejects or refreshes the stale quote before confirming the order. The customer is not silently charged a different total.

### CUS-023: Reject manipulated checkout totals

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Alter submitted price, discount, total, product ownership, or cart identifier while retaining the customer's session.

**Expected result:** The server uses authoritative prices and ownership. Manipulated input cannot create a cheaper or foreign order.

### CUS-024: Create a pending order

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit a valid order review once and reopen the resulting order.

**Expected result:** One order is created with the correct amount, items, and address snapshot. Payment remains pending until a verified payment event succeeds.

### CUS-025: Prevent duplicate order creation

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Double-click order submission and repeat the same request with the same checkout key.

**Expected result:** The customer receives the same order or a clear duplicate response. Only one order and one stock reservation exist.

### CUS-026: Reject checkout key reuse for different data

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Reuse a successful checkout key after changing the address, cart contents, or coupon.

**Expected result:** The conflicting request is rejected. The original order and reservation remain unchanged.

### CUS-027: Complete a sandbox payment

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Prepare payment for the pending order, complete payment in the provider sandbox, and wait for verified server confirmation.

**Expected result:** The correct order becomes paid once. The provider reference, amount, and currency match, and stock is committed once.

### CUS-028: Handle payment failure

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use the provider sandbox's failed-payment scenario and reopen the order.

**Expected result:** The order is not shown as paid. The customer can follow the supported retry or cancellation path.

### CUS-029: Handle abandoned payment

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open payment, close the payment window, and revisit the order after the configured reservation period.

**Expected result:** The order accurately shows its unresolved state. Expired stock reservations are released without a false payment confirmation.

### CUS-030: Retry a pending payment

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Reopen payment for the same pending order and complete one valid payment.

**Expected result:** The existing order is used safely. The application does not create duplicate paid orders or capture stock twice.

### CUS-031: Block payment on another customer's order

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use Customer B's order number in Customer A's payment preparation and verification requests.

**Expected result:** The request is denied without exposing payment information. Customer B's order remains unchanged.

### CUS-032: View order history and details

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open the order list, a pending order, a paid order, and a delivered order.

**Expected result:** Each order shows its correct items, total, address, payment state, and fulfillment state. Only Customer A's orders appear.

### CUS-033: Read shipment tracking

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Have the seller record carrier and tracking details, then refresh the customer's order and follow the supported tracking link.

**Expected result:** The customer sees the seller's saved shipment information. The link is safe and refers to the correct shipment.

### CUS-034: Download an order receipt

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open the receipt for Customer A's order and attempt the same receipt using Customer B's session.

**Expected result:** The authorized receipt contains correct order and financial details. The other customer cannot download it.

### CUS-035: Cancel an eligible order

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Cancel a pending order before payment and refresh the order, cart, and stock views.

**Expected result:** The order becomes cancelled once. Reserved stock is released, and no payment is reported as captured.

### CUS-036: Reject an ineligible cancellation

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Attempt cancellation after the order reaches a state that the cancellation rules reject, and attempt a foreign order cancellation.

**Expected result:** The request is denied. Order, payment, and inventory remain consistent.

### CUS-037: Request a return for a delivered order

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open a delivered order and submit a clear return reason of at least ten characters.

**Expected result:** One pending return is created for Customer A's order. Customer and operations views show the same request.

### CUS-038: Reject an ineligible return

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Request returns for pending, paid, processing, shipped, cancelled, and foreign orders.

**Expected result:** Ineligible requests are rejected. The current delivered-order rule is enforced without exposing another customer's data.

### CUS-039: Validate and deduplicate a return request

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit a reason below ten characters, above one thousand characters, and then a valid request twice.

**Expected result:** Invalid reasons are rejected. Repeated valid submissions do not create multiple active returns for the same order.

### CUS-040: Read a return decision

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Have Returns operations approve one request and reject another. Refresh the customer return pages.

**Expected result:** Each customer sees the saved decision and permitted next action. Rejected requests are not shown as approved.

### CUS-041: Open the return label

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open the label or return instructions for an approved request. Try opening the same document from Customer B.

**Expected result:** Authorized instructions match the request and order. A foreign customer cannot obtain the return document.

### CUS-042: Track a completed refund

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Have Returns operations receive the approved return, then submit the supported refund action as the owning customer. Use the provider sandbox and refresh the order and return pages.

**Expected result:** The processed refund is shown with the correct amount and reference. Pending or failed provider refunds are not reported as completed.

### CUS-043: Submit a verified purchase review

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Select a delivered purchase, enter a rating from one to five, a title, and a review from ten to two thousand characters.

**Expected result:** The review is saved once as awaiting moderation and marked as a verified purchase. It is not public before approval.

### CUS-044: Reject an invalid or unverified review

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit an out-of-range rating, short or oversized text, a foreign purchase, and a product without a delivered purchase.

**Expected result:** Every invalid submission is rejected. No false verified-purchase review is created.

### CUS-045: Prevent duplicate reviews

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit a second review for the same delivered purchase and product.

**Expected result:** The application rejects the duplicate. The existing review remains unchanged.

### CUS-046: Read moderated reviews

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Have Marketplace admin publish one review and reject another. Open the customer review list and public product page.

**Expected result:** Only published reviews appear publicly. The customer's own review list reflects the moderation state.

### CUS-047: Follow and unfollow a seller

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Follow a seller, refresh the store page, sign in again, and unfollow the seller.

**Expected result:** The relationship persists once and is removed successfully. Seller counts and customer state remain consistent.

### CUS-048: Save notification preferences

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Change order, delivery, and offer notification choices, then refresh and sign in again.

**Expected result:** Supported choices persist. Offer preferences agree with marketing consent and do not overwrite unrelated consent.

### CUS-049: Read account notifications

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Trigger a supported order or support event and open notifications as both customers.

**Expected result:** Each customer sees only their own notifications. The event links to the correct authorized record.

### CUS-050: Manage marketing and analytics consent

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Enable and withdraw each optional consent, then refresh the privacy center and account notification preferences.

**Expected result:** The latest choice persists with its consent record. Withdrawing optional consent does not block essential shopping.

### CUS-051: Submit privacy requests

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit each supported privacy request type with test details and reopen the privacy center.

**Expected result:** The request appears with its correct type and status. An unsupported request type receives validation feedback.

### CUS-052: Revoke one device session

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Keep two customer sessions open and revoke the second session from the current session's security page.

**Expected result:** Only the selected session loses access. The current session remains usable, and the revoked device cannot submit changes.

### CUS-053: Revoke all sessions

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use the account's sign-out-all-sessions action while two sessions are active.

**Expected result:** All sessions lose protected access. Sign-in is required again on each device.

### CUS-054: Delete a disposable customer account

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Create a deletion request for a disposable customer with orders, confirm the supported deletion action, and try its old sessions and credentials.

**Expected result:** The account is anonymized and cannot sign in. Personal records are removed or redacted according to the documented retention matrix, while required order totals remain.

### CUS-055: Protect privacy deletion ownership

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Submit Customer B's deletion request identifier from Customer A. Repeat an already completed request.

**Expected result:** The foreign request is denied, and completed deletion cannot execute twice. Customer A and Customer B remain unaffected by denied requests.

### CUS-056: Create a customer support case

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Create a case with a meaningful subject and message, link an owned order, and attach a supported image.

**Expected result:** The case, owned order link, message, and attachment persist. The customer can reopen the case after sign-in.

### CUS-057: Continue a support conversation

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Have the support agent reply, then reply as Customer A and reload both views.

**Expected result:** Public replies appear in chronological order with the correct author. Private agent notes do not appear in the customer conversation.

### CUS-058: Protect support case ownership

Priority: P0. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Use Customer B's case or attachment identifier in Customer A's view, reply, and download requests.

**Expected result:** Foreign case data remains inaccessible. No unauthorized message or attachment access is accepted.

### CUS-059: Validate support case details

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Create a case with a subject below four or above 160 characters, a message below ten or above four thousand characters, and a nonexistent or foreign order reference.

**Expected result:** Invalid cases are rejected without partial conversations. The customer cannot link another customer's order.

### CUS-060: Check address selection after deletion

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Open checkout with a saved address, delete that address in another session, and submit the stale checkout selection.

**Expected result:** The stale identifier cannot select an unrelated address. The customer receives clear recovery instructions or confirms a valid address snapshot.

### CUS-061: Remove unavailable wishlist and cart items

Priority: P1. Test mode: Application. Run as: Customer.

**Before you start:** Sign in as Customer A and keep Customer B in a separate browser. Use the product, order, address, coupon, review, and support fixtures described in the preparation guide.

**Test steps:** Wishlist and cart a product, have the seller archive it, then reopen both pages and try checkout.

**Expected result:** The customer can remove the unavailable item. It cannot be purchased from stale saved pages.

<a id="own-cases"></a>

## Seller owner workflows

Default context: Seller owner.

Before this group: Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

Source references: [internal/application/commerce/catalog.go](../internal/application/commerce/catalog.go), [internal/application/commerce/staff.go](../internal/application/commerce/staff.go), [internal/application/commerce/seller_orders.go](../internal/application/commerce/seller_orders.go), [internal/web/handlers/live_portal.go](../internal/web/handlers/live_portal.go), [web/selleradmin/](../web/selleradmin/).

### OWN-001: Complete seller onboarding

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Open onboarding with a new pending seller owner, enter the required profile information, and submit it.

**Expected result:** The seller profile persists with an honest pending state. The seller cannot claim approval before marketplace review.

### OWN-002: Restrict a pending seller

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Attempt listing publication and order processing before the seller becomes active.

**Expected result:** Actions restricted to active sellers are denied. The onboarding page explains the current state without showing fabricated sales.

### OWN-003: Save the seller profile

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Update Seller A's public display name and required reason, then refresh settings and its public store.

**Expected result:** The new name persists for Seller A. Seller B's profile remains unchanged.

### OWN-004: Validate seller settings

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit blank profile fields, excessive text, and a missing or invalid required reason.

**Expected result:** Invalid changes are rejected. The last valid profile remains stored.

### OWN-005: Create a product draft

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Create a product with a unique name, slug, SKU, existing brand and category, positive price, and initial stock.

**Expected result:** Exactly one draft and its initial variant are saved under Seller A. The draft is not publicly purchasable.

### OWN-006: Select existing brands and categories

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Open the product editor and choose existing brand and category options. Save and reopen the product.

**Expected result:** Selections persist and use the intended catalog records. The editor does not silently create duplicate directory entries.

### OWN-007: Validate product fields

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit missing fields, invalid slugs, a name over 180 characters, and a description over ten thousand characters.

**Expected result:** The application identifies invalid data and does not save an incomplete product.

### OWN-008: Validate product price and stock

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit zero or negative price, negative stock, stock above one hundred thousand, a compare-at price below selling price, and negative cost price.

**Expected result:** Invalid financial and stock values are rejected. Accepted values display without rounding errors.

### OWN-009: Reject duplicate product identifiers

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Create another listing using an existing slug or SKU where uniqueness is required.

**Expected result:** The duplicate receives clear conflict feedback. The original listing and stock remain unchanged.

### OWN-010: Edit an owned draft

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Change a draft's description, price, and category, save it, and reopen the editor.

**Expected result:** All accepted fields persist on the correct listing. No duplicate product or variant is created.

### OWN-011: Preserve stock while editing product text

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Change only the product name and description after stock has been adjusted. Refresh inventory.

**Expected result:** Text edits do not reset current stock to the original stock value or overwrite reserved units.

### OWN-012: Upload a product cover

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Upload a valid PNG, JPEG, and WebP cover in separate runs. Save and inspect the seller editor and public listing after approval.

**Expected result:** Each selected cover persists and loads as the correct image. The application never substitutes an unrelated stock image as a successful upload.

### OWN-013: Validate product cover limits

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Upload an empty file, an unsupported format, a renamed non-image, a file at the 25 MiB limit, and a file above it.

**Expected result:** Supported images within the limit are accepted. Invalid or oversized files are rejected without replacing the existing cover.

### OWN-014: Keep product covers after a restart

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Save a distinctive product cover, restart or rebuild the test application using its documented persistent volume, and reopen it.

**Expected result:** The original file remains available. The cover URL does not become broken after the restart.

### OWN-015: Upload additional product media

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Upload supported JPEG, PNG, WebP, MP4, and WebM media through the product media controls and complete upload finalization.

**Expected result:** Verified media appears on the correct product. Unfinished uploads do not become ready or publicly readable.

### OWN-016: Validate media metadata and content

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Upload a file whose actual content differs from its declared type or size. Attempt finalization before the object exists.

**Expected result:** Invalid or missing objects cannot become ready media. Existing product media remains usable.

### OWN-017: Recover from an interrupted upload

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Interrupt an upload, reopen the editor, and retry with a valid file. Run the supported orphan cleanup in the test environment.

**Expected result:** The failed upload is reported honestly. Retry succeeds, and cleanup removes only expired orphaned uploads.

### OWN-018: Remove product media

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Remove additional media, including the currently selected primary image, and refresh the product page.

**Expected result:** Removed media is no longer served. The gallery uses a valid remaining cover or an honest empty state.

### OWN-019: Protect another seller's media

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Use Seller B's product and media identifiers in Seller A's prepare, upload, finalize, delete, and preview requests.

**Expected result:** Every foreign operation is denied. Seller B's files and metadata remain unchanged.

### OWN-020: Create a product variant

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Add a second named variant with a unique SKU, price, and initial stock. Approve the listing and select both variants as a customer.

**Expected result:** Both variants persist with independent stock and price. Customer selection uses the chosen variant for checkout.

### OWN-021: Edit a product variant

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Change a variant's name and price without changing stock. Reopen the editor and product page.

**Expected result:** Accepted changes persist on that variant. Existing orders retain their purchased price and SKU snapshot.

### OWN-022: Validate product variants

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit missing variant names or SKUs, duplicate SKUs, nonpositive prices, invalid compare-at prices, and negative stock.

**Expected result:** Invalid variants are rejected. Other variants remain intact.

### OWN-023: Archive a product variant

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Archive a disposable variant with a valid reason, then try adding it from an old product page or cart.

**Expected result:** The variant stops accepting new purchases. Existing orders and their historical details remain readable.

### OWN-024: Submit a listing for review

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit a complete draft and inspect the seller product list and marketplace review queue.

**Expected result:** The listing moves to pending review once. It remains unavailable publicly until approval.

### OWN-025: Read a listing rejection

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Have Marketplace admin reject the listing with a reason, then reopen it as Seller A.

**Expected result:** The rejection state and reason are visible. The rejected product is not public.

### OWN-026: Correct and resubmit a rejected listing

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Correct the rejected listing and submit it again. Have Marketplace admin approve it.

**Expected result:** The new review state persists. Approval makes the corrected product publicly available.

### OWN-027: Prevent self-approval

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Attempt marketplace approval requests using the seller owner's session.

**Expected result:** The server denies self-approval. The pending listing does not become public.

### OWN-028: Archive and restore a listing

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Archive an owned listing with a reason, try buying it, then restore it with a reason.

**Expected result:** Archiving blocks new purchases and preserves historical orders. Restoration returns the listing to draft for the required review.

### OWN-029: Protect another seller's catalog

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Use Seller B's identifiers in Seller A's view, edit, submit, archive, restore, and variant requests.

**Expected result:** Foreign catalog records remain inaccessible and unchanged.

### OWN-030: Increase and decrease stock

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Make a positive stock adjustment and a permitted negative adjustment with reasons. Refresh inventory and the storefront.

**Expected result:** Stock changes by the exact amount and leaves an audit record. Public availability agrees with available stock.

### OWN-031: Prevent invalid stock adjustments

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit malformed quantities, a missing required reason, and a reduction below committed or reserved stock.

**Expected result:** Invalid adjustments are rejected. Available stock cannot become negative.

### OWN-032: Configure low-stock alerts

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Set the supported low-stock threshold, adjust stock across the threshold, and refresh the inventory page.

**Expected result:** The threshold persists. Low-stock indicators match current stock and the configured boundary.

### OWN-033: Check stock reservations

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Have a customer create an unpaid order, then cancel it or let its reservation expire. Inspect Seller A's inventory throughout.

**Expected result:** Reserved and available quantities remain consistent. Cancellation or expiry releases stock once.

### OWN-034: Open seller orders

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Create a paid order containing Seller A's items and open its seller order detail.

**Expected result:** The seller sees the correct items, quantity, address needed for fulfillment, and its own subtotal. Unrelated seller items are excluded.

### OWN-035: Start order processing

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Move an eligible paid seller order to processing and refresh both seller and customer views.

**Expected result:** The accepted fulfillment state persists. Payment remains server-controlled.

### OWN-036: Ship an order

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Record a carrier, a tracking number of at least three characters, and an optional note, then mark the order shipped.

**Expected result:** Tracking and shipped state persist on the correct seller shipment. The customer receives the supported dispatch notification.

### OWN-037: Validate shipping details

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Attempt shipment with missing or short tracking information and fields above their configured limits.

**Expected result:** Invalid shipping changes are rejected. The previous fulfillment state remains intact.

### OWN-038: Mark delivery

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Complete an eligible seller shipment's delivered action and inspect the customer order and return eligibility.

**Expected result:** The saved delivery state is consistent across views. The customer becomes eligible under the current delivered-order return rule.

### OWN-039: Reject invalid fulfillment transitions

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Try fulfilling an unpaid, cancelled, or refunded order and try moving a delivered shipment back to processing.

**Expected result:** Invalid transitions are rejected. No inconsistent order or shipment state is created.

### OWN-040: Protect another seller's shipment

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Use Seller B's seller identifier and order number in Seller A's fulfillment request.

**Expected result:** The action is denied. Seller B's tracking and shipment remain unchanged.

### OWN-041: View seller returns

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Create a return involving Seller A and inspect the seller returns page.

**Expected result:** The seller sees only relevant return information. Read access does not grant return approval or refund powers.

### OWN-042: Check earnings and analytics

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Compare seller reports with known paid, cancelled, and refunded fixtures. Change supported report tabs or periods.

**Expected result:** Reports use the correct seller scope and financial states. Counts and amounts reconcile with the underlying records.

### OWN-043: Read commissions and settlements

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Open earnings tabs for commission and settlement information after finance creates a test settlement.

**Expected result:** The seller sees its own ledger and settlement status. The seller cannot change commission calculations or mark payouts paid.

### OWN-044: Add seller staff

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Add an existing disposable account to Seller A's team with selected valid permissions. Sign in as that staff member.

**Expected result:** One active membership is created for Seller A. Staff receives only the selected permissions.

### OWN-045: Validate staff additions

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Try an unknown email, a duplicate member, an unsupported permission, and a disallowed membership combination.

**Expected result:** Invalid additions receive clear feedback. Existing membership and ownership remain unchanged.

### OWN-046: Change staff permissions

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Grant and revoke catalog and inventory permissions while staff keeps a page open. Retry its actions.

**Expected result:** Allowed actions follow the new permissions. Removed permissions cannot authorize an already open form.

### OWN-047: Remove staff access

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Remove a disposable staff member from Seller A and retry its seller workspace and saved requests.

**Expected result:** Seller A access is revoked. The account's other explicitly assigned roles remain usable.

### OWN-048: Protect seller ownership

Priority: P0. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Attempt to add the owner as removable staff, remove ownership through team controls, or edit Seller B's membership.

**Expected result:** Owner protection and seller boundaries hold. Team controls cannot transfer or delete another seller's ownership.

### OWN-049: Create seller support

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Create an owner support request with a meaningful subject and message and link an owned order where appropriate.

**Expected result:** The owner sees a persisted seller support request. Customer cases and another seller's requests are not substituted.

### OWN-050: Reply to seller support

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Reply to an owned seller support request, reload it, and inspect the support agent's view.

**Expected result:** The conversation persists with the correct author and seller context. Staff without owner support access cannot use the same request.

### OWN-051: Read seller policies

Priority: P2. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Follow the seller terms and privacy links from the seller workspace, including on a narrow screen.

**Expected result:** Both pages load the intended policy content. The seller can return to its workspace.

### OWN-052: Validate seller support creation

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Submit a short subject, empty message, oversized message, and Seller B's order reference through Seller A's support form.

**Expected result:** Invalid requests are rejected. No foreign order is linked or disclosed.

### OWN-053: Validate additional-media size boundaries

Priority: P1. Test mode: Application. Run as: Seller owner.

**Before you start:** Sign in as Seller A's owner. Keep Seller B's owner separate. Prepare a pending seller, an active seller, draft products, approved products, stock, paid orders, and staff accounts.

**Test steps:** Upload a valid supported additional-media file at 25 MiB and another above that limit. Try declaring a smaller size than the actual object.

**Expected result:** Only valid files within the authoritative limit can finalize. Declared metadata cannot bypass actual size verification.

<a id="stf-cases"></a>

## Seller staff permissions

Default context: Seller staff.

Before this group: Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

Source references: [internal/domain/commerce/models.go](../internal/domain/commerce/models.go), [internal/application/commerce/staff.go](../internal/application/commerce/staff.go), [internal/web/handlers/portal_routes.go](../internal/web/handlers/portal_routes.go), [internal/adapters/postgres/role_authorizer.go](../internal/adapters/postgres/role_authorizer.go).

### STF-001: Enter the assigned seller workspace

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Sign in as an active staff member and inspect its seller identity and navigation.

**Expected result:** The workspace identifies the correct seller and staff role. It exposes only permitted pages and actions.

### STF-002: Handle no staff permissions

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Sign in with an active seller membership that has no business permissions.

**Expected result:** The staff member can see only the allowed shell and informational pages. No catalog, order, inventory, or report data leaks through the dashboard.

### STF-003: Use product read permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant only PRODUCT_READ and open products. Attempt creation, editing, submission, archiving, and media changes.

**Expected result:** The product list is readable within Seller A. Every product write remains denied.

### STF-004: Use product write permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant PRODUCT_WRITE with PRODUCT_READ, create and edit a draft, upload media, and submit it for review.

**Expected result:** Permitted catalog writes succeed only for Seller A. Marketplace approval and owner settings remain denied.

### STF-005: Remove product write permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Open a product editor, revoke PRODUCT_WRITE, and submit its saved form and direct media request.

**Expected result:** Both requests are denied. The product and media remain unchanged.

### STF-006: Use inventory read permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant only INVENTORY_READ, open inventory, and try adjusting stock or changing alert thresholds.

**Expected result:** Stock is readable within Seller A. Inventory writes are denied.

### STF-007: Use inventory write permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant INVENTORY_WRITE with INVENTORY_READ and make a valid stock adjustment. Try the same action for Seller B.

**Expected result:** The authorized adjustment persists and is audited. The foreign adjustment is denied.

### STF-008: Use order read permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant only ORDER_READ, open an order and its detail, and attempt processing or shipping.

**Expected result:** Seller A's order details are readable. Fulfillment writes are denied.

### STF-009: Use order fulfillment permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant ORDER_FULFILL with ORDER_READ and process, ship, and deliver an eligible Seller A order.

**Expected result:** Supported fulfillment actions succeed within Seller A. Unpaid and foreign orders remain protected.

### STF-010: Use return read permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant only RETURN_READ, inspect seller returns, and attempt approval, receipt, and refund actions.

**Expected result:** Relevant returns are readable. The current staff permission does not authorize return processing or financial writes.

### STF-011: Use report read permission

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant REPORT_READ and inspect accessible earnings and analytics pages. Attempt commission or payout changes.

**Expected result:** Reports are limited to Seller A. Report access does not grant financial mutation permissions.

### STF-012: Check write-only permission combinations

Priority: P1. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant PRODUCT_WRITE, INVENTORY_WRITE, and ORDER_FULFILL individually without the corresponding read permission. Test direct page and action access.

**Expected result:** The application follows its documented independent permission rules. The interface does not imply a read grant that was never assigned.

### STF-013: Combine staff permissions

Priority: P1. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Grant a useful catalog, inventory, and order permission set. Perform one authorized action from each area.

**Expected result:** Each selected permission works independently. Permissions do not accidentally unlock owner-only areas.

### STF-014: Deny team management

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Try opening team management and submit add, update, and remove membership requests as staff.

**Expected result:** All owner-only team actions are denied. Staff cannot grant themselves permissions.

### STF-015: Deny seller settings changes

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Try opening or changing seller settings and submitting onboarding mutations as staff.

**Expected result:** Owner-only changes are denied. Any allowed onboarding information page remains read-only.

### STF-016: Deny owner support access

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Open the owner support page and submit seller support creation or reply requests as staff.

**Expected result:** Owner support access is denied unless an explicit supported permission is introduced. No seller request is created.

### STF-017: Deny marketplace and payment administration

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Attempt seller approval, product moderation, role assignment, payment-state changes, refunds, and settlement actions as staff.

**Expected result:** Every privileged action is denied. Staff permissions cannot become marketplace or finance powers.

### STF-018: Apply permissions to legacy routes

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Repeat a denied catalog and inventory action through older seller routes and the JSON media endpoints.

**Expected result:** The same server permissions apply to every entry point. Alternate routes do not bypass restrictions.

### STF-019: Protect another seller's data

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Guess Seller B's product, variant, order, return, inventory, media, and report identifiers.

**Expected result:** Foreign records are neither disclosed nor changed. Responses do not reveal sensitive record details.

### STF-020: Revoke a staff membership during work

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Keep staff signed in, remove its membership, and retry a page, an action, and a media upload.

**Expected result:** Every Seller A business request is denied after removal. A stale session does not preserve membership.

### STF-021: Suspend the assigned seller

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Suspend Seller A while staff is active, then try product changes, stock changes, and fulfillment.

**Expected result:** Seller suspension is enforced for staff. The interface accurately explains restricted seller access.

### STF-022: Avoid leaking reports through summary cards

Priority: P0. Test mode: Application. Run as: Seller staff.

**Before you start:** Use Seller A staff accounts with no permissions, each individual permission, useful permission combinations, and all eight permissions. Keep Seller B's records available for denial tests.

**Test steps:** Remove REPORT_READ and inspect the seller home page, direct analytics URLs, and query-selected report tabs.

**Expected result:** Restricted earnings, settlement, and report data is not exposed through alternate views.

<a id="mkt-cases"></a>

## Marketplace administration

Default context: Marketplace admin.

Before this group: Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

Source references: [internal/application/commerce/seller_admin.go](../internal/application/commerce/seller_admin.go), [internal/application/commerce/management.go](../internal/application/commerce/management.go), [internal/web/handlers/live_portal.go](../internal/web/handlers/live_portal.go), [internal/web/handlers/portal_management.go](../internal/web/handlers/portal_management.go), [web/admin/](../web/admin/).

### MKT-001: Review a seller application

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open the pending seller queue and inspect the applicant's profile and available review details.

**Expected result:** The queue contains real pending sellers. Details match the selected applicant.

### MKT-002: Approve a seller

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Approve a pending seller with a valid audit reason and reopen its seller workspace.

**Expected result:** The seller becomes active once. Its permitted business workflows become available, and the decision is audited.

### MKT-003: Reject or suspend a seller

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Apply each supported rejection or suspension action with a meaningful reason and inspect the seller's public products and workspace.

**Expected result:** The saved seller state is visible. Suspended sellers cannot continue restricted sales or writes.

### MKT-004: Restore an eligible seller

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Use the supported status action to reactivate a disposable suspended seller.

**Expected result:** The seller returns to the accepted active state. Existing orders remain intact, and the change is audited.

### MKT-005: Validate seller decisions

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Submit an unknown seller, unsupported status, missing reason, and reason outside the accepted limits.

**Expected result:** Invalid decisions are rejected without partial status or audit changes.

### MKT-006: Review pending products

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open the product moderation queue and inspect pending products from two sellers.

**Expected result:** The queue shows submitted products and their real variants and media. Drafts are not falsely shown as approved.

### MKT-007: Approve a product

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Approve a complete pending product and inspect its seller list, product page, category, and search results.

**Expected result:** The product becomes publicly available consistently. The seller cannot publish an unapproved version through a stale page.

### MKT-008: Reject a product with a reason

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Reject a pending product with a meaningful reason and inspect the seller's editor.

**Expected result:** The rejection and reason persist. The rejected product remains unavailable publicly.

### MKT-009: Validate moderation transitions

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Repeat an approval, reject without a required reason, and attempt moderation of an archived or nonexistent listing.

**Expected result:** Invalid decisions are rejected or safely deduplicated. No unintended listing becomes public.

### MKT-010: Manage product lifecycle within granted access

Priority: P1. Test mode: Application. Run as: Marketplace admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Use each catalog action exposed to Marketplace admin, including variants, archive, and restore. Attempt any action exposed only to Super Admin.

**Expected result:** Granted actions persist and are audited. Super Admin-only catalog creation or editing is denied rather than implied by a shared template.

### MKT-011: View marketplace inventory

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open inventory and compare a known variant's stock with the seller view. Use every exposed filter and action.

**Expected result:** Displayed quantities agree with authoritative inventory. Actions require the role's actual grant and cannot bypass stock rules.

### MKT-012: Create a category

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Create a category with a valid unique slug, name, and reason, then use it in a seller listing.

**Expected result:** The directory entry persists once and is selectable by sellers. Approved matching products appear in its public category.

### MKT-013: Edit a category

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Rename a category and refresh its directory, seller editor, and public category page.

**Expected result:** The accepted name change appears consistently. Existing product links remain valid.

### MKT-014: Protect referenced categories

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Try deleting a category used by a product, then delete an unused disposable category.

**Expected result:** Referenced records receive the supported protection or migration behavior. Unused deletion does not damage existing products.

### MKT-015: Validate category data

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Create invalid or duplicate slugs, blank names, and changes without valid reasons.

**Expected result:** Invalid data is rejected. The directory contains no duplicate or partial records.

### MKT-016: Manage brands

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Create, rename, and delete an unused brand with valid reasons. Inspect the brand directory and seller choices.

**Expected result:** Accepted changes persist and appear in the correct places. A referenced brand follows the supported deletion protection.

### MKT-017: Search and page directories

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Create enough test categories, brands, and customers for several pages. Search and move between pages.

**Expected result:** Results match the query and retain stable page navigation. Empty results are explained clearly.

### MKT-018: View customer records safely

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open customer records and compare visible contact fields with the masked customer view used for operations.

**Expected result:** Only information authorized for the workspace is displayed. Passwords, reset tokens, and payment credentials are absent.

### MKT-019: View marketplace orders

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open pending, paid, shipped, delivered, cancelled, and refunded orders.

**Expected result:** The queue reflects true order and shipment states. Order totals match customer and seller records.

### MKT-020: Process marketplace returns

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Approve one valid pending return, reject another, and record receipt of an approved return.

**Expected result:** Supported transitions persist and appear to customers. Return receipt does not itself claim a successful provider refund.

### MKT-021: Protect return transitions

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Attempt receipt before approval and alter a completed or nonexistent return.

**Expected result:** Invalid transitions are rejected. Existing decisions and financial state remain consistent.

### MKT-022: Inspect customer support issues

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Open marketplace support and use each exposed authorized case action. Compare public messages and private notes with the support workspace.

**Expected result:** The same case record is used across workspaces. Private notes remain private, and unsupported support powers stay denied.

### MKT-023: Publish a verified review

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Publish a pending verified purchase review and open the public product page.

**Expected result:** The review becomes public with the correct rating and verified-purchase marker. Product review summaries refresh correctly.

### MKT-024: Reject a review

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Reject a review with a valid reason and try rejecting another without the required reason.

**Expected result:** The valid decision persists and removes the review from public display. Missing reasons are rejected.

### MKT-025: Create percentage and fixed promotions

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Create separate draft campaigns with a unique coupon, valid discount, and optional date and usage limits.

**Expected result:** Both campaigns persist as drafts with correct coupon rules. Draft campaigns do not discount checkout.

### MKT-026: Edit and activate a promotion

Priority: P0. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Edit campaign details, activate the campaign, and apply its coupon to an eligible customer cart.

**Expected result:** Saved rules appear in checkout and deals. The customer receives the correct server-calculated discount.

### MKT-027: Pause and archive a promotion

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Pause an active campaign, attempt its coupon, then archive it and refresh marketplace and public views.

**Expected result:** Paused and archived campaigns stop granting discounts. Existing order discount snapshots remain unchanged.

### MKT-028: Delete an unused promotion

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Delete an unused disposable campaign, then attempt to delete a campaign already used by an order.

**Expected result:** The unused campaign is removed. Used campaigns are protected and can be archived without losing usage history.

### MKT-029: Validate promotion boundaries

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Submit percentages of zero and 101, nonpositive fixed discounts, duplicate or invalid codes, invalid dates, reversed dates, and nonpositive usage limits.

**Expected result:** Invalid campaigns are rejected without partial coupon records. Accepted percentage values range from one to one hundred.

### MKT-030: Inspect analytics and audit records

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Make a seller decision and a catalog decision, then inspect marketplace metrics and audit filters.

**Expected result:** Reports reflect current marketplace records. Audit entries identify the actor, action, resource, time, reason where required, and request context.

### MKT-031: Apply permitted marketplace settings

Priority: P1. Test mode: Application. Run as: Marketplace admin, Super admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Update permitted support email and shipping information with a reason. Refresh contact and shipping pages.

**Expected result:** Supported public settings persist and reach the storefront. Invalid email or insufficient shipping text is rejected.

### MKT-032: Deny security and finance overrides

Priority: P0. Test mode: Application. Run as: Marketplace admin.

**Before you start:** Sign in as Marketplace admin for the primary run, or as Super Admin for a listed repeat. Prepare pending and active sellers, products in each moderation state, public directory entries, promotions, reviews, orders, and returns.

**Test steps:** Try role management, user deletion, security settings, commission changes, settlement mutations, and finance pages through URLs and direct requests.

**Expected result:** Security and finance powers outside Marketplace admin's assignment are denied. Shared handlers and query parameters do not widen access.

<a id="sup-cases"></a>

## Super Admin workflows

Default context: Super admin.

Before this group: Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

Source references: [internal/application/auth/roles.go](../internal/application/auth/roles.go), [internal/adapters/postgres/role_admin_repository.go](../internal/adapters/postgres/role_admin_repository.go), [internal/web/handlers/live_portal.go](../internal/web/handlers/live_portal.go), [internal/web/handlers/portal_management.go](../internal/web/handlers/portal_management.go), [tests/e2e/portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js).

### SUP-001: Open all Super Admin administration areas

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Visit catalog, sellers, inventory, orders, returns, reviews, marketing, analytics, settings, security, roles, audit, and finance under the Super Admin workspace.

**Expected result:** Permitted administration pages use live records and correct navigation. A separate role workspace still requires its own assignment.

### SUP-002: Create an operational user

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Create a disposable account with a unique email, valid display name, a password of at least twelve characters, an assignable role, and a reason.

**Expected result:** One account and role assignment are created. Its first sign-in opens the correct assigned workspace.

### SUP-003: Create every assignable role

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Repeat operational user creation separately for customer, seller owner, seller staff, marketplace admin, support agent, finance operator, and operations.

**Expected result:** Every supported role receives the intended assignment. Seller staff requires a valid active seller; owner onboarding remains subject to seller approval.

### SUP-004: Validate operational user creation

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Submit malformed or duplicate emails, a short password, invalid role, missing reason, and an invalid seller association.

**Expected result:** Invalid requests are rejected without creating partial accounts or assignments.

### SUP-005: Edit an account profile

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Change a disposable account's email and display name with a valid reason, then inspect its sign-in and displayed profile.

**Expected result:** Changes persist on the intended account. The old email no longer authenticates if it was changed.

### SUP-006: Suspend and reactivate an account

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Suspend a disposable account while its session is active, then restore it using the supported account action.

**Expected result:** Suspension blocks protected work. Reactivation follows the current sign-in rules and is audited.

### SUP-007: Reset an account password

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Set a valid replacement password for a disposable operational account and retry its existing sessions and old password.

**Expected result:** Old sessions lose access. The old password fails, and the replacement works.

### SUP-008: Protect the active administrator

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Attempt to suspend or delete the current Super Admin and disable the last active Super Admin.

**Expected result:** Protected actions are rejected. The system retains an active administrative recovery path.

### SUP-009: Delete an eligible operational account

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Delete a disposable operational account with a valid reason, then try signing in and using an existing session.

**Expected result:** The account can no longer authenticate or continue work. Historical references follow the documented retention rules.

### SUP-010: Grant a supported role

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Grant a second supported role to a disposable account with a reason, then have the account sign in again.

**Expected result:** The role assignment persists, and the workspace chooser reflects it. Existing sessions follow the revocation policy.

### SUP-011: Revoke a supported role

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Revoke a disposable account's operational role and retry a protected action from its open session.

**Expected result:** Revoked permissions cannot continue. The account retains only its remaining explicit roles.

### SUP-012: Validate role changes

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Submit an unsupported role, duplicate grant, missing reason, invalid target user, and seller staff without an active seller.

**Expected result:** Invalid changes are rejected with no accidental privilege grant. The existing assignments remain correct.

### SUP-013: Prevent unrestricted Super Admin grants

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Attempt to create or grant the Super Admin role through ordinary operational role forms and direct requests.

**Expected result:** Ordinary role administration rejects the unsupported grant. A protected high-privilege provisioning path cannot be inferred from editable input.

### SUP-014: Create and edit a seller product

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Create a product for an active seller through Super Admin catalog controls, then edit it and add a variant.

**Expected result:** The product remains associated with the selected seller. Prices, stock, media, and variant details persist correctly.

### SUP-015: Validate seller selection in product creation

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Try creating a product without a seller, with a nonexistent seller, and with an ineligible seller.

**Expected result:** The application rejects invalid seller selection. It does not attach the product to an arbitrary seller.

### SUP-016: Moderate and archive catalog records

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Submit and approve an administrative draft, reject another submission, and archive and restore a disposable listing.

**Expected result:** Lifecycle rules match marketplace moderation. Public visibility and historical orders remain consistent.

### SUP-017: Exercise shared marketplace workflows

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Repeat the marketplace cases listed in the Super Admin reuse checklist through the Super Admin workspace. Record each case separately.

**Expected result:** Each granted workflow meets the same acceptance results and audit requirements. Record a separate result for each repeated case.

### SUP-018: Manage commission rules

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Create and update a commission rule under Super Admin and inspect a new seller sale's ledger.

**Expected result:** The accepted rate persists and applies according to its scope. Existing financial snapshots remain consistent.

### SUP-019: Exercise finance oversight

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Open payments, refunds, reconciliation, and settlement management under Super Admin. Repeat each finance case listed in the reuse checklist through Super Admin's own routes.

**Expected result:** Super Admin's own finance routes work within their grants. The separate finance workspace does not become implicitly assigned.

### SUP-020: Save platform settings

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Change supported public settings with a reason and inspect storefront contact and shipping content.

**Expected result:** The accepted setting is persisted and reaches its consumer. Unsupported security configuration is not falsely reported as changed.

### SUP-021: Inspect full audit history

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Search audit records after account creation, role changes, seller decisions, stock updates, and financial actions.

**Expected result:** Authorized history identifies the actor and affected resource with useful timestamps and reasons. Sensitive credentials and tokens are not exposed.

### SUP-022: Inspect system dependency health

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Open system, search-indexing, and worker health pages while dependencies are healthy and while a test dependency is unavailable.

**Expected result:** Health reflects real probes and queue state. An unavailable dependency is not represented by a static success indicator.

### SUP-023: Inspect security events

Priority: P1. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Trigger controlled failed sign-ins and denied privileged actions, then inspect the available security records.

**Expected result:** Supported events can be correlated with the request and account. Missing event coverage is recorded explicitly.

### SUP-024: Reject untrusted administrative input

Priority: P0. Test mode: Application. Run as: Super admin.

**Before you start:** Sign in as Super Admin A. Keep a second active Super Admin, disposable operational accounts, an active seller, and a customer with retained historical orders.

**Test steps:** Enter harmless script-like text in profile names, audit reasons, product descriptions, and settings, then view all affected pages.

**Expected result:** Text is escaped safely. Viewing stored text cannot execute script or alter privileged forms.

<a id="fin-cases"></a>

## Finance operator workflows

Default context: Finance operator.

Before this group: Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

Source references: [internal/transport/web/finance_handlers.go](../internal/transport/web/finance_handlers.go), [internal/application/commerce/finance.go](../internal/application/commerce/finance.go), [internal/web/handlers/live_portal.go](../internal/web/handlers/live_portal.go), [internal/adapters/postgres/management_repository.go](../internal/adapters/postgres/management_repository.go), [tests/e2e/portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js).

### FIN-001: Read payment records

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Open payments and compare known order amounts, currencies, provider references, and payment states.

**Expected result:** Displayed finance records agree with the authoritative order and payment records. Pending payments are not counted as captured.

### FIN-002: Read failed payment records

Priority: P1. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Open failed payments and inspect a known failed attempt and a successfully captured order.

**Expected result:** The failure view includes the failed record and excludes unrelated successful records. Sensitive provider secrets are absent.

### FIN-003: Read refund records

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Open refunds after a customer refund is pending, failed, and processed in separate fixtures.

**Expected result:** Each record shows its true amount, reference, and state. Only a processed provider refund appears completed.

### FIN-004: Search and page financial records

Priority: P1. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Search by a known supported order or transaction reference and navigate a multi-page list.

**Expected result:** Search returns matching records, and pages remain consistent. No-record results are clearly explained.

### FIN-005: Reconcile payments with orders

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Compare known captured, pending, failed, and refunded payment records with their order states and totals.

**Expected result:** Matching records reconcile correctly. Mismatches are visible for investigation instead of silently treated as balanced.

### FIN-006: Investigate a mismatch

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Use an isolated mismatch fixture, open reconciliation, and follow the available investigation action.

**Expected result:** The discrepancy remains traceable to its order and provider reference. Unsupported repair actions are recorded as a gap.

### FIN-007: Create a commission rule

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Create a rule with a valid scope and percentage, save it with a reason, and reopen it.

**Expected result:** The rate persists as the intended basis-point value. The rule can be related to the correct seller or category scope.

### FIN-008: Validate commission boundaries

Priority: P1. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Test rates below zero, zero, one hundred percent, and above one hundred percent, with valid reasons.

**Expected result:** Rates from zero through one hundred percent follow the accepted rules. Values outside that range are rejected.

### FIN-009: Update or remove a commission rule

Priority: P1. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Change a disposable rule, inspect a new sale, and remove an unused rule. Inspect historical ledger entries.

**Expected result:** Future calculations follow the active rules. Historical financial entries are not silently rewritten.

### FIN-010: Check the seller ledger

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Compare a known sale and refund with seller gross, commission, and net entries.

**Expected result:** Amounts reconcile in integer currency units with consistent rounding. Refund handling does not leave an unexplained seller balance.

### FIN-011: Create settlement batches

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Create batches from known eligible open ledger entries and inspect each seller's settlement.

**Expected result:** Each eligible entry is assigned once to the correct seller batch. Gross, commission, net, and covered period agree with its ledger.

### FIN-012: Repeat settlement creation

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Submit batch creation twice and repeat it concurrently in a disposable environment.

**Expected result:** Already assigned ledger entries are not batched again. No duplicate seller entitlement is created.

### FIN-013: Approve a settlement

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Approve a pending batch and refresh finance and seller earnings views.

**Expected result:** The batch becomes approved once with audit evidence. Both roles see the same reference and amounts.

### FIN-014: Record a local mock payout

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** In the local demo, approve a batch and use Record mock payout. Read its confirmation and reopen the record.

**Expected result:** The stored batch becomes paid and its ledger entries become settled. The interface states that no bank transfer was made.

### FIN-015: Reject invalid settlement transitions

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Attempt paying a pending batch, failing a paid batch, and repeating a terminal decision.

**Expected result:** Invalid transitions are rejected. Terminal financial records and settled ledger entries remain unchanged.

### FIN-016: Handle a failed settlement

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Mark an approved disposable batch failed, inspect its ledger, and create a replacement batch through the supported workflow.

**Expected result:** The failure persists. Eligible open entries are released for a new batch without being paid twice.

### FIN-017: Preserve financial audit records

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Create a commission change and settlement decision, then inspect authorized audit history.

**Expected result:** Entries identify the actor, resource, state change, and required reason. Financial writes cannot succeed without the required evidence.

### FIN-018: Protect refund ownership

Priority: P0. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Attempt the customer refund endpoint using the tested administrative role's session for a customer's order.

**Expected result:** The current customer-owned endpoint denies the administrative request. Operator-initiated refunds require a separately implemented and tested authorization path.

### FIN-019: Deny catalog and security changes

Priority: P0. Test mode: Application. Run as: Finance operator.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Attempt product changes, seller approval, team management, role grants, user deletion, and platform security changes.

**Expected result:** Finance access does not authorize those actions. Target records remain unchanged.

### FIN-020: Check finance scope and exports

Priority: P1. Test mode: Application. Run as: Finance operator, Super admin.

**Before you start:** Sign in as Finance operator for the primary run, or as Super Admin for a listed repeat. Prepare captured, failed, pending, and refunded payments, seller ledger entries, commission rules, and settlements in every supported state.

**Test steps:** Use every exposed report period, status filter, and export control, then compare output with the displayed financial records.

**Expected result:** Supported controls use the selected scope and correct totals. A decorative or unavailable export is recorded as a gap.

<a id="ops-cases"></a>

## Returns and operations workflows

Default context: Returns operations.

Before this group: Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

Source references: [internal/application/commerce/returns.go](../internal/application/commerce/returns.go), [internal/transport/web/return_handlers.go](../internal/transport/web/return_handlers.go), [internal/web/handlers/portal_routes.go](../internal/web/handlers/portal_routes.go), [web/admin/portal_views.templ](../web/admin/portal_views.templ), [internal/adapters/postgres/return_eligibility_integration_test.go](../internal/adapters/postgres/return_eligibility_integration_test.go).

### OPS-001: Open operations queues

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Open the operations dashboard, orders, returns, refunds, and every exposed dispute view.

**Expected result:** Authorized queues load real records or honest empty states. A missing workflow is recorded instead of accepted as complete.

### OPS-002: Inspect a return request

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Open a pending return and compare its owner, order, delivery state, reason, and current status.

**Expected result:** The return relates to the correct delivered order. The customer reason is preserved.

### OPS-003: Approve a pending return

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Approve a valid request with an appropriate decision note and refresh customer and seller views.

**Expected result:** The approved state persists consistently. Customer return instructions become available through the supported path.

### OPS-004: Reject a pending return

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Reject another request with a clear note and inspect the customer view.

**Expected result:** The rejection persists and is visible to the customer. The order is not falsely marked refunded.

### OPS-005: Record receipt of an approved return

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Mark an approved request received with a receipt note.

**Expected result:** The received state persists and becomes eligible for the current customer-owned refund flow. No provider refund is fabricated.

### OPS-006: Reject receipt before approval

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Attempt to mark a pending or rejected return received.

**Expected result:** The transition is denied. The original status remains intact.

### OPS-007: Reject repeated terminal changes

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Attempt another decision on an already received or otherwise terminal return.

**Expected result:** The server rejects conflicting transitions or safely recognizes a replay. The existing result remains consistent.

### OPS-008: Handle competing return decisions

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Have two authorized sessions approve and reject the same pending request concurrently.

**Expected result:** Only one valid transition wins. The losing session receives a conflict or refreshed state.

### OPS-009: Validate return updates

Priority: P1. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Submit unsupported statuses, malformed identifiers, and decision notes over one thousand characters.

**Expected result:** Invalid requests are rejected without changing the return.

### OPS-010: Inspect marketplace order operations

Priority: P1. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Open known unpaid, paid, processing, shipped, delivered, cancelled, and refunded orders.

**Expected result:** Order state matches customer and seller records. Operations cannot alter payment state through editable page values.

### OPS-011: Read refund progress

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Inspect a return before refund, during provider processing, and after a processed refund.

**Expected result:** Operational views reflect the real refund state. Receiving goods is distinct from completing payment reversal.

### OPS-012: Protect financial permissions

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Attempt commission changes, settlement creation, payout decisions, and another customer's refund endpoint.

**Expected result:** Operations access does not grant finance administration or customer impersonation. No financial mutation succeeds.

### OPS-013: Protect unrelated administration

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Attempt catalog edits, seller team changes, role assignment, user deletion, and security settings.

**Expected result:** Every action outside operations permissions is denied.

### OPS-014: Search and filter return queues

Priority: P1. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Use supported order, status, and request searches and navigate several pages.

**Expected result:** Results match the selected scope. An empty search does not show fabricated returns.

### OPS-015: Retain operations audit evidence

Priority: P0. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Approve and receive a return, then inspect its audit trail from an authorized administrator.

**Expected result:** Accepted transitions identify the actor, request, time, and notes where supported. Rejected changes do not create a false success history.

### OPS-016: Link a dispute to its underlying records

Priority: P1. Test mode: Application. Run as: Returns operations.

**Before you start:** Sign in as Returns operations. Prepare delivered and undelivered customer orders, pending returns, approved returns, rejected returns, received returns, and refund records.

**Test steps:** Open an available dispute entry and inspect its linked order, return, support case, and permitted next action.

**Expected result:** Links refer to the correct authorized records. If no persisted dispute workflow exists, record the blocked scope and run the release-gap case.

<a id="agt-cases"></a>

## Support agent workflows

Default context: Support agent.

Before this group: Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

Source references: [internal/application/commerce/support.go](../internal/application/commerce/support.go), [internal/application/commerce/support_attachments.go](../internal/application/commerce/support_attachments.go), [internal/transport/web/support_handlers.go](../internal/transport/web/support_handlers.go), [internal/web/handlers/live_portal.go](../internal/web/handlers/live_portal.go), [web/supportportal/](../web/supportportal/).

### AGT-001: Read the support dashboard

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Open the dashboard and compare open-case counts with the actual case list.

**Expected result:** Counts agree with persisted cases and the selected scope. Failed loading is not displayed as an empty healthy queue.

### AGT-002: Find and open a case

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Search a known case subject or reference and open the selected conversation.

**Expected result:** The intended case opens with its real owner, linked order, and chronological message history.

### AGT-003: Separate customer and seller cases

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Open one customer case and one seller owner case with similar subjects.

**Expected result:** Each case retains its correct owner type and order context. Messages are not combined between cases.

### AGT-004: Inspect necessary customer context

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Open customer or seller context from a case and inspect the displayed profile and history.

**Expected result:** Only supported information needed for support is shown. Credentials, tokens, and unrelated private records are absent.

### AGT-005: Inspect linked order context

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Open the linked order and compare items, payment summary, shipment, and return state with the customer view.

**Expected result:** The summary agrees with the linked order and does not expose payment credentials or security data.

### AGT-006: Reply publicly to a customer

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Send a meaningful reply on Customer A's case, then reopen the case as Customer A and Customer B.

**Expected result:** Customer A sees the reply with the correct author and time. Customer B cannot read it.

### AGT-007: Reply publicly to a seller

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Send a reply on Seller A's owner case and reopen it as that owner.

**Expected result:** The reply appears in the same seller conversation and persists after reload.

### AGT-008: Validate support replies

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Submit an empty reply, whitespace-only text, and a reply above four thousand characters.

**Expected result:** Invalid replies are rejected without creating blank or truncated messages.

### AGT-009: Save a private case note

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Use the case status or note action to add an internal note. Inspect customer and seller conversations afterward.

**Expected result:** The note is visible only to authorized staff. Public participants do not see internal notes.

### AGT-010: Move a case through supported states

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Move separate cases through open, in progress, waiting for customer, resolved, and closed using supported actions.

**Expected result:** Accepted states persist and update queue counts. Unsupported transitions receive clear feedback.

### AGT-011: Validate a case status update

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Submit an unsupported status, malformed case identifier, and note above one thousand characters.

**Expected result:** Invalid updates are rejected. The prior case state remains unchanged.

### AGT-012: Handle a customer follow-up

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Set a case to waiting for customer, have its owner reply, and reopen the agent view.

**Expected result:** The new reply is visible and the saved case state follows the documented workflow. A status that requires manual action is not silently assumed to change.

### AGT-013: Attach supported images

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Attach up to three valid PNG, JPEG, or WebP images to a supported reply and download them from authorized views.

**Expected result:** Images retain the correct case and message association. Authorized participants can open the original images.

### AGT-014: Validate attachment limits

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Submit four images, an empty image, a file above 10 MiB, an unsupported format, and a renamed non-image.

**Expected result:** Invalid attachments are rejected with clear feedback. They do not create a false complete message or unsafe downloadable object.

### AGT-015: Protect attachment downloads

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Copy a private attachment URL and open it signed out and as an unrelated customer or seller.

**Expected result:** Unauthorized downloads are denied. Guessing a URL does not expose the image.

### AGT-016: Handle attachment storage failure

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Interrupt storage while sending an attachment, then reopen the conversation and retry through the supported path.

**Expected result:** Failure is reported honestly. Any saved text and retry behavior are clear, and no broken file is shown as a successful attachment.

### AGT-017: Escape message content

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Send harmless script-like text as a public message and private note, then view each supported recipient page.

**Expected result:** Content is rendered as text. It cannot execute script in a customer, seller, or agent session.

### AGT-018: Handle concurrent case work

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Have two agents update the same case and add replies close together.

**Expected result:** Accepted replies retain their authors and order. Conflicting state changes do not silently erase history.

### AGT-019: Search and page support records

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Use every supported case status, participant, order, and date filter. Navigate multiple pages.

**Expected result:** Results match supported selections and persist through refresh. Unsupported controls are recorded explicitly.

### AGT-020: Prevent support privilege escalation

Priority: P0. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Attempt refunds, payment-state changes, commissions, payouts, catalog changes, seller approval, and role grants as the agent.

**Expected result:** Support access does not authorize these actions. Assistance cannot become financial or security administration.

### AGT-021: Preserve support history

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Resolve a case, sign out and in, and reopen its complete history.

**Expected result:** Public messages, internal notes, state changes, and attachments remain associated with the correct case.

### AGT-022: Handle missing case records

Priority: P1. Test mode: Application. Run as: Support agent.

**Before you start:** Sign in as Support agent. Prepare Customer A and Customer B cases, a seller owner case, public replies, private notes, owned orders, and supported attachment fixtures.

**Test steps:** Open and mutate nonexistent case and attachment identifiers.

**Expected result:** The application returns a controlled not-found or validation result. It does not substitute another case.

<a id="ui-cases"></a>

## Usability, accessibility, and browser checks

Default context: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

Before this group: Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

Source references: [web/pages/](../web/pages/), [web/selleradmin/](../web/selleradmin/), [web/admin/](../web/admin/), [web/supportportal/](../web/supportportal/), [tests/e2e/role-portals.spec.js](../tests/e2e/role-portals.spec.js), [playwright.config.js](../playwright.config.js).

### UI-001: Use narrow and wide layouts

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Open representative forms, lists, detail pages, and navigation at widths of 360, 390, 768, 1024, and 1440 pixels.

**Expected result:** Content and actions remain readable and usable. The page does not overflow horizontally except within a deliberate dense-table scrolling area.

### UI-002: Use the mobile navigation drawer

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Where a drawer exists, open it, select a link, close it, and press Escape. Repeat after changing screen width.

**Expected result:** The drawer works consistently. It does not trap scrolling or leave an invisible overlay over the page.

### UI-003: Use keyboard navigation

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Complete a representative workflow using Tab, Shift+Tab, Enter, Space, and Escape without a mouse.

**Expected result:** Every essential control is reachable and has visible focus. Focus order follows the page's reading and action order.

### UI-004: Manage drawer and dialog focus

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Open and close a drawer or confirmation dialog using the keyboard and inspect focus before and after it.

**Expected result:** Focus enters the active surface, stays within it when appropriate, and returns to the triggering control on close.

### UI-005: Read forms with assistive technology

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Use an available screen reader to inspect headings, field labels, required fields, errors, buttons, and state changes.

**Expected result:** Controls have meaningful names and roles. Errors and important updates can be understood without relying only on visual position.

### UI-006: Use zoom and larger text

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Zoom to 200 percent and 400 percent, then complete a representative form and navigate its result.

**Expected result:** Text remains readable and essential actions remain reachable. Content is not clipped or overlapped.

### UI-007: Recognize states without color

Priority: P2. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Inspect success, failure, stock, payment, and return states using grayscale or a color-vision check.

**Expected result:** Meaning is conveyed through readable labels or icons as well as color. Text has adequate contrast for the chosen design.

### UI-008: Handle long and multilingual content

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Use long product names, names with Indian-language characters, long order references, and multiline messages.

**Expected result:** Text is preserved and safely displayed. Layout and searching do not corrupt or hide essential content.

### UI-009: Understand validation errors

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Submit a representative invalid form, correct one field at a time, and resubmit.

**Expected result:** Feedback identifies the affected input and allows recovery. Valid values are preserved where appropriate.

### UI-010: Cancel a confirmation

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Cancel an archive, removal, payout, or deletion confirmation that exists for the role.

**Expected result:** No write is accepted. The record and current page remain usable.

### UI-011: Avoid duplicate writes while loading

Priority: P0. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Click a supported save or submit action repeatedly while its response is delayed.

**Expected result:** The server prevents duplicate business effects. The interface gives understandable progress or conflict feedback.

### UI-012: Use refresh and direct links

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Open a detail page directly, refresh it, and copy its URL into an authorized browser.

**Expected result:** The same authorized record loads. Direct navigation does not depend on a previous client-side page state.

### UI-013: Recover after a network interruption

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Interrupt the network during a reversible test action, restore it, and inspect the stored result before retrying.

**Expected result:** The interface reports failure or uncertainty honestly. A retry cannot silently duplicate an already accepted change.

### UI-014: Use core HTML fallbacks

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Disable JavaScript and complete supported sign-in, browsing, form submission, and role navigation paths.

**Expected result:** Supported server-rendered workflows remain usable. Features requiring JavaScript show a clear limitation instead of false success.

### UI-015: Check supported browser engines

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Complete the role's critical workflow in Chromium, Firefox, and WebKit, or the project's explicitly approved browser list.

**Expected result:** Supported browsers reach the same saved result without blocking layout or script errors. Record the actual engine and version.

### UI-016: Show useful empty states

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Open a scoped workspace with no matching orders, products, returns, cases, or financial records.

**Expected result:** The page explains the empty state and preserves navigation. It does not display fabricated records.

### UI-017: Check page assets and errors

Priority: P1. Test mode: Application. Run as: Guest, Customer, Seller owner, Seller staff, Marketplace admin, Super admin, Support agent, Finance operator, Returns operations.

**Before you start:** Use the listed role's actual pages and representative long names, empty lists, multi-page lists, validation errors, and uploaded media. Test the browser and viewport matrix in the preparation guide.

**Test steps:** Inspect browser errors and failed network requests while completing the role's critical workflow.

**Expected result:** Required scripts, styles, images, and documents load successfully. No unexplained error blocks the workflow.

<a id="e2e-cases"></a>

## Complete workflows across roles

Default context: Cross-role team.

Before this group: Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

Source references: [tests/e2e/portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js), [tests/e2e/seller-support-live.spec.js](../tests/e2e/seller-support-live.spec.js), [internal/application/commerce/](../internal/application/commerce/), [internal/application/payment/](../internal/application/payment/).

### E2E-001: Complete a marketplace sale

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Have the seller create a stocked draft, Marketplace admin approve it, and a customer purchase it through sandbox payment. Have the seller process, ship, and deliver the order, then inspect finance and customer records.

**Expected result:** Every role sees the same durable order and correct scope. Payment, stock, shipment, customer receipt, and seller ledger agree.

### E2E-002: Complete a seller onboarding journey

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Create a seller owner account, complete onboarding, approve it as Marketplace admin, and create its first reviewed listing.

**Expected result:** The seller progresses from pending to active before restricted sales work. Its first approved product is discoverable and purchasable.

### E2E-003: Complete a restricted staff journey

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Have the owner grant catalog and fulfillment permissions, then have staff create a listing and fulfill a paid order. Revoke fulfillment access and retry.

**Expected result:** Granted staff work succeeds within the seller. Revoked and owner-only actions are denied without damaging the order.

### E2E-004: Complete a multi-seller purchase

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Buy products from Seller A and Seller B in one supported checkout. Have each seller fulfill only its own items.

**Expected result:** One customer order has correctly scoped seller portions. Each seller sees and changes only its own shipment and earnings.

### E2E-005: Keep a partly fulfilled order accurate

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** In a multi-seller order, deliver Seller A's shipment while Seller B remains processing. Inspect customer status and return eligibility.

**Expected result:** The order does not claim complete delivery prematurely. Return eligibility and shipment details follow the actual aggregate rule.

### E2E-006: Complete a cancelled checkout

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Create a pending customer order, cancel it, inspect released stock as the seller, and inspect finance records.

**Expected result:** All roles agree that the order is cancelled and unpaid. Stock is released once, and no sale is settled.

### E2E-007: Complete a return and refund

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Deliver an order, request a return as its customer, approve and receive it as operations, then submit the customer-owned sandbox refund. Inspect customer, seller, and finance views.

**Expected result:** One processed full refund reaches the correct order. Return history, payment reversal, stock treatment, and seller ledger follow the implemented rules consistently.

### E2E-008: Complete a rejected return

Priority: P1. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Request a return for a delivered order, reject it as operations, and try the customer refund action.

**Expected result:** The customer sees the rejection. The refund is denied without a received return, and finance remains unchanged.

### E2E-009: Complete customer support resolution

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Have a customer open an order-linked case with an image, have support add a public reply and private note, and have the customer reply before resolution.

**Expected result:** The conversation is durable and belongs to the correct order. Public messages are shared, private notes remain private, and resolution is visible.

### E2E-010: Complete seller owner support

Priority: P1. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Have a seller owner open an owned-order case, exchange replies with support, and resolve it through supported controls.

**Expected result:** The case remains in the seller conversation and agent queue. Unrelated sellers and owner-restricted staff cannot access it.

### E2E-011: Complete review publication

Priority: P1. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Deliver a purchase, submit a customer review, publish it as Marketplace admin, and inspect the product as a guest.

**Expected result:** The review appears only after moderation and retains verified-purchase status. Product summaries reflect the published review.

### E2E-012: Complete a promotion purchase

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Create and activate a marketplace campaign, use its coupon in customer checkout, and inspect the resulting finance and seller records.

**Expected result:** The same discount and order total appear across roles. Usage is counted once, and financial records reconcile.

### E2E-013: Complete a local settlement journey

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Complete a local demo paid sale, have finance create and approve a settlement, record its mock payout, and inspect seller earnings.

**Expected result:** The ledger is included once and the settlement agrees across roles. The run proves local state transitions only, without proving a bank payout.

### E2E-014: Complete account suspension and recovery

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Keep a disposable operational user working, suspend it as Super Admin, retry its open action, reactivate it, and sign in again.

**Expected result:** Suspension stops current protected work. Reactivation restores only its explicit assignments, with audit evidence.

### E2E-015: Complete customer privacy deletion

Priority: P0. Test mode: Application. Run as: Cross-role team.

**Before you start:** Keep separate browser sessions for every participating role. Use unique QA record names and track the same seller, product, order, case, return, and settlement identifiers throughout each journey.

**Test steps:** Create a disposable customer's address, wishlist, order, and privacy request. Complete deletion and inspect customer, finance, and support views.

**Expected result:** Authentication and personal data access stop. Retained financial records remain consistent, and deleted personal fields follow the retention matrix.

<a id="int-cases"></a>

## Service, payment, persistence, and recovery checks

Default context: QA and technical operator.

Before this group: Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

Source references: [internal/adapters/postgres/](../internal/adapters/postgres/), [internal/adapters/redis/](../internal/adapters/redis/), [internal/adapters/razorpay/](../internal/adapters/razorpay/), [internal/adapters/storage/](../internal/adapters/storage/), [internal/worker/](../internal/worker/), [cmd/website/main.go](../cmd/website/main.go), [docs/api/openapi.yaml](../docs/api/openapi.yaml).

### INT-001: Verify a valid payment event

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Send a genuine signed sandbox capture event for the exact pending payment and inspect the order, reservation, and financial records.

**Expected result:** Only the matching amount, currency, provider order, and payment settle the order. Stock commits atomically with the accepted payment.

### INT-002: Reject an invalid payment signature

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Send an altered event body and an invalid signature for a known pending order.

**Expected result:** The event is rejected. Order, payment, stock, and ledger state remain unchanged.

### INT-003: Reject a mismatched payment event

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Send controlled sandbox events with an incorrect amount, currency, provider reference, or unsupported event type.

**Expected result:** Mismatched and unsupported events cannot mark an order paid. The failure is traceable without exposing secrets.

### INT-004: Replay an accepted payment event

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Resend the same verified capture event several times, including concurrent deliveries.

**Expected result:** The order settles once. Stock, ledger, notifications, and audit records have no duplicate business effect.

### INT-005: Race payment with cancellation or expiry

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** In a disposable environment, submit payment confirmation near cancellation and reservation expiry. Inspect the final records.

**Expected result:** The system reaches one consistent state under its transition rules. It never captures payment while silently losing the required inventory allocation.

### INT-006: Verify checkout callback data

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Alter payment identifiers or the checkout verification signature in a customer's callback request, then repeat with valid sandbox data.

**Expected result:** Invalid data cannot mark an order paid. Only server-verified provider data reaches the matching customer order.

### INT-007: Prevent overselling under concurrency

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Set a variant to ten available units and submit one hundred simultaneous one-unit checkout attempts from independent customers.

**Expected result:** Accepted reservations never exceed ten units. Remaining requests receive clear stock conflicts, and no quantity becomes negative.

### INT-008: Handle identical concurrent checkout retries

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Submit twenty identical checkout requests with the same customer, cart, and key simultaneously.

**Expected result:** One durable order and reservation are created. Successful retries refer to that order without partial duplicate records.

### INT-009: Expire reservations safely

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Create unpaid reservations, wait using the configured test clock, and run the reservation worker repeatedly.

**Expected result:** Only expired eligible reservations are released, once each. Paid orders keep their committed inventory.

### INT-010: Enforce a received return before refund

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Attempt the owning customer's refund while its return is pending, approved, rejected, and received in separate fixtures.

**Expected result:** Only the received-return path reaches the provider under the current rules. Other states remain financially unchanged.

### INT-011: Retry a refund safely

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Submit an eligible full refund repeatedly with the same request key, including concurrent requests.

**Expected result:** One provider refund and one durable refund record result. Replays return the stored result without paying twice.

### INT-012: Reject changed refund keys or amounts

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Alter the refund key for an existing request and attempt to submit a different amount or currency.

**Expected result:** The server enforces its full-refund and idempotency contract. Client values cannot change the authoritative refund amount.

### INT-013: Handle a pending or failed provider refund

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Use controlled provider pending, failed, timeout, and mismatched-response scenarios. Retry only through the supported recovery path.

**Expected result:** The order is not falsely marked refunded. Provider references and uncertainty are retained so recovery does not duplicate a refund.

### INT-014: Check order confirmation and dispatch mail

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Complete one paid order and one shipment, run the workers, and inspect the test mailbox.

**Expected result:** Messages reference the correct order, customer, shipment, and public URL. No message is sent to another account.

### INT-015: Recover from an email failure

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Interrupt SMTP during a supported notification, restore it, and run the outbox worker again.

**Expected result:** The business transaction remains durable. The worker retries safely and reports unresolved delivery rather than silently losing it.

### INT-016: Recover a interrupted outbox worker

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Stop a worker during processing, restart it, and inspect the affected order, mail, and search event.

**Expected result:** Durable events remain recoverable. Reprocessing does not duplicate financial or inventory effects.

### INT-017: Use PostgreSQL search fallback

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Make Meilisearch unavailable and repeat product search, category filters, sorting, and pagination.

**Expected result:** Search falls back to authoritative PostgreSQL with safe results. Unapproved or stale products do not become public.

### INT-018: Rebuild a stale search index

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Change and archive catalog products, run the supported index rebuild, and compare public results with PostgreSQL.

**Expected result:** Current approved data replaces stale index state. Archived records and stale prices do not remain purchasable.

### INT-019: Handle a required database outage

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Interrupt PostgreSQL in a disposable environment, try a protected read and write, and inspect readiness.

**Expected result:** Readiness and user responses report the failure. No fabricated success, empty healthy queue, or partial business write is produced.

### INT-020: Handle a Redis outage

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Interrupt Redis, try sign-in and a protected mutation, then restore it.

**Expected result:** Session and rate-limit failures follow the configured safe behavior. The application does not authorize requests because the dependency failed.

### INT-021: Handle object storage failure

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Interrupt storage during additional-media upload and public media access, then restore it.

**Expected result:** Errors are explicit, existing catalog records remain intact, and retry does not produce false ready files.

### INT-022: Restart with durable business data

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Create an order, product cover, support message, and preference, then restart the application and dependencies using their persistence configuration.

**Expected result:** PostgreSQL records and persisted files survive. The application resumes with consistent state and health.

### INT-023: Upgrade an existing database

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Restore a supported prior-version backup in a disposable stack and start the candidate application so embedded migrations run.

**Expected result:** The upgrade completes or fails safely with diagnostics. Required orders, accounts, inventory, and historical records remain intact.

### INT-024: Restore a backup

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Restore the test database, uploaded-cover volume, and object storage into a clean isolated environment. Reopen representative records.

**Expected result:** Accounts, orders, stock, messages, and media reconcile with the backup point. The documented restore procedure produces a usable application.

### INT-025: Validate the JSON API contract

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Call documented authentication, catalog, search, cart, checkout, order, wishlist, privacy, and media endpoints with valid and invalid input.

**Expected result:** Responses follow the documented JSON structure and status codes. API errors never return rendered HTML or internal stack traces.

### INT-026: Deny unsupported HTTP methods

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Send GET to mutation-only endpoints and unsupported methods to representative API and web routes.

**Expected result:** Unsupported methods cannot perform writes. The application returns a controlled method or access response.

### INT-027: Isolate local demo endpoints

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Start the candidate with local mode off and request development payment and local-only API routes.

**Expected result:** Local-only helpers are not callable for production actions. They cannot simulate payment capture or grant administrative powers.

### INT-028: Inspect secure session handling

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** In the HTTPS release environment, inspect session cookie flags, log out, and try reusing the previous session identifier.

**Expected result:** Cookies follow the configured Secure, HttpOnly, and SameSite policy. Logged-out or expired identifiers cannot authorize new work.

### INT-029: Protect caches from private data

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Open two users' protected pages through the deployed proxy or cache and inspect response caching behavior.

**Expected result:** Private account, role, order, and support content is not shared between sessions. Public caching cannot disclose protected data.

### INT-030: Check a representative workload

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Run the documented staged public load smoke and the checkout concurrency cases with recorded resources, duration, and data volume.

**Expected result:** Error rate and latency distributions are compared with approved budgets. A local smoke result alone does not claim production capacity.

### INT-031: Verify operational diagnostics

Priority: P1. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Trigger a controlled dependency failure and follow its request identifier through logs, health, and available metrics.

**Expected result:** Operators can locate the failing boundary without exposing credentials or personal message content. Recovery changes health back to its true state.

### INT-032: Enforce coupon usage under concurrency

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Set a coupon usage limit to one and submit eligible checkouts concurrently from independent customers. Repeat one accepted checkout with the same key.

**Expected result:** Coupon usage follows its defined reservation or consumption rules without exceeding the limit. Identical checkout retries do not consume additional usage.

### INT-033: Protect anonymous cart submissions

Priority: P0. Test mode: Integrated environment. Run as: QA and technical operator.

**Before you start:** Use an isolated integrated environment with PostgreSQL, Redis, storage, SMTP, workers, and sandbox payment access. Keep read-only evidence access to database records, provider records, and logs.

**Test steps:** Submit guest cart changes with missing and altered anonymous request tokens, including a request from an unrelated origin.

**Expected result:** Anonymous mutation defenses reject forged requests according to the configured policy. A guest token cannot address another browser's cart.

<a id="gap-cases"></a>

## Requirements and release gaps

Default context: Release owner.

Before this group: Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

Source references: [Prd.md](../Prd.md), [docs/phase-status.md](../docs/phase-status.md), [docs/privacy-retention.md](../docs/privacy-retention.md), [docs/processor-inventory.md](../docs/processor-inventory.md), [internal/domain/commerce/models.go](../internal/domain/commerce/models.go), [web/admin/portal_views.templ](../web/admin/portal_views.templ).

### GAP-001: Verify a real seller payout

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Run the approved payout integration in its provider sandbox and reconcile an eligible seller settlement with an actual provider payout reference and callback.

**Expected result:** A real integration confirms the payout exactly once and updates the ledger. A local Record mock payout result does not pass this requirement.

### GAP-002: Verify operator-initiated refunds

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Run the approved finance or operations refund workflow for a customer's eligible order, including denied roles and duplicate requests.

**Expected result:** The operator acts under an explicit audited grant without impersonating the customer. If no operator workflow exists, record the release gap.

### GAP-003: Verify persisted dispute processing

Priority: P1. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Create a dispute, attach the related order and evidence, record an authorized decision, and reopen it across operations and customer views.

**Expected result:** The dispute has durable states, ownership, history, and authorized resolution. A filter over return records is not proof of a dispute workflow.

### GAP-004: Verify seller staff return processing

Priority: P1. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** If RETURN_PROCESS is approved for release, grant it independently and test one allowed seller return action and denied foreign actions.

**Expected result:** The grant has a real server-enforced workflow. The current eight-permission model contains RETURN_READ only, so record missing RETURN_PROCESS explicitly.

### GAP-005: Verify privileged multifactor authentication

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** If required for release, enroll an administrator and finance account, test valid and invalid second factors, recovery, and session reuse.

**Expected result:** Required privileged actions cannot bypass the second factor. Ordinary password sign-in does not count as multifactor coverage.

### GAP-006: Verify controlled emergency access

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** If required for release, exercise the approved emergency administration process, its expiry, audit trail, and removal in an isolated environment.

**Expected result:** Emergency privileges are limited and traceable. Normal user creation cannot become an undocumented emergency access path.

### GAP-007: Verify privacy request fulfillment

Priority: P1. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Process an access, correction, and grievance request through the approved operational workflow and inspect the recipient's result.

**Expected result:** Each request reaches a documented completed outcome with identity and data protections. Creating a request row alone does not prove fulfillment.

### GAP-008: Verify approved return and financial policies

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Compare implemented return eligibility, refund scope, shipping charges, taxes where required, commission rules, and retention with the approved release policies.

**Expected result:** Implemented rules match accepted policies, including boundaries and financial rounding. Unapproved or missing rules remain release gaps rather than assumed requirements.

### GAP-009: Verify production provider configuration

Priority: P0. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** Run release checks with HTTPS, production-like SMTP, persistent storage, verified payment callbacks, and restricted development endpoints.

**Expected result:** Configured integrations work end to end with durable evidence. Local deterministic providers cannot substitute for this run.

### GAP-010: Verify genuine shipping documents and tracking

Priority: P1. Test mode: Release requirement. Run as: Release owner.

**Before you start:** Use the PRD and implementation status with the release candidate. Obtain the accepted scope and implementation details for each requirement before running it.

**Test steps:** If carrier integration is in release scope, create a provider sandbox shipment, obtain its label, and receive a tracking update.

**Expected result:** The label and tracking refer to the actual provider shipment. Locally stored tracking text or return instructions do not prove carrier integration.

## Execution order and acceptance

1. Verify the environment and prepare separate accounts and records.
2. Run AUTH and NAV for all eight signed-in roles. Resolve critical sign-in and permission failures before relying on role results.
3. Run GST and CUS, then OWN and STF. These groups establish customer data, seller catalog, stock, and fulfillment.
4. Run MKT and SUP, including the listed Super Admin repeat cases. Prepare the shared moderation and administration outcomes.
5. Run AGT, OPS, and FIN. Prepare support decisions, return receipt, financial visibility, and local settlement results.
6. Complete every E2E journey using separate role sessions and the same linked records.
7. Run UI for guest and all roles, then INT with configured real boundaries.
8. Resolve every applicable GAP case for the accepted release scope. Run provider and deployment checks in the representative release environment.
9. Fix failures and repeat the affected cases and complete journeys. Preserve evidence for both the original failure and the retest.

Call the tested build operational for its accepted scope only when every applicable role workflow has passed, all P0 cases have passed, no unresolved defect prevents a required workflow, and integration and recovery evidence is complete. Record remaining lower-priority defects and any approved scope exclusions.

A successful local demo demonstrates only the tested local behavior. Real payment, payout, shipping where required, mail, persistence, deployed permission checks, backup restoration, and approved policy gaps need their own evidence. Test completion cannot guarantee that every possible input or future failure has been covered.

### Minimum handoff record

Complete these fields for a release decision:

- Tested revision and deployment URL.
- Environment, payment mode, provider boundaries, and migration version.
- Counts of Passed, Failed, Blocked, Not run, and Not applicable execution rows.
- Results for each role and each critical E2E journey.
- Open defect identifiers, affected workflows, and retest evidence.
- Accepted scope exclusions and approving owner.
- Links to screenshots, logs, provider references, and restoration evidence.
- Final decision, decision owner, and decision date.

## Existing automated coverage

The repository contains automation that can support these procedures. Its presence is not evidence of a successful run for this document.

| Existing source | Related manual coverage | Important limit |
| --- | --- | --- |
| [role-portals.spec.js](../tests/e2e/role-portals.spec.js) | AUTH, NAV, UI | Samples role landing, a denied portal, and responsive layout; does not cover every authorization action. |
| [portal-workflows.spec.js](../tests/e2e/portal-workflows.spec.js) | CUS, OWN, MKT, SUP, FIN, OPS, AGT, E2E | Includes local simulated payments and mock payouts; some mutation tests can skip outside the local target. |
| [seller-support-live.spec.js](../tests/e2e/seller-support-live.spec.js) | OWN, STF, AGT | Checks persisted seller support and role denials. |
| [integration-gaps.spec.js](../tests/e2e/integration-gaps.spec.js) | GST, MKT, FIN, SUP | Exercises selected catalog directory, setting, and commission boundaries. |
| [reported-issues.spec.js](../tests/e2e/reported-issues.spec.js) | OWN, SUP | Covers reported product image and seller selection behavior. |
| [PostgreSQL integration tests](../internal/adapters/postgres/) | INT, permissions, checkout, returns, refunds, media, and privacy | Require configured test resources; mocks cannot establish real persistence evidence. |
| [Application service tests](../internal/application/) | Business validation and transition rules | Do not establish complete browser or provider workflows. |
| [Performance load plan](performance-load-plan.md) | INT-030 | A local staged load smoke is not a production capacity decision. |

Use the commands documented by the repository for the test environment, including `npm run test:e2e` for Playwright and the appropriate Go package tests. Record actual outcomes and skipped cases. The [9 October execution report](qa/QA-20261009-01/report.md) records the application checks actually executed and distinguishes full procedures from supporting automation.

