# Guided Fulfillment setup — visual concepts

Status: flow and visual style approved by the owner on 17 November 2026; implemented in the guided Super Admin Setup. See FIDELITY.md for the implementation contract and verification scope.

One product, one task at a time. Missing, Complete and Pending statuses identify what needs attention. Save and continue leads to the next required step; existing complete details remain available without re-entry.

Flow: Choose product → Invoice details → Pickup location → Packed measurements → Tax & return policy → Review → Pilot setup when authorized and eligible.

These images use illustrative statuses, not verified current production data. State, invoice, tax, stock, pickup and parcel validity remain server-enforced in implementation. Completing setup must not activate checkout automatically.

Screens:
1. 01-missing-details.png — overview and next action.
2. 02-invoice-step.png — one active invoice task.
3. 03-parcel-step.png — one active packed-measurement task.
4. 04-review.png — saved details and the separate launch/pilot gate.

Generated using the built-in image generation tool. The invoice image was refined to remove duplicate step numbers. The accepted concepts are retained as references; application UI is implemented as native Go/templ markup and scoped CSS. Deployment evidence is recorded separately against the released revision.

## Prompt set

### Screen 1

Use case: ui-mockup.
Create ONE high fidelity, implementation-ready desktop screenshot for WeevCrafts Super Admin, an Indian handmade marketplace. This is a new GUIDED PRODUCT SETUP design concept for owner review, not a live data screenshot. Landscape 1536 by 1024 composition, extremely crisp legible English text, calm professional product UI.

Brand: preserve an existing warm ivory and forest green operations workspace. Background #f7f5ee, white #fffdf8 surfaces, primary #123f32, ink #20362d, muted #56665d, border #dfd8ca, amber #9a6700 for Missing, green #246b45 for Complete. Inter interface typography, restrained Roboto headings. 44px controls, 6px corner radius, generous but useful spacing. Flat screenshot, not a device mockup. Use only a text wordmark 'WeevCrafts'; do not invent a new logo.

SCREEN 1: missing-details overview, BEFORE starting the guided flow.
Left sidebar about 220px wide: WeevCrafts / Super Admin, Dashboard, Orders & delivery, Fulfillment (correctly active), Sellers, Products, Inventory, Returns, Finance, Support. Small quiet icons.
Minimal top header 'Fulfillment', account at right. Main navigation 'Operations' and 'Guided setup', with Guided setup active. No global search bar or ornamental illustration.

Main heading: 'Finish product setup'
Supporting sentence: 'Choose a product. We will guide you through only the missing details.'
A compact seller dropdown labelled 'Seller' showing 'All sellers', and a search input 'Find a product'.
Below, a clear spacious TWO ROW product list, not a dashboard of cards, not a long form. Headings Product / Missing details / Next action.
Row 1: 'Tree of Life', secondary 'Kamala', amber statuses 'Invoice details missing' and 'Tax details missing'. Small line 'Pickup and parcel details complete'. One forest primary button 'Continue setup'.
Row 2: 'Handcrafted flower-brown', secondary 'WeevCrafts', amber statuses 'Invoice details missing', 'Packed measurements missing', 'Tax details missing'. Secondary outlined button 'Start setup'.
Make the Missing labels readable and meaningful, with an amber warning icon and words, not color alone. Each product row has a simple unfilled square thumbnail symbol, no invented product photograph.
Below the list a quiet note: 'Completing setup does not automatically open checkout.'
Small bottom corner 'Design concept · Illustrative status'.
Keep most of the screen light and uncluttered, one obvious next action. Do not include tax forms, invoice forms, parcel forms, charts, revenue, fake metrics, a hero banner or a tower of panels. This is a coherent workflow start screen that can actually be built in Go/templ with native controls.

### Screen 2

Use case: ui-mockup. Create ONE high-fidelity, implementation-ready desktop screenshot, landscape 1536x1024, for WeevCrafts Super Admin GUIDED PRODUCT SETUP. Crisp readable English text, practical HTML/native forms. It is a design concept with illustrative status, not a live screenshot.

Coordinated visual system: dark forest sidebar #123f32 with white 'WeevCrafts' wordmark and 'Super Admin', light sage active Fulfillment item; ivory canvas #f7f5ee, warm-white form surface #fffdf8, forest primary buttons, charcoal #20362d text, muted #56665d, amber #9a6700 for Missing and green #246b45 for Complete, thin #dfd8ca borders. Inter interface typography, restrained Roboto headings, 44px controls, 6px radius, no gradients. Sidebar entries Dashboard, Orders & delivery, Fulfillment active, Sellers, Products, Inventory, Returns, Finance, Support. Keep the layout calm and generously spaced. No ornamental images, charts, global search, invented data or unnecessary badges.

SCREEN 2: one focused invoice task inside a guided wizard. Main top bar 'Product setup' with a clear 'Back to products' text link. Under it product context 'Tree of Life' and secondary 'Kamala'. Small 'Step 2 of 6'.
A compact VERTICAL progress rail inside main area, NOT another app sidebar:
'1 Choose product' Complete with check
'2 Invoice details' Missing, active forest line
'3 Pickup location' Complete with check
'4 Packed measurements' Complete with check
'5 Tax & return policy' Missing
'6 Review' Pending
Only labels and status in this rail; no other forms visible.

