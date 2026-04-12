# Access Levels

Design doc for the configurable access level system. Replaces the hardcoded role→permission mapping from `role-mapping.json` with per-unit access levels stored in the database, configurable by equipment managers.

## Overview

Every user's OIDC token contains claims like `troop:17443:vice_leader` and `group:766:it_manager`. Today, a static `role-mapping.json` hardcodes which claims map to which app roles (leader, project_leader, equipment_manager). This is inflexible — adding a group or changing trust levels requires editing a JSON file and redeploying.

This change also removes the `project_leader` role and `project` unit type. The old model distinguished "units" (troops) from "projects" (committees/roles) and gave project leaders special approval bypass. The new model replaces all of this with per-unit access levels — a unit's trust level is configured directly, regardless of whether it's a troop or a committee.

The new model:
1. Each OIDC claim maps to a **unit** in the database (auto-created on first login if unknown)
2. Each unit has a configurable **access level** (view, book, trusted, manager)
3. The **booking's "used by" unit** determines the access level for that booking — not the user's global max
4. Equipment managers configure access levels per unit in the UI
5. Group defaults control what new/unknown units start at
6. `role-mapping.json` is eliminated — replaced by `unit_claim_mappings` table + auto-discovery

## Access levels

| Level | Key | Swedish | Booking behavior |
|---|---|---|---|
| 0 | `view` | Visa | Cannot create bookings. Browse-only. Can report issues. |
| 1 | `book` | Boka | Can book. `low` and `high` articles always need approval. |
| 2 | `trusted` | Betrodd | Can book. `low` auto-approves. `high` always needs approval. |
| 3 | `manager` | Utrustningsansvarig | Can book + approve others. `low` and `high` always need approval (self-included). Full admin access. |

Key decisions:
- **`high` always needs approval**, even for managers. Managers can approve their own bookings, but the booking goes through the approval flow — there's always a paper trail.
- **`force_approval` stays** — any user can request manager review on a booking that would otherwise auto-confirm. Useful when someone is unsure and wants a more experienced person to check their booking.
- **Personal bookings** (no "used by" unit) always use `book` level — needs approval for both `low` and `high`.
- **Managers can approve their own bookings** — the approval queue shows all submitted bookings including their own.

### Approval matrix

| Article approval_level | `view` | `book` | `trusted` | `manager` |
|---|---|---|---|---|
| `none` | ❌ can't book | ✅ auto-confirm | ✅ auto-confirm | ✅ auto-confirm |
| `low` | ❌ can't book | ⏳ needs approval | ✅ auto-confirm | ✅ auto-confirm |
| `high` | ❌ can't book | ⏳ needs approval | ⏳ needs approval | ⏳ needs approval |

The booking's effective access level is determined by the "used by" unit. If the booking has no unit (personal), it's `book`.

## OIDC claim mapping

### Token structure

A Keycloak token contains roles like:
```
group:766:it_manager
group:766:walpurgis_committee
troop:17443:vice_leader
troop:9109:leader
```

The claim has three parts: `scope:id:role_name`.

### Claim→unit mapping table

```sql
unit_claim_mappings (
  id          uuid PK DEFAULT gen_random_uuid(),
  group_id    text NOT NULL FK → groups,
  unit_id     uuid NOT NULL FK → units,
  claim_scope text NOT NULL,  -- 'group' or 'troop'
  claim_id    text NOT NULL,  -- e.g. '766', '17443'
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (group_id, claim_scope, claim_id)
)
```

The `role_name` part of the claim (e.g. `vice_leader`, `leader`) is ignored for mapping purposes — any role within a troop or group grants membership in the corresponding unit. This matches reality: a `vice_leader` and a `leader` in the same troop have the same access.

One claim maps to one unit. Multiple claims can map to the same unit (e.g. if two troop IDs should be treated as one unit).

### Auto-discovery on login

When a user logs in and the auth middleware processes their token:

1. Extract all `group:*` and `troop:*` claims
2. Determine the user's group from `group:GROUP_ID:*` claims
3. Look up `unit_claim_mappings` for the group
4. For each claim:
   - **Matched**: user belongs to that unit, gets its `access_level`
   - **Unmatched `troop:*`**: auto-create unit (name: "Avdelning {id}", type: `unit`) with `default_access_troop`, create mapping
   - **Unmatched `group:*`**: auto-create unit (name: "Roll {role_name}", type: `role`) with `default_access_role`, create mapping
5. The manager sees auto-created units in settings and renames/adjusts them

Auto-created unit names are temporary placeholders. The manager renames them to proper names (e.g. "Avdelning 17443" → "Yggdrasil").

