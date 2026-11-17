# Guided Fulfillment implementation record

The owner approved the flow and visual style of these four concepts. The primary
implementation workflow is frontend-app-builder, with frontend-design-premium and
frontend-design supporting the existing Go/templ application. Browser/IAB was
unavailable; authenticated Playwright Chromium uses disposable PostgreSQL/Redis,
synthetic accounts and no live payment/shipping worker.

## Fidelity ledger

| Accepted feature | Implementation and comparison |
| --- | --- |
| Forest navigation and sage Fulfillment selection | Scoped forest #123f32, nine accepted destinations, white Inter wordmark, sage active row; order navigation never shares the active state. |
| Ivory main and warm white panels | #f7f5ee background, #fffdf8 surfaces and #dfd8ca borders are explicit tokens in fulfillment-guided.css. |
| Product overview with missing details | Named rows, seller and SKU, real four-task completion count, amber missing tasks and Continue setup. No task forms on the overview. |
| One open task and progress rail | Six native links with numbered/check states and Complete/Missing/Pending text. Only the selected task form is rendered. |
| Invoice form anatomy | Legal name and address, two-column native State/GST selection, conditional GSTIN, fixed saved prefix, audit note and Back / Save and continue. Shared textarea owner honors the approved compact minimum and grows for content. |
| Parcel form anatomy | Weight in grams, technical line diagram to the right, three dimensions in cm, measurement warning, and Back / Save and continue. |
| Review and separate checkout gate | Verified/Saved checklist with Edit links, green setup completion only after all details pass, amber launch requirements and separate pilot task. |
| Restrained typography and controls | Inter interface, Roboto headings, 38px main heading, 30px task headings, 17px fields, at least 44px controls, 6px control radius. Old global font overrides explicitly exclude this scoped surface. |
| Phone continuation | Shared mobile navigation with focus return, same single form, progress disclosure initially collapsed on phones, native keyboard operation and natural scrolling. |

The accepted generated images vary in sidebar width, wordmark font and progress
rail framing. The refined invoice reference supplies the shared shell/rail:
264px sidebar, 296px rail and 16px gap. The parcel arrangement and review hierarchy
follow their state references. The overview Missing chip background is #fff8e8: the original darker fill gave 4.33:1 contrast with the accepted #9a6700 text; this lighter fill preserves the text color and raises contrast to 4.60:1. No image is used as an interactive UI background.

## Copy and data audit

Real catalog names, SKUs, profile values and readiness replace illustrative image
content. Fixture screenshots deliberately use synthetic names. The concept-only
footer is absent from the product. Verification notes remain required and audited.
Missing and saved states are server-derived. Save and continue recomputes the
first missing task, so already saved details do not require re-entry. Review
lists actual stock, policy, shipping connection, configuration and launch gates;
it never promises that saved profiles alone enable checkout. Success feedback is
shown after a save and is absent on a normal fresh visit.

Pilot, launch, policy publication and shipping pause each receive a dedicated page
in the same form system: they are necessary continuations of the existing business
workflow and retain backend confirmation and prerequisites. They do not create a
payment or a courier booking from setup. Existing operations and other-role shells
retain their established behavior.

## Verification boundaries

Runtime verification covers isolated profile/parcel saves, pickup lookup failure
and retry, server validation recovery, current-role authorization, CSRF-backed
forms, paused pilot rejection, customer policy/claim evidence, seller fulfillment
and invoice PDF download. Desktop reference size is 1536 by 1024; phone is 390px.
The check also inspects keyboard focus, narrow overflow and axe WCAG violations.
Production payment, courier collection, legal/tax correctness and actual package
measurements require genuine business inputs and are not inferred from fixtures.
Exact test outcomes and deployment revision belong to the release verification
record; this file does not assert live transactions were exercised.
