# Organisation types (groups and districts)

Status: **implemented** for groups and districts. The parser accepts every
ScoutID entity type, so regions, organisations, corps, networks and projects
work without further auth changes once provisioned.

## Why

ScoutID exposes a `memberships` claim keyed by Scoutnet entity type:

- `groups` (kårer) — the only type with `name` and `is_primary`
- `districts`, `regions`, `organisations`, `corps`, `networks`, `projects` —
  roles only, **no name** and no `is_primary`
- `troops` — the user's troops, with an optional parent `groupId`

Before this change Scoutrustning only parsed `memberships.groups`, so a user
who only had a district role was rejected with `403 group_not_found`. The
tenant table also had no concept of organisation type.

ScoutID does **not** expose a district → member-group hierarchy. A leader in a
member kår has no district membership unless they personally hold a district
role. Letting member kårer book a district's equipment is therefore a separate
feature (see "Deferred" below).

## Data model

`groups` remains the tenant table. It is conceptually "organisation", but the
table and `group_id` columns are not renamed — that would touch every table,
query, API field and frontend type for no functional gain.

Migration `00024_organisation_types.sql` adds:

| Column | Notes |
|---|---|
| `org_type` | `group` (default), `district`, `region`, `organisation`, `corps`, `network`, `project` |
| `scoutnet_id` | Entity ID in that type's own Scoutnet namespace |

Constraints:

- `UNIQUE (org_type, scoutnet_id)` — group `111` and district `111` are
  different organisations.
- `CHECK (id = scoutnet_id)` for groups, and
  `CHECK (id = org_type || ':' || scoutnet_id)` for every other type.

### Tenant key

```
TenantKey(orgType, scoutnetID):
  group        → scoutnetID          // "766" (backwards compatible)
  other type   → orgType + ":" + id  // "district:111"
```

Existing group rows keep their raw Scoutnet group number as `groups.id`, so no
child-table data or URLs change. Non-group tenants are prefixed to avoid ID
namespace collisions. `auth.TenantKey` is the single source of truth;
`splitTenantKey` reverses it for dev personas and tooling.

## Auth

- `tokenMemberships` parses all seven entity types plus troops.
- `parseOrgMemberships` flattens them into one deterministic list (`OrgTypes`
  order, IDs sorted within each type).
- `OrgMembership` carries both `ID` (Scoutnet ID) and `TenantID` (internal
  `groups.id`, empty when unregistered), plus `Type`.
- For each org, the middleware computes `TenantKey(type, id)` and checks the
  tenant exists. Registered orgs are the ones offered by the switcher.
- Higher-level entities have no name in the token, so their display name comes
  from `groups.name` (set during provisioning).
- `pickActiveGroup` prefers the `X-Active-Group-Id` hint (matched against
  `TenantID`), then `is_primary`, then the stable first entry. Only groups can
  be primary; a member who belongs to both a kår and a district defaults to the
  kår.
- Roles of the active organisation are emitted with claim scope `group`. The
  scope means "role in the active tenant" regardless of entity type, so
  existing `team_claim_mappings` and `init-group` mappings keep working.
- District roles have no translated `name`; auto-created teams use a humanised
  role key (`distriktsstyrelsen` → "Distriktsstyrelsen").

## Onboarding

`init-group` (alias `init-org`) gains `--org-type` and `--org-id`:

```bash
init-group --org-type=district --org-id=111 \
  --group-name="Mälardalens distrikt" \
  --role-key=distriktsstyrelsen --team-name="Distriktsstyrelsen"
```

`--group-id` remains as an alias for `--org-id`. The command computes the
tenant key itself and writes `org_type` + `scoutnet_id`.

`/join` reads groups and districts from the JWT, shows `name || key` for roles
(district roles have no name), and submits `org_type`. The handler validates the
`(org_type, org_id)` pair against the token, not just the ID. The admin email
includes the type and a ready-to-run `init-group --org-type=... --org-id=...`
command. When ScoutID has no name for the entity, the applicant types it in.

The form exposes groups and districts. Other types are deliberately hidden until
there is a reason to offer them; the backend already accepts them. To expose a
new type, add it to `SIGNUP_ORG_TYPES` in `web/src/lib/org.ts`.

## Frontend

- `/me` adds `group_type` and returns `groups[]` as
  `{ id (tenant key), scoutnet_id, type, name, is_primary }`.
- `$lib/org.ts` maps types to labels and defines the signup type list.
- The organisation switcher shows the type next to each name when a member
  belongs to more than one organisation.
- Generic UI copy keeps kår terminology in Swedish and group wording in
  English; district users read the same text. Type labels ("Kår", "Distrikt")
  appear only where the type itself is shown (switcher, signup, signup email).

## Multi-tenancy

Unchanged: `group_id` is still the tenant key on every table and every query
filters on it. The only difference is that non-group tenants use a prefixed key
such as `district:111`. Existing group data needs no backfill beyond the two new
`groups` columns.

## Deferred: member groups booking district equipment

This is the likely Phase 2. Requirements:

- Local `district_member_groups (district_group_id, member_group_id)` table;
  ScoutID has no hierarchy claim.
- Auth rule granting a user whose token group is a linked member group access to
  the district tenant, with a configurable default access level.
- A district setting for whether member-group booking is enabled.
- UI to manage member groups and to show the district in those members'
  switcher.

Not implemented here. District tenants currently serve people who hold a
district role in ScoutID.

## Tests

- `auth_test.go`: tenant key round-trip, deterministic org parsing, active-org
  selection with a prefixed district.
- `organisation_types_test.go`: a district persona resolves to `district:111`,
  gets manager access from its team, sees only district-scoped data, and `/me`
  reports the type; `/join` accepts a district and emits the typed init command.
- The `view-only` dev persona (Vera Visa) is also manager of the seeded
  `district:111` tenant, provisioned by `dev-seed.sh`.
