# Equipment packages

## Status

Design. Not yet implemented. Supersedes the dormant `packages`/`package_items`
tables from the original `SPEC.md` Phase 4 plan (never built - no handler,
queries, or UI ever referenced them). See "Relationship to original schema"
below for what changes.

## Purpose

Predefined, reusable sets of articles for common scenarios (e.g.
"Spårarhajk", "Valborgsfirande"). A package is a named template a leader can
apply to a booking to fill it in one step, or use as a filter while
browsing. Equipment managers (and other qualifying users) maintain packages
similar to how they maintain articles.

## Data model

### `packages` (reshaped from existing table)

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| group_id | text | FK → groups |
| category_id | uuid | FK → package_categories, nullable |
| name | text | |
| description | text | default `''` |
| color | text | free text - either a design-token palette name (e.g. `trackergreen`) or a custom hex value; unconstrained, see "Color" below |
| visibility | text | `view`, `book`, `trusted`, or `manager` - default `book`, see "Access control" below |
| archived | boolean | default `false` - soft delete |
| created_by | text | FK → users |
| created_at / updated_at | timestamptz | |

Drops `scope` and `owner_id` - no personal packages for now, all packages
are group-wide, gated by the access-level setting below. Can be
reintroduced later without conflicting with this design.

### `package_categories` (new)

Per-group grouping for packages, with its own color. Not the same taxonomy
as article categories (`categories` table) - see "Relationship to original
schema".

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| group_id | text | FK → groups |
| name | text | |
| color | text | same free-text convention as `packages.color` |
| created_at / updated_at | timestamptz | |

A package's `category_id` is nullable - uncategorized packages are
allowed and shown ungrouped in the settings list. Packages can move
between categories at any time via the edit form. No archiving for
categories - they're either present or deleted (see "Packages settings
tab: Categories module" below). Deleting a category sets `category_id` to
`null` on its member packages rather than touching the packages
themselves - the packages survive as uncategorized, only the grouping is
removed.

### `package_items` (reshaped from existing table)

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| group_id | text | FK → groups |
| package_id | uuid | FK → packages, `ON DELETE CASCADE` |
| commercial_name | text | matches `articles.commercial_name` |
| location_id | uuid | FK → locations |
| quantity | int | default 1 - a suggested fill amount, not enforced |

