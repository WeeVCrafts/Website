# WeeVCrafts agent guidance

## Project context

Read README.md and the relevant docs before changing behavior. WeeVCrafts is a Go marketplace with server-rendered templ pages, JavaScript, Tailwind CSS, PostgreSQL, and Redis. Preserve this stack and existing shared UI conventions. Inspect the working tree first and preserve unrelated in-progress changes.

## Required skill selection

The user requested a global skill library for this project and future projects. Whenever a requirement benefits from an installed skill, select and read it before planning or implementation. Use `C:/Users/Vinay/.codex/skills/requested-skill-library/SKILL.md` to find the appropriate entry. The inventory and dependency status are in `C:/Users/Vinay/.codex/skill-library/README.md` and `inventory.json`.

- New UI surfaces or substantial requested redesigns: frontend-app-builder.
- Product/admin UX: frontend-design-premium plus its prerequisite frontend-design.
- Website structure: website-ui-design; general UI review/polish: impeccable.
- Design-system research: ui-ux-pro-max; animation: Emil's skills or transitions-dev.
- Accessibility/layout/typography review: the focused better-* skills.
- UX research, interaction specifications, and handoff: the designer-* skills.
- Marketing/customer research or video creation: the relevant installed specialist skill only when requested.

Use one primary workflow and only the supporting references needed for the task. Preserve accepted designs and existing shared tokens/components. A generic skill default does not authorize introducing React/Vite into this Go application. Check actual MCP/connector availability before following tool-dependent instructions. Use optional agent definitions only when delegation is authorized.

## Verification

Run checks appropriate to the changed surface. For UI work, inspect the rendered browser result, keyboard operation, and a narrow viewport; use the existing Playwright suite when relevant. For CSS changes, use the existing package scripts. Report what actually ran and any unresolved dependencies.