**Future**: The OIDC provider (ScoutID/Keycloak) will include troop and role display names as custom claims in the token. When available, auto-discovery will use these names directly instead of generating placeholders — no manual renaming needed. This is tracked as upstream work on the Keycloak configuration.

### Group identification and multi-group members

A user's token can contain claims for multiple scout groups (e.g. a leader in both Mälarscouterna and another group). The system extracts **all** group IDs from `group:*` claims and checks which ones exist in the `groups` table.

Resolution:
1. Extract all unique group IDs from `group:*` and `troop:*` claims (troop→group via `unit_claim_mappings`)
2. Check which of those groups exist in the database
3. If **zero** groups match → "group not found" error
4. If **one** group matches → use it (most common case)
5. If **multiple** groups match → check for a stored preference (`users.active_group_id`). If no preference, prompt the user to choose on first login. The choice is stored and used for subsequent logins.

The user can switch groups from the profile page. Switching changes `active_group_id` and reloads the session — all data is scoped to the new group.

`users.active_group_id` is a new nullable column. When set, the auth middleware uses it to scope the session. When null (first login with multiple groups), the frontend shows a group picker.

For users with only `troop:*` claims and no `group:*` claims: the troop must already have a mapping in `unit_claim_mappings` (which includes `group_id`). If no mapping exists and there's no `group:*` claim, the user gets a "group not found" error.

## Active group display

The landing page shows which group the user is logged in as:

```
[Logo]
ms-utrustning
Utrustningsbokning för {group_name}
```

The group name replaces the hardcoded "Mälarscouterna" text. It comes from `data.user.group_name` which is already in the User type.

For multi-group users, the landing page shows a group switcher directly — a list of available groups with the active one highlighted. Clicking another group switches `active_group_id` and reloads. No need to navigate to settings first.

The desktop nav shows the group name next to the user's name: `{user.name} · {group_name}`.

### Unit types

The `units.type` column changes from `unit`/`project` to `troop`/`role`:

- `troop` — a scout troop (from `troop:*` OIDC claims). Swedish: "Avdelning"
- `role` — a functional role or committee (from `group:*` OIDC claims). Swedish: "Roll"