To the right, one large SINGLE form pane, all content fitting the viewport.
Heading 'Add seller invoice details'
Subtext 'This profile is shared by all Kamala products.'
A compact amber inline warning: 'Missing: seller invoice details'
Fields with persistent clear labels, all empty placeholders rather than invented legal identity:
'Legal name' placeholder 'Enter registered legal name'
'Registered address' two-line textarea placeholder 'Enter invoice address'
A two-column row:
'State / Union Territory' native select placeholder 'Choose state'
'GST registration' native select placeholder 'Choose registration status'
Then 'Invoice prefix' compact input placeholder 'e.g. KAM', small help 'Choose once. Used for future invoice numbers.'
Then 'Verification note' compact input placeholder 'What did you check?'
No GSTIN input until GST registered is selected. Show hint 'GSTIN appears when registered is selected.'
Bottom clear action row within the pane: outlined 'Back', primary forest 'Save and continue'. Quiet text 'You can return to this step later.'
No other invoice, pickup, tax, parcel, policy or launch forms elsewhere on this screen. Important: no giant scrolling page; only the current task and compact progress navigation.
Tiny bottom-corner 'Design concept · Illustrative status'.

### Screen 3

Use case: ui-mockup. Generate ONE polished desktop product UI screenshot, landscape 1536x1024. Screen 3 of a coordinated WeevCrafts Super Admin guided setup concept. This is a design concept with illustrative statuses, not real business data.

Match this visual system precisely: dark forest #123f32 app sidebar, white WeevCrafts wordmark and Super Admin text; sidebar Dashboard, Orders & delivery, Fulfillment active in light sage, Sellers, Products, Inventory, Returns, Finance, Support. Ivory main canvas #f7f5ee, warm-white surface #fffdf8, charcoal #20362d, muted #56665d, thin #dfd8ca border, amber Missing and green Complete labels. Inter/Roboto professional type, generous spacing, 44px fields, subtle 6px rounding. No global search, decorative cards, hero, KPIs or extra forms.

Top main bar 'Product setup', a 'Back to products' link at right. Product context 'Handcrafted flower-brown', secondary 'WeevCrafts'. 'Step 4 of 6'.
A compact vertical step progress rail:
Choose product — Complete
Invoice details — Complete
Pickup location — Complete
Packed measurements — Missing, current highlighted
Tax & return policy — Missing
Review — Pending
Use check icons for completed steps, an active circle with 4 for the current step, neutral outlined circles for remaining steps. Do not duplicate step numbers in the text labels.

Only ONE form pane visible, title 'Add packed measurements'. Supporting copy 'Measure the parcel with all outer packaging included.'
Amber inline alert 'Missing: packed weight and dimensions'.
Four comfortably-sized blank fields, never guessed values:
'Packed weight' with fixed unit suffix 'g'
'Length' with 'cm'
'Width' with 'cm'
'Height' with 'cm'.
Weight on its own row, other three in a neat row with substantial width. Place a small refined forest line drawing of a cardboard parcel alongside the form with dimension arrows labelled Length, Width, Height and tiny 'Packed parcel'. The illustration teaches measurement, not a decorative hero. Do not add product photography.
Under fields a concise note 'Use actual measurements. Shipping charges depend on these values.'
Bottom row: outlined 'Back' and one primary forest button 'Save and continue'. Secondary text 'Next: Tax & return policy'.
Tiny bottom corner 'Design concept · Illustrative status'.
Ensure the entire active task and all buttons fit within one desktop viewport. Other steps show status only, not expanded content. High quality legible text and calm usable hierarchy.

### Screen 4

Use case: ui-mockup. Generate ONE high fidelity desktop UI screenshot, landscape 1536x1024, WeevCrafts Super Admin guided setup SCREEN 4: final review. It must be an elegant, legible, implementable product interface with only the current task visible, not a board of multiple screenshots.

Same coordinated design language: dark forest sidebar #123f32, white sans-serif WeevCrafts wordmark / Super Admin, navigation Dashboard, Orders & delivery, Fulfillment highlighted sage, Sellers, Products, Inventory, Returns, Finance, Support. Warm ivory #f7f5ee main canvas, warm white #fffdf8 surface, ink #20362d, muted #56665d, forest buttons, amber Missing/Pending, green Complete, thin warm borders #dfd8ca, Inter and Roboto type, 6px controls, 44px target heights. No charts, hero, extra cards, decorative illustration, or global search.

Main header 'Product setup' and 'Back to products'. Context 'Tree of Life' / 'Kamala', 'Step 6 of 6'.
Compact vertical step rail in the main workspace: Choose product, Invoice details, Pickup location, Packed measurements, Tax & return policy, Review. First five show checkmarks, Review active; do not duplicate numbers beside labels.

Single review pane:
small green check icon beside heading 'Product setup complete'
subtext 'The required product details have been saved.'
A quiet compact summary list with four rows, not a grid:
'Seller invoice details' — 'Verified' — 'Edit'
'Pickup location' — 'Verified' — 'Edit'
'Packed measurements' — 'Saved' — 'Edit'
'Tax & return policy' — 'Verified' — 'Edit'
No fabricated legal names, GST numbers, prices or parcel measurements.

Below the summary, a prominent amber alert with heading 'Checkout is still paused'.
Explanation 'Next, complete the genuine prepaid pilot before opening the selected catalog.'
A small guidance line 'We will guide you through the pilot details next.'
At the bottom one primary forest button 'Continue to pilot setup' and one secondary outlined 'Back to products'.
This screen must clearly distinguish setup completion from live purchase activation and must not show an 'Enable all orders' or 'Book shipment' action.
Tiny footer 'Design concept · Illustrative status'. Lots of deliberate whitespace, concise copy, comfortably readable state labels and practical navigation.

### Invoice refinement

Keep the layout unchanged and remove duplicated numeric prefixes from the progress labels; retain checkmarks and numbered circle indicators.
