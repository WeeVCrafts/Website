# Responsive role design studies

The browser audit used the source-built Go application, existing role-workflow
fixtures and the project's own CSS, fonts, images and templates. No generated
inventory or replacement visual identity was introduced.

Before implementing changes, `scripts/preview-role-ux-designs.cjs` generated
responsive HTML screen studies and 1440/390/320px screenshots in
`tmp/authenticated-ui-audit/designs`. These are private browser DOM studies;
they do not save any application state. The studies cover customer account,
catalogue, seller, Admin, Super Admin and category presentation.

| Screen | Observed problem | Chosen design |
| --- | --- | --- |
| Customer catalogue | Obsolete banner columns narrow phone text and delay product discovery | Block banner, full-width copy, shallow photo; retain collection rail and filters |
| Customer account | Main order action is below statistics and the collapsed phone account menu | Shared next-action card linking to the actual latest order; address/shopping states for an empty account |
| Customer payments | Captured status text measures 4.11:1 | Reuse canonical status component and existing success colors |
| Shared customer footer | 20px desktop links fail the target-size audit | At least 32px link height at every width |
| Seller dashboard | Stock alert is a small inline link | Existing low-stock information in the shared guidance card with a 44px queue action |
| Seller navigation | Search promises multiple workspaces even on a screen that cannot show results | Search label and destination match supported queues; catalogue fallback elsewhere |
| Seller keyboard | Global Escape moves focus to the navigation trigger even when no drawer is open | Drawer-only Escape handling; shared dialog retains focus ownership |
| Super Admin | Identity card detours through security activity to reach account/role tasks | Separate direct actions for Users, Roles and Security |
| Admin/Super Admin categories | Long action overflows at 320px | Short task label and shared wrapping action |

The seller study initially repeated generic guidance above the page title.
The implementation deliberately uses the existing stock-alert position and
real stock data instead, avoiding an extra duplicate dashboard instruction.
The rest of the screens keep their current accepted composition. Screens that
passed the audit were not redesigned solely for visual novelty.

The generated comparison gallery links the original screenshots, design studies
and implementation screenshots. Full screenshots and the machine-readable route
reports remain in `tmp/authenticated-ui-audit` for local inspection.

## Contract drift review

| Maintained rule | Live evidence | Outcome |
| --- | --- | --- |
| Reuse TaskGuidance | Earlier account/seller guidance existed in preview templates | Extended the live `LiveAccount` and `PortalDashboard` owners |
| Use shared semantic status colors | Payment activity had a separate low-contrast green badge | Uses `ui-status` and the captured-payment alias in `StatusTone` |
| Match searchable scope and URL state | Seller header submitted to non-searchable workspace routes | Supported queue search or explicit product-search destination |
| Shared dialog owns Escape/focus | Seller window Escape handler also ran outside navigation | Scoped to an open drawer, preserving confirmation focus |
| Preserve accepted typography and palette | Existing shared brand and operational fonts remain authoritative | No new brand tokens or font dependencies |