Drops `category_id` and `article_id`. Lines key on `commercial_name +
location_id`, the same grain `AvailabilityGroup` already uses for browsing
and booking (see `AddItemSheet.svelte`'s `groupKey()`). This means:

- A package line doesn't pin specific physical items (e.g. "Sibley 1") - it
  survives individual items being archived, lost, or reassigned.
- Quantity-tracked and individually-tracked commercial names both just get
  a quantity.
- If every article under a `commercial_name + location` is archived, the
  line has nothing left to resolve to - shown greyed out with a warning in
  the package editor and skipped when applying the package to a booking.
  The line itself is not deleted - the commercial name could reappear
  later (e.g. new stock added under the same name). Unlike unavailable
  lines (see "Unavailable lines when filling from a package" below), a
  skipped archived-away line must not be silent: "Fyll från paket" shows a
  post-apply summary naming any lines that couldn't be added for this
  reason, so the distinction stays visible to the user (archived-away =
  not added at all, vs. unavailable-for-these-dates = added but flagged).

### `package_events`

Append-only edit trail, following the existing `article_events` /
`booking_events` pattern - no revert/snapshot capability, just visibility
into who changed what.

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| group_id | text | FK → groups |
| package_id | uuid | FK → packages |
| user_id | text | FK → users, who made the change |
| event_type | text | `created`, `renamed`, `item_added`, `item_removed`, `item_quantity_changed`, `visibility_changed`, `category_changed`, `archived`, `unarchived` |
| detail | jsonb | event-specific payload (e.g. `{commercial_name, location_id, quantity}`) |
| created_at | timestamptz | |

### `group_settings` addition

One new column, following the existing `*_role` pattern:

| Column | Type | Notes |
|---|---|---|
| package_manage_role | text | `book`, `trusted`, or `manager` - default `trusted` |

Minimum access level required to create/edit/archive packages. Group-wide,
not per-package (matches how `booking_role`, `article_edit_role`, etc. work
today). Excludes `view` from the CHECK constraint, consistent with
`booking_role`/`article_edit_role`/`issue_resolve_role` - view-only users
can't act on anything, so it's not a meaningful floor here either.

## Relationship to original schema

The original `SPEC.md` packages design (from Phase 4, never implemented)
differs in three ways this doc changes:

1. **Item grain**: original keyed on `category_id` ("10 from this
   category") or a specific `article_id`. This design uses
   `commercial_name + location_id` instead, consistent with how
   availability and booking already group articles. Article categories
   (`categories` table) remain a separate, still-active taxonomy (browse
   filtering, article forms) - a package line is not a category. The new
   `package_categories` table is a distinct, unrelated grouping - it
   organizes packages themselves (e.g. "Hajker", "Högtider"), not the
   articles inside them. Naming collision is coincidental; no shared code
   or table between the two.
2. **Scope**: original had `scope: org | personal` + `owner_id`. Dropped
   for now - all packages are group-wide. Personal packages can be added
   later as a separate `scope`/`owner_id` addition without touching this
   design.
3. **New fields**: `color`, `visibility`, `category_id`, `archived`, and
   the `package_events` trail are new - none existed in the original
   table.

A migration will `DROP` and recreate `package_items` (it's never had rows,
so no data migration needed) and `ALTER` `packages` to drop `scope`/
`owner_id` and add `color`/`archived`.

## Access control

- **Create/edit/archive a package**: any user whose team access level meets
  or exceeds `group_settings.package_manage_role` (default `trusted`) -
  not restricted to the creator. Matches how team access levels work
  generally (a shared group resource, not personal ownership). This is a
  group-wide setting, unrelated to any individual package's `visibility`.
- **View a package** (browse, apply to booking, see package membership on
  an article, appear in any package picker): gated per-package by
  `packages.visibility` (`view`, `book`, `trusted`, or `manager`,
  default `book`) against the viewing user's team access level - same
  four-level scale used elsewhere (`access_level` on teams), not the
  three-level `package_manage_role` scale. A package set to `manager`
  visibility is invisible to book/trusted-level leaders entirely - it
  doesn't just block editing. Default `book` matches the level that can
  already book articles, so out of the box every booking-capable user
  sees every package; tightening it down is opt-in per package (e.g. a
  manager-only package for something not meant to be self-served).
- Editing a package's `visibility` itself is subject to the same
  `package_manage_role` check as other edits - not a separate permission.
- Archiving/editing follows the standard `RequireRole`-style middleware
  pattern used elsewhere (`equipment_manager` for hard-gated actions,
  a resolved access-level check for this graduated one - similar to how
  `TeamHandler.requireTeamMembership` resolves membership dynamically).
  Viewing follows the same resolved access-level check, applied per
  package row rather than group-wide.

## Color

`@scouterna/design-tokens` (via `ui-webc`) ships a fixed brand palette:
`blue, neutral, gray, orange, green, red, black, white,
challengerpink, discovererblue, trackergreen, roveryellow,
adventurerorange` - the last five are the scout age-group colors. These
become the preset swatches.

`packages.color` is unconstrained text (not a CHECK-limited enum like
`access_level`) - it holds either a palette token name or an arbitrary hex
value, because packages should be color-customizable beyond the fixed
brand set.

New shared component: `ColorSwatchPicker.svelte` (no existing `ui-webc` or
`j26-components` color picker to reuse - confirmed by inspecting both
repos). No new npm dependency - built from scratch as a grid of swatch
buttons for the brand palette (rendered with the real Tailwind token
classes, so they read as genuinely on-brand, selected one gets a
ring/check), plus a final "custom" swatch that reveals a native
`<input type="color">` for anything outside the palette. Used in the
package create/edit form and the category create/edit form; the
resulting value is stored as-is in `packages.color` /
`package_categories.color` and rendered via a small `packageColor()`
helper in `$lib/styles.ts` (same `Record`-based pattern as
`articleStatusColors`, mapping token names to Tailwind classes; falls
through to an inline `style="background:{value}"` when the value isn't a
known token, i.e. a custom hex).

### Text contrast on colored badges

Anywhere a package/category is rendered as a filled badge with its name
as text (settings list, `PackagePicker` rows, the browse popover) needs
readable text against the fill - the palette spans from `white`/
`roveryellow` (light) to `black`/`blue` (dark), so a single fixed text
color doesn't work, and custom hex is unbounded. `packageColor()` returns
a `{ bg, text }` pair instead of just a background class:

- **Known palette tokens** (the fixed 13): `text` is a hardcoded
  `neutral-900`/`neutral-50` (design-system near-black/near-white, not
  pure `#000`/`#fff`) choice per token in the same `Record`, decided once
  against each token's actual hex - no runtime computation needed since
  the set is small and fixed.
- **Custom hex**: computed at render time using the actual WCAG relative
  luminance formula (sRGB-gamma-corrected, not a naive weighted average)
  and the real contrast-ratio calculation against both `neutral-900` and
  `neutral-50` - picking whichever candidate wins, rather than a single
  fixed luminance threshold. A threshold cutoff picks the *lighter* color
  band correctly but can pick the *worse* of two weak options right at
  the boundary; computing both ratios and taking the max avoids that.
  Returned as an inline `style="color:..."` alongside the inline
  `background`. Near-black/near-white (rather than pure black/white) keep
  custom-color badges visually consistent with how the fixed palette
  tokens already render, instead of looking like a stark warning label.

Every badge/swatch call site consumes the `{ bg, text }` pair rather than
re-deriving contrast itself, so this is handled once in `packageColor()`.

**Pick-time warning for weak custom colors**: if a chosen custom hex
produces a weak contrast ratio even against the better of the two
candidates (e.g. under ~3:1 - certain mid-tone hexes land here no matter
which text color is picked), that's a signal the background itself is a
poor badge color, not something text-color choice alone can fix. Rather
than silently shipping a low-contrast badge, `ColorSwatchPicker` shows a
small inline warning next to the native `<input type="color">` when the
picked hex would read poorly, so the person picking the color sees the
problem immediately instead of it surfacing later as an unreadable badge
elsewhere in the app.

### Color and categories: default, not inherited

`packages.color` always stays the source of truth for rendering a
package - never resolved through its category at read time. Category
color is only ever a *seed value* at creation time:

- **Creating a package with an existing category selected**: the
  category's color pre-fills the `ColorSwatchPicker` selection, but it's
  a normal preselection - the user can still pick a different swatch or
  custom hex before saving, and later edits to the category's color never
  cascade back to the package.
- **Creating a new category inline from the package form** (typing a
  category name that doesn't exist yet): the new category is created
  with the color currently selected for the package being created - no
  separate color step for the category.
- **Creating a category directly**, via the Categories module on the
  Packages settings tab (see below): its color is chosen explicitly via
  the same `ColorSwatchPicker`, since there's no package in context to
  seed from.

This keeps color resolution simple everywhere it's rendered (always read
`packages.color` directly, no fallback branching), avoids a cascade-update
problem when a category's color changes later, and keeps per-package
customization intact while still making same-category packages *usually*
match by default.

## UI surfaces

### Settings: new "Packages" tab

`web/src/routes/settings/+page.svelte` is already 2300+ lines split into
`profile` / `teams` / `group` tabs via a local `tab` state variable. A
`packages` tab follows the same pattern but lives in its own file -
`PackagesTab.svelte` - imported and rendered conditionally, to avoid
growing the settings page further. Lists all packages (name, color swatch,
item count, archived state), create/edit/archive actions, and the
`package_manage_role` control (rendered in the existing `group` tab
alongside other role settings, since it's a `group_settings` field like
the others there - not moved into the new tab).

Access to the tab itself: shown to any user meeting
`package_manage_role`, not just managers (unlike the `group` tab, which
stays manager-only). The list is grouped by `package_categories`
(uncategorized packages shown in a final ungrouped section), and each
package row shows its `visibility` level alongside name, color swatch,
item count, and archived state. The create/edit form includes: name,
description, color (`ColorSwatchPicker`, seeded from the selected
category per "Color and categories" above), category (existing dropdown
or type-to-create-new), and visibility (four-level select, default
`book`).

### Packages settings tab: Categories module

The Packages settings tab opens with a **Categories** module above the
package list itself - a compact list of existing categories (name +
color swatch) with create/rename/recolor/delete actions, gated by the
same `package_manage_role` check as everything else on the tab. This is
the *only* place a category can be deleted; deletion needs its own
explicit affordance because it's destructive to the grouping (member
packages become uncategorized) even though it doesn't touch the packages
themselves, so it's kept out of the inline creation flow entirely.

Category creation itself is available in two places: this module
(explicit color choice, per "Color and categories" above), and inline
from the package create/edit form by typing a new category name
(color seeded from the package being edited, no separate step). Both
paths write to the same `package_categories` table; there's no
distinction between a category created one way vs. the other afterward.

Archived packages get a **delete** action in addition to unarchive -
archiving is the default (reversible) removal path, but a fully archived
package can be permanently deleted from the same row (with a
confirmation, since it's the one irreversible action in this surface).
This uses the `ON DELETE CASCADE` on `package_items` already in the
schema; deleting logs a final `package_events` row before the cascade
removes the package's own event rows too, or an audit event is instead
recorded against a `null` package reference - to be settled during
implementation depending on whether `package_events` should survive its
own package's deletion.

### Article/item page

- Shows which packages the article's `commercial_name` belongs to (a
  simple list query joining `package_items` on `commercial_name +
  location_id`, scoped to the article's own location and filtered to
  packages visible at the current user's access level).
- "Add to package" action opens the shared `PackagePicker` (see "Shared
  package picker" below) in multi-select mode - a leader/manager can add
  the same article to several packages in one step, each getting its own
  quantity prompt, plus the option to create a new package inline.

### Browse view: compact package indicator

The article expand view (where images are shown today) gets a compact
package-membership row alongside the image: a small cluster of up to
~3 color swatches (first few packages the article belongs to) plus a
"+N" chip if there are more, rather than a horizontally-scrollable strip.
Reasoning: horizontal scroll on a small compact row is an easy-to-miss,
hard-to-discover gesture on mobile, and competes with the page's own
scroll; a swatch cluster + "+N" is a well-known compact pattern and gives
a single obvious tap target instead of relying on hover (which doesn't
exist on touch). Tapping the cluster/chip opens a small popover/sheet
listing all packages by name and color - reusing the same list styling as
`PackagePicker` rather than inventing a fourth package-list treatment.

### Booking page

- **"Fyll från paket" button** beside the existing "Add articles"
  (`AddItemSheet`) action. Opens the shared `PackagePicker` (single-select
  mode); applying a package adds its lines to the cart at the suggested
  quantities (additive - doesn't
  clear existing cart contents; if a line's `commercial_name` already
  exists in the cart, quantities add together, still freely editable
  afterward - no validation against the package quantity).
- **"Spara som paket" action**: available to the same users who can edit
  the booking, gated additionally by `package_manage_role` (i.e. a
  book-level leader can only save a package if `package_manage_role` is
  `book` or looser). Copies current booking lines 1:1 into a new package
  as a starting point, prompts for name/color before saving.
- **"Jämför med paket"**: pick a package via the shared `PackagePicker`
  (single-select), show a diff of the booking's current lines against it:
  - lines in the package but missing from the booking
  - lines in the booking but not in the package
  - lines in both with a quantity mismatch (booking qty vs package qty)
  No side-effect - purely informational, with quick "add missing" actions
  per diff row.

**Unavailable lines when filling from a package**: "Fyll från paket" adds
every line regardless of availability - same behavior as adding an item
manually via `AddItemSheet` today. If a line's `commercial_name +
location` has no stock left for the booking's date range, or its
`approval_level` requires manager sign-off, it's still added to the cart
and shows the same unavailable/needs-approval indicator a manual add would
show; nothing is silently skipped. This keeps "Jämför med paket" diff
semantics simple - a line missing from the booking always means "not in
the cart," never "in the cart but unavailable."

### Browse / shop

- The shared `PackagePicker` (single-select) filters the catalog to just
  the `commercial_name`s present in the selected package (a scoping
  filter, not an additive selection - narrows what's shown, coexists with
  the existing category filter and search). Only packages visible at the
  current user's access level are offered.

### Shared package picker

One new component, `PackagePicker.svelte`, reused across every "pick a
package" surface instead of building one per surface: add-to-package
(multi-select) on the article page, Fyll från paket (single-select),
Jämför med paket (single-select), and the browse scoping filter
(single-select). Visually a dropdown/sheet listing packages vertically
(grouped by category like the settings tab, color swatch + name per row),
with a `multiple` prop switching row selection between single-tap-closes
and toggleable-checkbox behavior. Always filtered server-side to packages
visible at the caller's access level - a book-level leader never sees a
manager-only package in any of these pickers, not just in the settings
tab.

On mobile this renders as a bottom sheet (consistent with `AddItemSheet`
and other existing pickers) rather than an inline dropdown, given the
project's mobile-first convention and that leaders primarily interact
with booking on phones. The `ColorSwatchPicker` grid and the native
`<input type="color">` custom swatch are likewise verified to work as
touch targets at 375px width - the grid wraps rather than scrolling
horizontally.

### i18n

All new user-facing strings introduced by this feature - button labels
("Fyll från paket", "Spara som paket", "Jämför med paket", "Lägg till i
paket"), the package/category forms, visibility level labels, and the
compact package-indicator popover - go through Paraglide message keys
(`package_` namespace) per the project's i18n conventions, not hardcoded
Swedish text. The Swedish phrases used throughout this doc are working
labels for the design, not final hardcoded strings.

### Package activity trail

`package_events` (see "Data model" above) is surfaced in the package edit
form in the settings tab as a collapsible activity log, mirroring
wherever `article_events`/`booking_events` are already shown in the
article/booking edit views (same list treatment: user, event type,
timestamp, detail). Not surfaced anywhere else (e.g. not shown in
`PackagePicker` or the browse popover) - it's an editing/management-time
detail, not a browsing one.

### Shared editing UX

`AddItemSheet.svelte` is booking-shaped: it takes `bookingId` +
`startDate`/`endDate` and calls `checkAvailability()`, which is
date-range-dependent and doesn't apply to packages (no dates). The
reusable part is the search/category-filter/result-list UI built around
`AvailabilityGroup`-shaped rows (`groupKey()` already keys on
`commercial_name + location_name`, matching `package_items`'s grain).

Plan: extract that list/search/filter UI into a component parameterized
over a plain `{commercial_name, location_id, location_name, quantity}[]`
source, with two thin callers:
- `AddItemSheet` (existing) - source is `checkAvailability()` results,
  keeps availability/approval-level display and date dependency.
- New `PackageItemsEditor` - source is all active `commercial_name +
  location` combinations (no availability check, no date dependency),
  used both in the package edit form and reused for the "compare to
  booking" diff rendering.

## Implementation plan

Sequential phases, each its own commit per the project's workflow
convention. Each phase is independently shippable/reviewable; later
phases depend on earlier ones but nothing here requires big-bang
delivery.

### Phase 1: Schema

- Goose migration: `ALTER packages` to drop `scope`/`owner_id`, add
  `color`, `visibility` (with CHECK `view|book|trusted|manager`,
  default `book`), `category_id` (nullable FK), `archived`
  (default `false`).
- `DROP`/recreate `package_items` with the new `commercial_name +
  location_id + quantity` shape (no data migration needed - table has
  never had rows).
- Create `package_categories` (`id`, `group_id`, `name`, `color`,
  timestamps).
- Create `package_events` (append-only, per "Data model" above).
- `ALTER group_settings` to add `package_manage_role` (CHECK
  `book|trusted|manager`, default `trusted`).
- sqlc queries for all of the above (CRUD on packages/categories/items,
  event insert, event list); `sqlc generate`.

### Phase 2: Backend - packages & categories CRUD

- Handler + routes for package categories: create, rename, recolor,
  delete (nulling `category_id` on member packages), list.
- Handler + routes for packages: create, edit (name/description/color/
  visibility/category), archive/unarchive, delete (archived-only),
  list (with visibility filtering against the caller's resolved access
  level), get-one.
- Access-level middleware: `package_manage_role` check for
  create/edit/archive/delete/category actions; per-row visibility
  filtering for reads.
- `package_events` logging on every mutating action.
- Integration tests: happy path for each action, access-control
  rejections (below `package_manage_role`, wrong `group_id`), visibility
  filtering (a book-level user can't see a manager-only package via any
  endpoint).

### Phase 3: Backend - package items

- Handler + routes for package line management: add/remove/update-
  quantity on a package's `commercial_name + location_id` lines.
- Query to resolve a package's lines against current article state, for
  the "greyed out with a warning" archived-away case.
- Query joining `package_items` on `commercial_name + location_id` for
  the article-page "which packages is this in" lookup.
- `package_events` logging (`item_added`, `item_removed`,
  `item_quantity_changed`).
- Integration tests.

### Phase 4: Frontend - shared components

- `ColorSwatchPicker.svelte`: palette grid + custom `<input
  type="color">` swatch, selected-state ring/check.
- `packageColor()` helper in `$lib/styles.ts`: `{ bg, text }` resolution
  per "Text contrast on colored badges" above (hardcoded per-token pairs
  for the fixed palette, WCAG contrast-ratio computation for custom
  hex).
- Pick-time weak-contrast warning in `ColorSwatchPicker`'s custom-color
  path.
- `PackagePicker.svelte`: category-grouped vertical list, `multiple`
  prop, bottom-sheet on mobile, always calling the visibility-filtered
  list endpoint.
- Extract `AddItemSheet`'s search/filter/list UI into a shared component
  parameterized over a plain result array; re-point `AddItemSheet` at it
  with no behavior change (a refactor-only sub-step, verified against
  existing `AddItemSheet` tests/smoke coverage before building on top of
  it).
- `PackageItemsEditor.svelte` built on that extracted component (no
  availability/date dependency).

### Phase 5: Frontend - Packages settings tab

- `PackagesTab.svelte`: Categories module (list + create/rename/
  recolor/delete) above the package list.
- Package list: category-grouped, swatch, item count, visibility,
  archived state; create/edit form (name, description, color seeded
  from category, category picker/type-to-create, visibility select);
  archive/unarchive; delete-when-archived with confirmation.
- `PackageItemsEditor` embedded in the package edit form for line
  management.
- Collapsible `package_events` activity log in the edit form.
- `package_manage_role` control added to the existing `group` tab.
- Wire tab visibility to `package_manage_role` (shown to any qualifying
  user, not just managers).
- i18n keys (`package_` namespace) for every new string; `pnpm run
  build` to compile Paraglide.
- Smoke test entry for the new tab state.

### Phase 6: Frontend - article/item page integration

- Package-membership list on the article page (packages containing this
  `commercial_name + location_id`, visibility-filtered).
- "Add to package" action opening `PackagePicker` in multi-select mode,
  with a per-selected-package quantity prompt and inline
  create-new-package option.
- Compact package-indicator (swatch cluster + "+N") in the browse
  expand view, tap-to-open popover reusing `PackagePicker` list styling.

### Phase 7: Frontend - booking page integration

- "Fyll från paket" button + `PackagePicker` (single-select); additive
  cart merge with quantity summing; post-apply summary for any lines
  skipped due to fully-archived commercial names; unavailable/needs-
  approval lines added with existing indicators, never silently
  skipped.
- "Spara som paket" action, gated by booking-edit rights +
  `package_manage_role`; copies current lines 1:1, prompts for name/
  category/color before saving.
- "Jämför med paket": `PackagePicker` (single-select) + diff view
  (missing/extra/quantity-mismatch rows) with per-row "add missing"
  actions, no side effects otherwise.

### Phase 8: Frontend - browse/shop integration

- `PackagePicker` (single-select) as a scoping filter alongside the
  existing category filter and search.

### Phase 9: Documentation & cleanup

- `docs/API.md`: new package/category endpoints.
- `.amazonq/rules/project-context.md`: packages as a first-class
  concept, data model summary.
- `docs/implementation/accomplished.md`: log completion; remove any
  corresponding item from `BACKLOG.md`.
- `docs/guide.md`: user-facing description of packages if warranted.
- Full pre-done checklist per `workflow.md` (tests, svelte-check,
  duplication/dead-code pass) before considering the feature finished.

## Open items / questions

None outstanding - all resolved in discussion (see git history of this
file for earlier revisions if this section stays empty for a while, it
means no revisit is pending).