The type is purely descriptive — it controls the label shown in the UI and which group default applies on auto-creation. It has no effect on permissions (that's `access_level`).

The old `project` type and `project_leader` role are removed entirely. Committees like Valborgskommittén are just units of type `role` with whatever access level the manager configures.

### Booking unit picker

When creating a booking, the unit picker shows all units the user belongs to. Each option shows the unit name and its access level as a hint, so the user understands the approval implications:

```
Yggdrasil (Boka)
Valborgskommittén (Betrodd)
IT-gruppen (Ansvarig)
───
Personlig bokning
```

The access level label is shown in parentheses, muted. This replaces the old `(projekt)` suffix. The user sees at a glance: "if I book for Yggdrasil, I'll need approval for more things than if I book for Valborgskommittén."

## Access resolution per booking

The critical rule: **the booking's "used by" unit determines the access level**.

```
When submitting a booking:
  if booking.used_by_unit_id is set:
    access_level = units.access_level WHERE id = booking.used_by_unit_id
  else (personal booking):
    access_level = 'book'  -- always book level for personal bookings

  Then apply the approval matrix above.
```

Examples with Mälarscouterna:
- **Hanna** (Yggdrasil, `book`) books for Yggdrasil → `low` items need approval
- **Julia** (Yggdrasil `book` + Valborgskommittén `trusted`) books for Valborgskommittén → `low` items auto-confirm
- **Julia** books for Yggdrasil → `low` items need approval (Yggdrasil is `book`)
- **Teo** (Yggdrasil `book` + IT-gruppen `manager`) books for IT-gruppen → `low` items need approval (manager level, but `low` still needs approval for managers)
- **Teo** books for Yggdrasil → `low` items need approval
- **Teo** makes a personal booking → `low` items need approval (`book` level)
- **Gillis** (Utrustningsgruppen `manager`) books for Utrustningsgruppen → `low` items need approval, but Gillis can approve his own booking

### User-level capabilities

While access is per-booking, some UI decisions need a user-level check:

```
user_max_access = max(access_level of all user's units)
```

| Capability | Required level |
|---|---|
| Browse articles | any (including `view`) |
| Report issues | any (including `view`) |
| Create bookings | `book` or higher |
| See "Boka" button | `book` or higher |
| Manage inventory | `manager` |
| See approval queue | `manager` |
| Approve/reject bookings | `manager` |
| Manage group settings | `manager` |

A user with at least one `manager`-level unit gets full admin access to the group's inventory and settings. The manager role is not scoped to a unit — it's a group-wide admin capability.

## Group defaults

Three new columns on `group_settings`:

| Column | Type | Default | Controls |
|---|---|---|---|
| `default_access_unknown` | text | `view` | Users with no recognized claims |
| `default_access_troop` | text | `book` | Auto-created units from `troop:*` claims |
| `default_access_role` | text | `book` | Auto-created units from `group:*` claims |

The distinction between troop and role defaults exists because the pattern "committees are more trusted than random troops" is common. A group can set `default_access_role = trusted` while keeping `default_access_troop = book`.

`default_access_unknown` applies to authenticated users whose token has a `group:*` claim (so we know their group) but no claims that map to any unit. These users can browse but not book by default.

## Group bootstrap and system admin

### The init-group CLI command

A subcommand built into the Go API binary. Run via `docker compose exec`:

```bash
docker compose exec api /app/server init-group \
  --group-id 766 \
  --group-name "Mälarscouterna" \
  --manager-claim "group:766:it_manager" \
  --unit-name "IT-gruppen"
```

This command:
1. Creates the `groups` row (or skips if exists)
2. Creates the `group_settings` row with defaults
3. Creates a unit with `access_level = 'manager'` and the given name
4. Creates a `unit_claim_mappings` row linking the claim to the unit
5. Creates a default "Övrigt" category and seed locations (optional, via `--seed-locations` flag)

The command is idempotent — running it twice doesn't create duplicates.

**Renaming a group**: The group name is a display name set during `init-group` (or editable by managers in settings). The group ID (Keycloak org ID) never changes. Managers can rename their group from the settings page — this updates `groups.name` which flows through to the landing page, nav bar, and anywhere the group name is displayed.

For dev/demo, the seed script calls this instead of manually inserting groups and units.

### Adding units in advance

Managers can pre-create units and their claim mappings through the settings UI before anyone with those claims logs in. This is useful when the manager knows the troop IDs from Scoutnet:

1. Go to settings → "Avdelningar och roller"
2. Click "Lägg till"
3. Enter: name ("Yggdrasil"), type (troop/role), claim scope (troop/group), claim ID (17443), access level (book)
4. Save → creates unit + mapping

When a user with `troop:17443:*` logs in, the mapping already exists — they're immediately placed in Yggdrasil with the configured access level.

## Changes to existing code

### Claims struct

```go
// Before
type Claims struct {
    MemberID string   `json:"member_id"`
    GroupID  string   `json:"group_id"`
    Name     string   `json:"name"`
    Email    string   `json:"email"`
    Roles    []string `json:"roles"`
    Units    []string `json:"units"`
}

// After
type UnitMembership struct {
    UnitID      string `json:"unit_id"`
    UnitName    string `json:"unit_name"`
    UnitType    string `json:"unit_type"`    // "troop" or "role"
    AccessLevel string `json:"access_level"` // "view", "book", "trusted", "manager"
}

type Claims struct {
    MemberID    string            `json:"member_id"`
    GroupID     string            `json:"group_id"`
    Name        string            `json:"name"`
    Email       string            `json:"email"`
    Units       []UnitMembership  `json:"units"`
    MaxAccess   string            `json:"max_access"` // highest access across all units
}
```

`Roles []string` is removed. All permission checks use `MaxAccess` (for user-level capabilities) or the booking's unit access level (for approval decisions).

Helper methods:
- `claims.IsManager()` → `claims.MaxAccess == "manager"`
- `claims.CanBook()` → `claims.MaxAccess >= "book"` (using level ordering)
- `claims.AccessForUnit(unitID)` → returns the access level for a specific unit
- `claims.UnitNames()` → returns unit names (for booking visibility queries)

### Auth middleware changes

The middleware currently does two things:
1. Validates the JWT (or dev persona)
2. Extracts claims

The new flow adds a DB lookup between token parsing and claims construction:

1. Validate JWT / dev persona (unchanged)
2. Extract raw token claims (member_id, group_id, raw OIDC roles)
3. **New**: Query `unit_claim_mappings` + `units` for the user's group
4. **New**: Match token claims to units, auto-create unknown ones
5. Construct `Claims` with `[]UnitMembership` and `MaxAccess`
6. Upsert user record (unchanged)

This adds one DB query per request (cacheable per group — claim mappings rarely change). The auto-creation of unknown units happens inline during login.

### Booking submit handler

```go
// Before
switch maxLevel {
case "high":
    if !claims.HasRole("equipment_manager") {
        needsApproval = true
    }
case "low":
    if unitAccess != "trusted" && unitAccess != "manager" {
        needsApproval = true
    }
}

// After
var unitAccess string
if booking.UsedByUnitID != nil {
    unitAccess = claims.AccessForUnit(*booking.UsedByUnitID)
} else {
    unitAccess = "book" // personal bookings always book level
}

switch maxLevel {
case "high":
    needsApproval = true // always needs approval
case "low":
    if unitAccess != "trusted" && unitAccess != "manager" {
        needsApproval = true
    }
}
```

`force_approval` stays on the submit request — any user can opt into manager review regardless of access level and article approval level. This covers the case where someone is booking for the first time or is unsure about quantities/dates and wants a sanity check.

### Route guards

Replace `RequireRole("equipment_manager")` middleware with `RequireAccess("manager")`:

```go
func RequireAccess(level string) func(http.Handler) http.Handler
```

For booking creation, use `RequireAccess("book")`.

### Frontend User type

```typescript
// Before
export interface User {
    member_id: string;
    group_id: string;
    group_name: string;
    name: string;
    email: string;
    roles: string[];
    units: string[];
    role_units?: Record<string, string[]>;
}

// After
export interface UnitMembership {
    unit_id: string;
    unit_name: string;
    unit_type: 'troop' | 'role';
    access_level: 'view' | 'book' | 'trusted' | 'manager';
}

export interface User {
    member_id: string;
    group_id: string;
    group_name: string;
    name: string;
    email: string;
    units: UnitMembership[];
    max_access: 'view' | 'book' | 'trusted' | 'manager';
}

export function isManager(user: User | null): boolean {
    return user?.max_access === 'manager';
}

export function canBook(user: User | null): boolean {
    return accessAtLeast(user?.max_access, 'book');
}
```

### Dev personas

Personas no longer carry `roles` — they carry unit names. The auth middleware resolves unit names to full `UnitMembership` structs (with UUIDs and access levels) by querying the DB at request time. This keeps the persona file human-readable and avoids hardcoding UUIDs that change on every reseed.

```json
{
  "personas": {
    "manager-equipment": {
      "member_id": "3000002",
      "name": "Gillis Utrustning",
      "email": "gillis@example.com",
      "groups": {
        "766": ["Utrustningsgruppen"]
      }
    },
    "trusted-and-troop": {
      "member_id": "3000003",
      "name": "Julia Valborg-Yggdrasil",
      "email": "julia@example.com",
      "groups": {
        "766": ["Valborgskommittén", "Yggdrasil"]
      }
    },
    "leader-yggdrasil": {
      "member_id": "3000005",
      "name": "Hanna Yggdrasil",
      "email": "hanna@example.com",
      "groups": {
        "766": ["Yggdrasil"]
      }
    },
    "leader-flaskpost": {
      "member_id": "3000006",
      "name": "Fredrik Flaskpost",
      "email": "fredrik@example.com",
      "groups": {
        "766": ["Flaskpostorné"]
      }
    },
    "leader-unit-it": {
      "member_id": "3000004",
      "name": "Teo IT-Yggdrasil",
      "email": "teo@example.com",
      "groups": {
        "766": ["IT-gruppen", "Yggdrasil"]
      }
    },
    "view-only": {
      "member_id": "3000008",
      "name": "Vera Visa",
      "email": "vera@example.com",
      "groups": {
        "766": []
      }
    },
    "multi-group": {
      "member_id": "4000001",
      "name": "Linn Två-Kårer",
      "email": "linn@example.com",
      "groups": {
        "766": ["Yggdrasil"],
        "999": ["Avdelning 1"]
      }
    }
  }
}
```

The middleware flow for dev personas:
1. Load persona by name from the file (static: member_id, groups, name, email, unit names per group)
2. Determine active group: check `active_group_id` on the user record, or use the first group
3. Query `units` table for the active group to resolve each unit name → UUID + access_level
4. Construct full `Claims` with `[]UnitMembership` and computed `MaxAccess`
5. For the `view-only` persona (empty units), `MaxAccess` = group's `default_access_unknown`

This adds one DB query per dev request but keeps personas maintainable. The same resolution logic is used for both dev personas and real OIDC tokens — only the source of unit names differs (persona file vs claim mappings).

Multi-group personas (like `multi-group`) list multiple group IDs. The middleware uses the stored `active_group_id` preference (or defaults to the first group). The group switcher on the landing page lets the user switch between them.

A new `view-only` persona for testing the browse-only experience. A new `multi-group` persona for testing the group switcher.

## Removed concepts

The following are eliminated by this change:

- **`project_leader` role** — replaced by `trusted` access level on specific units. Valborgskommittén being trusted is a property of that unit, not a role the user carries globally.
- **`project` unit type** — replaced by `role` type. The word "project" implied temporary cross-unit activities with special approval bypass. Now it's just a unit type label with no permission implications.
- **`leader` role** — replaced by `book` access level. Being a leader in a troop gives you membership in that unit; the unit's access level determines what you can do.
- **`equipment_manager` role** — replaced by `manager` access level. Any unit can be set to manager level.
- **`role-mapping.json`** — replaced by `unit_claim_mappings` table + `init-group` CLI.
- **`ProjectRoles` / `AdminRoles` / `Troops` in role mapping** — all replaced by generic claim→unit mappings with configurable access levels.

Code locations that reference these (to update during implementation):
- `api/internal/auth/claims.go` — `ParseClaims`, `RoleMapping`, `GroupMapping` structs
- `api/internal/auth/auth.go` — `Claims.HasRole()`, `RequireRole()`
- `api/internal/handler/bookings.go` — approval logic in Submit, AddItems
- `api/internal/handler/group_settings.go` — `image_upload_role` validation
- `api/internal/images/handler.go` — upload permission check
- `api/internal/handler/units.go` — type validation (`unit`/`project` → `troop`/`role`)
- `api/internal/handler/tests/` — all test files referencing roles
- `web/src/lib/user.ts` — `User` type, `hasRole()`
- `web/src/lib/components/DevPersonaSwitcher.svelte` — role labels
- `web/src/routes/+layout.server.ts` — `parseUserFromSession`, role mapping loading
- `web/src/routes/profile/+page.svelte` — role cards
- `web/src/routes/book/+page.svelte` — unit picker `(projekt)` suffix
- `dev-personas.json` — `roles` field
- `role-mapping.json` — entire file deleted

## Database changes

Since no production deployment exists yet, this is implemented as a **full schema recreation** — all existing migrations are consolidated into a single clean `00001_init.sql` that includes the new tables and columns from the start. No incremental ALTER TABLE migrations. This avoids accumulating migration debt before launch.

The consolidated init migration includes:
- `units.access_level` column (with CHECK constraint)
- `unit_claim_mappings` table
- `group_settings` with `default_access_unknown`, `default_access_troop`, `default_access_role` columns
- All other existing tables and indexes

### New table: unit_claim_mappings

```sql
CREATE TABLE unit_claim_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    unit_id uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    claim_scope text NOT NULL,
    claim_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ucm_scope_check CHECK (claim_scope IN ('group', 'troop')),
    CONSTRAINT ucm_unique_claim UNIQUE (group_id, claim_scope, claim_id)
);
CREATE INDEX idx_ucm_group ON unit_claim_mappings(group_id);
```

### Modified table: units

```sql
-- Added column
access_level text NOT NULL DEFAULT 'book',
CONSTRAINT units_access_level_check CHECK (access_level IN ('view', 'book', 'trusted', 'manager'))
```

### Modified table: group_settings

```sql
-- Added columns
default_access_unknown text NOT NULL DEFAULT 'view',
default_access_troop text NOT NULL DEFAULT 'book',
default_access_role text NOT NULL DEFAULT 'book',
CONSTRAINT gs_access_unknown_check CHECK (default_access_unknown IN ('view', 'book', 'trusted', 'manager')),
CONSTRAINT gs_access_troop_check CHECK (default_access_troop IN ('view', 'book', 'trusted', 'manager')),
CONSTRAINT gs_access_role_check CHECK (default_access_role IN ('view', 'book', 'trusted', 'manager'))
```

### Modified table: users

```sql
-- Added column
active_group_id text REFERENCES groups(id)  -- nullable, for multi-group users
```

Seed data for Mälarscouterna (units, claim mappings, access levels) moves to the seed script / `init-group` CLI instead of being baked into migrations.

## Manager UI for access levels

The current profile page is renamed to **Inställningar** (`/settings`). The landing page links to it as "Profil & inställningar". The page header still shows the user's name. It keeps the existing tabs structure: personal settings tab (all users) and group settings tab (managers only). The access level management lives in the group settings tab.

A new section in the group settings tab: **Avdelningar och roller**.

### Layout

The section is organized as four columns/groups — one per access level — displayed as a kanban-style layout on desktop and stacked cards on mobile:

```
┌─ Visa ──────┐ ┌─ Boka ──────┐ ┌─ Betrodd ───┐ ┌─ Ansvarig ──┐
│             │ │ Yggdrasil   │ │ Valborgs-   │ │ Utrustnings-│
│             │ │ Spindlarna  │ │ kommittén   │ │ gruppen     │
│             │ │ Valarna     │ │ Läger       │ │ IT-gruppen  │
│             │ │ Flaskpost.  │ │             │ │             │
└─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘
```

Each unit is shown as a card/chip with:
- **Name** (editable — click to rename)
- **Type badge**: "Avd." or "Roll" (small, muted)
- **Claim ID**: "troop:17443" or "group:it_manager" (small helper text, muted)

Units can be moved between columns by:
- Drag and drop (desktop)
- A dropdown/select on the unit card (mobile, or as alternative to drag)

This makes the access structure immediately visible — the manager sees at a glance which troops and roles have which access level.

### Editing unit names

Click the unit name to edit it inline. This is the primary way managers fix auto-generated placeholder names ("Avdelning 17443" → "Yggdrasil"). The claim mapping is immutable after creation — only the display name changes.

**Future**: When the OIDC provider includes display names as custom claims, auto-created units will have proper names from the start. The rename UI remains for corrections.

### Adding units

Button "Lägg till" at the bottom of the section opens a form:
- Name (text input)
- Type: Avdelning / Roll (radio or toggle)
- Kopplad till (claim mapping): scope dropdown (Avdelning / Kårroll) + ID text input
- Access level defaults to the column the button was clicked in, or the group default for the type

The claim mapping fields use plain language: "Avdelning" maps to `troop` scope, "Kårroll" maps to `group` scope. The ID is the Scoutnet troop/role ID.

### Deleting units

Delete button (trash icon) on each unit card. Blocked if the unit has active bookings — shows count of affected bookings. Deleting a unit also removes its `unit_claim_mappings` row. Users with that claim will get a new auto-created unit on next login.

### Group defaults

Three dropdowns above the columns:
- "Standardnivå för okända användare" → `default_access_unknown`
- "Standardnivå för nya avdelningar" → `default_access_troop`
- "Standardnivå för nya roller" → `default_access_role`

These control what access level auto-created units start at. Changing a default does not retroactively change existing units.

### Group name editing

Managers can rename their group from the settings page. This updates `groups.name` — a display name that appears on the landing page, nav bar, and anywhere the group is referenced. The group ID (Keycloak org ID, e.g. "766") is immutable.

The `init-group` CLI sets the initial name. After that, the manager owns it.

### Multi-group users — group switcher

The group switcher lives on the landing page (not buried in settings). For users who belong to multiple configured groups, the landing page shows all their available groups with the active one highlighted. Clicking another group switches `active_group_id` and reloads.

The settings page also shows the active group name for context, but the switcher itself is on the landing page where it's immediately accessible.

## Dev seed script changes

The seed script currently creates units via `POST /api/v0/units` with just name + type, and relies on `role-mapping.json` for claim→unit mapping. After this change:

### New flow

```bash
# 1. init-group creates the group, group_settings, and first manager unit+mapping
docker compose exec api /app/server init-group \
  --group-id 766 --group-name "Mälarscouterna" \
  --manager-claim "group:766:material_responsible" --unit-name "Utrustningsgruppen"

# For test group
docker compose exec api /app/server init-group \
  --group-id 999 --group-name "Testkåren" \
  --manager-claim "group:999:admin" --unit-name "Admin"

# 2. Seed script creates remaining units with claim mappings + access levels
#    POST /api/v0/units now accepts: name, type, access_level, claim_scope, claim_id
```

### Unit creation API change

The `POST /api/v0/units` endpoint (manager-only) is extended:

```json
{
  "name": "Yggdrasil",
  "type": "unit",
  "access_level": "book",
  "claim_scope": "troop",
  "claim_id": "17443"
}
```

`access_level` defaults to the group default for the type if omitted. `claim_scope` and `claim_id` are optional — a unit can exist without a claim mapping (manually managed).

### Seed data

The seed script creates all units with their access levels and claim mappings:

```bash
# Troops (book level)
for entry in "Yggdrasil:troop:17443" "Spindlarna:troop:9109" "Valarna:troop:19260" "Flaskpostorné:troop:20956"; do
  IFS=: read NAME SCOPE ID <<< "$entry"
  curl -sf -X POST "$API/api/v0/units" -H "$HEADER" -H "Content-Type: application/json" \
    -d "{\"name\":\"$NAME\",\"type\":\"unit\",\"access_level\":\"book\",\"claim_scope\":\"$SCOPE\",\"claim_id\":\"$ID\"}" > /dev/null
done

# Roles with specific access levels
curl ... -d '{"name":"IT-gruppen","type":"role","access_level":"manager","claim_scope":"group","claim_id":"it_manager"}'
curl ... -d '{"name":"Valborgskommittén","type":"role","access_level":"trusted","claim_scope":"group","claim_id":"walpurgis_committee"}'
curl ... -d '{"name":"Läger","type":"role","access_level":"trusted","claim_scope":"group","claim_id":"group_camp_committee"}'
```

Note: Utrustningsgruppen is already created by `init-group` with `manager` access. The seed script skips it (or the API returns "already exists").

### What's removed from seed script

- The simple `for UNIT in ...` and `for PROJECT in ...` loops that created units without claim mappings
- Any reference to `role-mapping.json`

## Implementation plan

### Step 1: Database + init-group CLI

1. Migration `00016_access_levels.sql`
2. `init-group` CLI subcommand in `cmd/server/`
3. sqlc queries for `unit_claim_mappings` CRUD and access level lookups
4. Update seed script to use `init-group` instead of manual inserts

### Step 2: Auth middleware refactor

1. New `Claims` struct with `[]UnitMembership` and `MaxAccess`
2. Auth middleware: DB lookup for claim mappings, auto-create unknown units
3. Replace `RequireRole` with `RequireAccess`
4. Update all handlers using `claims.HasRole(...)` to use new access checks
5. Update dev persona format and loading
6. Update test helpers

### Step 3: Booking approval refactor

1. Submit handler: resolve access from booking's unit, apply new approval matrix
2. Keep `force_approval` — works the same as before, sends any booking to approval queue
3. Add items handler: check access when auto-transitioning confirmed bookings
4. Update booking visibility queries (replace role-based filtering with unit membership)

### Step 4: Frontend

1. Update `User` type and helpers in `$lib/user.ts`
2. Update layout and all pages using `hasRole()` to use new access checks
3. Update persona switcher for new format
4. Hide booking UI for `view`-level users
5. Settings page: show units with access levels instead of roles
6. Landing page: show group name dynamically, group switcher for multi-group users
7. Desktop nav: show `{user.name} · {group_name}`

### Step 5: Manager settings UI

1. Kanban-style access level columns on settings page
2. Unit cards with inline rename, type badge, claim ID
3. Move units between columns (drag-and-drop + dropdown fallback)
4. Add unit form with claim mapping
5. Delete unit (blocked if active bookings)
6. Group default dropdowns

### Step 6: Tests + cleanup

1. Update all integration tests for new Claims format
2. New tests: access level resolution, per-unit booking approval, view-only restrictions
3. Update smoke tests for view-only persona
4. Remove `role-mapping.json` from the codebase
5. Update all documentation (SPEC.md, API.md, README.md, project-context.md, guide.md)

## Migration path

Since this is pre-launch, the migration is a clean slate:

1. All existing migrations are consolidated into a single `00001_init.sql` with the new schema
2. `role-mapping.json` is deleted from the codebase
3. The seed script uses `init-group` CLI to bootstrap Mälarscouterna with claim mappings
4. `docker compose down -v` + `docker compose up --build` + `./dev-seed.sh` to start fresh

For the eventual production deployment, the `init-group` CLI is the entry point — no seed data in migrations.

## SPEC.md updates needed

When implementing this feature, the following sections of `docs/SPEC.md` need updating to stay coherent:

### Roles section
- Replace the three hardcoded roles (Leader, Project leader, Equipment manager) with the four access levels (view, book, trusted, manager)
- Remove the `project_leader` role entirely — its behavior is now covered by the `trusted` access level on specific units
- Remove references to roles coming from OIDC token claims mapping — access comes from unit membership + configurable access levels
- Document that manager is a group-wide admin capability, not unit-scoped

### Core Concepts → Bookings → Booking lifecycle
- Update the submission logic: replace "If any article has `low` approval and user is a project leader, auto-confirms" with the new approval matrix (trusted + manager auto-confirm `low`, `high` always needs approval)
- Keep `force_approval` description, clarify it works for any user at any access level
- Remove "If any article has `high` approval, only managers auto-confirm" — `high` always needs approval now

### Core Concepts → Bookings → Booking ownership
- Add that the booking's "used by" unit determines the access level for approval decisions
- Document personal booking = `book` level always
- Remove "Projects bypass article approval requirements" — replaced by per-unit access levels
- Replace `project` type references with `role` type
- Remove "project leaders can only book for projects they belong to" — replaced by generic unit membership check

### Users section
- Add `active_group_id` to the users table description
- Document multi-group support: group detection, preference storage, switching

### Multi-tenancy section
- Update to reflect multi-group users (currently says "deferred")
- Note that cross-group data isolation remains — only the active group is visible
- Group switching is session-level, not cross-group queries

### Data Model
- `units` table: add `access_level` column, change `type` from `unit`/`project` to `troop`/`role`
- Add `unit_claim_mappings` table
- `group_settings` table: add `default_access_unknown`, `default_access_troop`, `default_access_role` columns
- `users` table: add `active_group_id` column
- `groups` table: note that `name` is editable by managers (display name)
- Remove references to `role-mapping.json` as the source of role definitions

### Tech Stack → Architecture
- Remove `role-mapping.json` from the architecture description
- Document `init-group` CLI as the bootstrap mechanism
- Note auto-discovery of units from OIDC claims

### OIDC flow (step 6–7)
- Replace "extracts claims (member_id, name, email, roles, units, projects, group_id)" with the new flow: extract raw claims → look up unit_claim_mappings → resolve per-unit access levels → construct Claims with UnitMembership array
- Add multi-group resolution step
- Remove reference to role-mapping.json

### User Flows → Leader: Book equipment
- Update step 5 (submit) to reflect new approval logic based on unit access level

### User Flows → Equipment manager: Inventory
- Add access level management to the manager's capabilities
- Add group name editing
- Reference the settings page (renamed from profile)

### Implementation Plan
- Add a new phase/step for access levels implementation
- Mark `role-mapping.json` as replaced
- Update Phase 3 Step 1 (OIDC authentication) to note the claim mapping is now DB-driven
- Note removal of `project_leader` role and `project` unit type

### Dev and demo modes
- Update persona descriptions to use access levels instead of roles
- Add view-only persona

### Testing → Critical test scenarios
- Update access control tests to use access levels instead of roles
- Add test scenarios for: per-unit access resolution, multi-group login, view-only restrictions, access level changes by manager

## README.md updates needed

When implementing this feature, the following parts of `README.md` need updating:

### Status section
- Replace "Role-based access (leader, project leader, equipment manager)" with "Configurable per-unit access levels (view, book, trusted, manager)"
- Remove any mention of project leaders or project unit type
- Add "Auto-discovery of troops/roles from OIDC claims"
- Add "Multi-group support (group switching for members of multiple scout groups)"
- Remove any mention of `role-mapping.json`

### Stack section
- Remove reference to `role-mapping.json` if mentioned
- Note `init-group` CLI for group bootstrap

### Development section
- Update seed script description — it now uses `init-group` CLI instead of manual unit creation
- Remove `role-mapping.json` from the list of files or references
- Update dev persona descriptions to use access levels instead of roles
- Remove all references to "project leader" personas

### Deployment section → Files needed on the server
- Remove `role-mapping.json` from the file list
- Add note about `init-group` CLI for first-time setup

### Deployment section → Demo/Production deployment
- Add `init-group` step between "start" and "seed" in the deployment instructions
- Remove any `role-mapping.json` setup steps

### Environment modes table
- No changes needed (DEV_MODE/DEMO_MODE/BUILD_TARGET are unaffected)

### Security model
- Update to reflect that access levels are resolved from DB, not a static JSON file
- Note that the `init-group` CLI requires SSH access (secure bootstrap)

## guide.md updates needed

When implementing this feature, the following parts of `docs/guide.md` need updating:

### Introduction
- Replace "Mälarscouternas utrustning" with generic group-aware text (the group name is dynamic now)

### Boka utrustning
- Step 3 ("Välj vem bokningen gäller"): no change needed, but add a note that the access level of the chosen unit determines whether approval is needed
- Update the bullet about booking for units: access is per-unit, not per-role. Remove "projekt" distinction from unit picker description

### Godkännande section
- Replace the three-level description:
  - Current: "Projektledare får automatiskt godkännande" / "alla utom utrustningsansvariga"
  - New: Explain that approval depends on the *unit's* access level, not the user's role. Trusted units auto-confirm `low`. `high` always needs approval, even for managers.
- Remove all references to "projektledare" — the concept no longer exists
- Remove "Hög nivå — alla utom utrustningsansvariga behöver godkännande" — managers also need approval for `high` now
- Keep the `force_approval` description ("Vill du ha bekräftelse ändå?")

### Del 2: För utrustningsansvariga
- Add a new section: **Hantera åtkomstnivåer** — explain the settings page kanban UI, how to rename units, move them between access levels, add new units with claim mappings
- Add a new section: **Lägga till avdelningar och roller** — explain pre-creating units before first login
- Update "Godkänna bokningar" to note that managers' own bookings with `high` articles also appear in the queue
- Remove all references to "projektledare" as a role — replaced by access levels on units

### Navigation references
- Update any reference to "Profil" page → "Inställningar" (the page is renamed)
- The landing page link text is "Profil & inställningar"

### På gång section
- Remove items that are now implemented (if any)
- Add: "Automatiska namn från ScoutID — avdelnings- och rollnamn hämtas direkt från inloggningen"
- Remove any mention of "projektledare" from planned features

## Future work

- **OIDC display names**: The Keycloak/ScoutID provider will include troop and role display names as custom claims in the token (work in progress on the OIDC provider side). When available, auto-discovery will use these names directly instead of generating placeholder names like "Avdelning 17443". The rename UI stays for corrections but becomes rarely needed.
- **Group onboarding UI**: Currently groups are created via CLI. A future admin UI could let system admins manage groups through the browser.
- **Cross-group visibility**: Deferred. Each group is fully isolated. No cross-group booking or article sharing (except product images which already support sharing).
