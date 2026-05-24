# Workflow

## Purpose

Defines how the AI assistant should interact with the user: when to ask, when to act, what to verify, and how to finish work. These are behavioral expectations, not coding style rules.

## Priority

Critical — these override all other rules.

## Instructions

### Before writing code

- Read `docs/SPEC.md` for requirements before making architectural decisions.
- Clarify anything ambiguous before implementing.
- Never start writing or modifying files until the user explicitly approves the plan. Present what you intend to do, wait for a go-ahead, then implement.
- Build minimal viable first. Get things working end-to-end before adding complexity. Don't over-scaffold or create stubs for things not yet needed.

### While implementing

- Multi-tenancy is non-negotiable. Every new table gets `group_id`. Every new query filters on it. No shortcuts.
- Before modifying a handler, read the existing handler file and the corresponding query file to understand current patterns.
- After changing sqlc queries, regenerate with `sqlc generate`. Do not edit generated files in `internal/db/` by hand.
- After adding i18n keys to `sv.json`/`en.json`, run `pnpm run build` (from `web/`) to compile Paraglide. Do not edit generated files in `web/src/lib/paraglide/` by hand — they are overwritten on every build.
- Modify files with a clear intent per modification. Don't make all changes at the same time, but also don't divide same file edits into too small chunks.
- Try avoiding asking for permission more than needed. Structure your queries so that you need less follow up queries, if they need approval.

### After implementing

- If any change requires a container rebuild, database reset, migration run, or other manual step, explicitly state what the user needs to do (e.g. `docker compose up --build`, `./dev-seed.sh`, restart a service). Never assume the user knows which changes require a rebuild.
- Keep documentation updated proactively — don't wait to be asked:
  - `docs/API.md` — when adding or changing API endpoints
  - `docs/SPEC.md` — when making architectural decisions or completing spec milestones. The spec is both a plan and an overarching implementation document. Completed steps that later changed must note the original was accomplished per spec, then add an **UPDATE** section with the current status. Overall descriptions (flows, architecture, data model) must always reflect the current state. Future/planned sections can describe intended behavior.
  - `.amazonq/rules/project-context.md` — when architecture, conventions, or project structure changes
  - `docs/BACKLOG.md` and `docs/accomplished.md` — anything deferred goes in the backlog, completed items move to accomplished. Never leave resolved items in the backlog.
  - When building a complex feature, add a dedicated doc in `docs/` (e.g. `docs/availability.md`) documenting design decisions and trade-offs, and reference it from `accomplished.md`.

### When the user says we're done

Only the user decides when work is finished. Never assume a task is complete or offer a commit message unless the user explicitly says so.

When the user says done or finished, perform this checklist:

1. **Code quality**: No TODO comments, placeholder logic, or incomplete implementations left behind.
2. **Duplication**: No duplicated constants, labels, or utility functions across files. Shared code extracted to `$lib/` modules. No unused SQL queries, API endpoints, or dead code.
3. **Security**: No missing auth checks, unscoped queries, or credential leaks.
3. **Multi-tenancy**: `group_id` filtering on all new queries.
4. **Style**: Code matches existing conventions (see `coding-conventions.md`).
5. **Tests**: New functionality has integration tests covering the happy path. Run the full test suite: `cd api && go test ./internal/handler/tests/ -timeout 180s -count=1 2>&1 && bash ../smoke-test.sh`
6. **Svelte warnings**: Run `cd web && PATH=/usr/local/bin:$PATH pnpm run check` and verify zero errors and warnings. Requires Node 24 — the system default `node` is v18 which cannot load the Vite plugin. Do not introduce new warnings.
7. **Documentation** — verify each of these is still accurate and update if needed:
   - `docs/API.md` — reflects any new or changed endpoints
   - `docs/SPEC.md` — reflects current architecture and decisions
   - `docs/BACKLOG.md` — any resolved items removed
   - `docs/accomplished.md` — completed work logged
   - `.amazonq/rules/project-context.md` — reflects current state
   - `.amazonq/rules/coding-conventions.md` — any new patterns captured
   - `README.md` — reflects current status and setup instructions
   - `docs/guide.md` — user-facing guide reflects current UI and features
8. **Commit message**: Suggest a message following [Conventional Commits](https://www.conventionalcommits.org/). Explain the version impact (`feat:` → minor bump, `fix:` → patch bump, `feat!:` → major bump).

### Commits

- Always show the commit message for user approval before committing. Never commit without asking.

## Error Handling

- If the user's request conflicts with the spec, point out the conflict and ask how to proceed.
- If a change would break existing tests, flag it before implementing.
- If unsure whether something needs user approval (e.g. new table, architectural change), ask. When in doubt, ask.
