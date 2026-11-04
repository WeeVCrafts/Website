# Standalone Super Admin categories

Super Admin Products now links to Add category and Manage categories. Saving a
category publishes its name, photograph, order and customer discovery visibility
immediately. Product assignments and stable URLs remain separate from visibility.
The general Super Admin Storefront editor remains retired.

Publication locks the storefront head, checks the expected release, and commits
category projection, media metadata, immutable release and audit event together.
Only the selected category changes. Existing release images remain available.
Uploads use the configured S3-compatible bucket and existing normalization and
responsive media delivery. No migration or new public JSON API was introduced.

## Verification

- `go test ./...` and `go vet ./...` passed.
- Three isolated PostgreSQL regressions passed: category publication, existing
  category membership/primary protection, and existing draft publication/conflicts.
  Publication checks cover permissions, duplicate slugs, stale edits, preservation
  of unrelated configuration, audit records and rollback of new media metadata.
- Nine Playwright checks passed across Chromium, Firefox and WebKit. They cover
  create/replace/hide, all discovery surfaces, existing links, editor choices,
  CSRF and role boundaries, stale recovery, invalid/oversized files, retained
  selection, keyboard containment/cancellation, axe and responsive layouts.
- Browser checks exercised 320, 390, 768, 1024 and 1440px and CSS 200% zoom reflow.
  This does not represent a physical-device or screen-reader certification.
- Real uploads, replacements and old-image reads passed against a dedicated local
  Garage bucket. Temporarily denying write access to only this QA bucket verified
  actual storage failure and retry without changing the published category.
  A saved image remained readable after application restart.
- `templ generate` finished with zero updates after generation. CSS/JS builds,
  architecture import checks and strict premium audit passed. Design lint returned
  zero errors and its existing orphaned-accent warning.
- Desktop and phone captures were visually inspected. Native file pickers and
  scrollable shared dialogs retain platform behavior. No new sidebar styling or
  customer homepage composition was introduced.

QA writes were confined to category_publication_20261102 at 127.0.0.1:15432 and
weevcrafts-category-publication-qa in the local object store. Production smoke
checks are read-only; authenticated production category creation is not exercised.
Unrelated working-tree deletions, renamed reference images and the untracked
hosting research document were preserved.

Release target: main, Coolify application vy2hpttvmehhauvskvilgoju,
https://weevcrafts.com. Previous application revision:
3e6ddc22591ddea9ddc16e7545fa0767f36619b4.
