# RBAC, privacy & resource access (design)

Status: **implemented (first cut).** Migrations `000011_rbac.sql` (additive
grant tables + visibility) and `000012_rbac_foldin.sql` (fold project
members/owners into grants, drop the legacy tables) are in. The CSIL spec,
the Go resolver (`api/internal/csilservices/policy.go`), the Task/Project
handlers, and the SPA wrapper are wired. The materialized ancestor path
(§3.5) is the only deferred piece — the resolver currently walks ancestors at
read time via recursive CTE. Decisions may still be tuned.

This document is the source of truth for how Longhouse decides **who can see and
do what** to tasks and projects. It supersedes the ad-hoc "house member sees
everything" model.

**Implementation map:**
- Resolver / policy: `api/internal/csilservices/policy.go` (`taskAccess`,
  `projectAccess`, `maxAllowedTaskVisibility`, grantee/group expansion).
- Handlers: `task.go` and `project.go` (visibility filtering on lists with
  `hidden_count`, `edit`/`full` gates, `set-*-visibility`, `*-grant` ops).
- Store: `task_grants` / `project_grants` + resolver helpers in
  `postgres_store.go`; the legacy `Add/Remove/List ProjectMember/Owner` store
  methods are now a **compatibility facade over `project_grants`** (owners =
  member grantee @ full, members = member grantee @ edit) — the six CSIL ops
  and their SPA usage are unchanged.
- Group membership is NOT in the bearer token (it carries house + roles only),
  so the resolver looks up the caller's groups once per request
  (`ListGroupIDsForMember`).

---

## 1. Goals & principles

- **Tasks are public within a house by default.** There are **no free-floating
  private tasks** — privacy is conferred by *containment*. To make a task
  private, put it in a private project (or under a private parent task). A task
  with no container is house-visible and cannot be made private on its own.
- **Four access levels everywhere:** `none < read < edit < full`.
  - `none` — cannot see the resource; it appears only as a "*N hidden*" count.
  - `read` — view all content **and add comments**; no other mutation.
  - `edit` — modify *content* fields; cannot touch *governance* fields or delete.
  - `full` — everything, including governance (owner, project membership,
    visibility, grants) and delete.
- **Access is the MOST-PERMISSIVE path that reaches you** (a task seen through
  any project you can access is visible), **bounded by an umbrella guardrail on
  how permissive a resource's own default may be set** (a task can't be made
  more public than its most-private container). These two rules sound opposed;
  §4 shows why they aren't.
- **Enforced by the API, never the UI.** The UI may hide controls for polish,
  but every level/guardrail check is server-side.
- **Aggregates over private work still count** — "you can hide a *task*, you
  can't hide *that you hid one*" (§5).
- **Grants for members now, groups next** (same `access_level` vocabulary).
  **External sharing is deferred** to the existing `shares` table and blocked on
  linkkeys domain-trust; it reuses the level vocabulary (`read`/`edit`).
- **Must scale** to ~hundreds of thousands of members and hundreds of millions
  to billions of tasks, given adequate Postgres (incl. future Vitess/Multigres
  sharding). PostgreSQL only — no SQLite — so we use array/GIN indexes, partial
  indexes, and declarative partitioning freely.

---

## 2. Current state (what the schema already gives us)

Verified against `coredb/migrations/*` and the GORM models:

- **`tasks`** — `house_id`; `parent_task_id` (single self-FK, `ON DELETE
  CASCADE`, indexed → a real tree); **`owner_member_id` is `NOT NULL` and is
  literally the renamed `created_by` column** (migration `000002`), so a task's
  owner *is* its creator until reassigned. The creator-fallback rule is already
  satisfied for tasks with zero work.
- **Task ↔ project is ALREADY many-to-many.** `project_tasks (project_id,
  task_id, position, created_at)`, PK `(project_id, task_id)`, both FKs `ON
  DELETE CASCADE`, indexed on `task_id` and `(project_id, position)`. "The same
  task may appear in multiple projects with independent positions." CSIL already
  exposes `add-project-task` / `remove-project-task` / `list-project-tasks`.
  **We build privacy on this table — no new join table, no link backfill.**
- **`projects`** — `house_id`, `name`, `description`, `category`, `status`,
  timestamps. **No owner column and no `created_by` column.** "Owners" are a
  *set* in the metadata-only `project_owners` join, alongside `project_members`
  (both display-only, not authz today). Also `milestones`.
- **Authz today**: house membership + `admin`/`member` role + owner-can-mutate
  for tasks/events. Every house member can read everything in the house.
- **`shares`** table + store methods exist but the service is unregistered /
  unimplemented. Reserved for **external** (linkkeys) read/edit access.

---

## 3. Target data model

### 3.1 Access level & grantee enums
```
access_level  = 'none' | 'read' | 'edit' | 'full'   -- ordered
grantee_type  = 'member' | 'group'                  -- 'external' lives in `shares`
```

### 3.2 Visibility (the "house-at-large" surface)
Add `visibility access_level NOT NULL DEFAULT 'read'` to **`tasks`** and
**`projects`**. This is the level granted to a house member who reaches the
resource through **no other surface** (no project, no grant). Default `read`
preserves today's "everyone in the house can see it."

**The default a *new project* gets is a house setting** (`default_project_visibility`,
§6), not hard-wired. It defaults to `read` (public house-wide), but a house may
set it to `none` (private-by-default) or even `edit`/`full` if they want members
to be able to change project content by default. The column default (`read`) is
just the fallback when the setting is unset; the create-project handler stamps
the configured value.

**Tasks have no meaningful visibility default to choose:** a free-floating task
is always `read` (no free-floating private tasks), and a task inside a project
has its own `visibility` floored to its container by the umbrella guardrail
(§4.2) — so the project's setting dominates regardless. The task column default
stays `read`.

### 3.3 Owner & creator fallback
Rule: **if there is no owner, the creator is the owner**, for all analysis.
- **Tasks**: satisfied already (`owner_member_id` NOT NULL = creator).
- **Projects**: add **`created_by_member_id`** (nullable FK → members, `ON
  DELETE SET NULL`). `project_owners` is the authoritative owner *set*; when it's
  empty, fall back to `created_by_member_id`.

### 3.4 Per-resource grant tables (additive grants)
Per-resource (not one polymorphic table) to keep **real FKs + cascade delete**
and a known query target. Shared `access_level` enum so resolution is identical.
```
task_grants    (task_id   FK→tasks   ON DELETE CASCADE, house_id, grantee_type,
                grantee_id, access_level, created_at,
                PK (task_id, grantee_type, grantee_id))
project_grants (project_id FK→projects ON DELETE CASCADE, house_id, grantee_type,
                grantee_id, access_level, created_at,
                PK (project_id, grantee_type, grantee_id))
```
Indexes lead with `house_id` (tenant = shard boundary) and add a reverse index
`(house_id, grantee_type, grantee_id)` for "what can X / group G see?" and for
the list-page filter. These tables are intended to **replace** `project_owners` /
`project_members` (migrate owners → `full`, members → `edit`; then drop) — but
that fold-in is deferred to the handler phase (§9), since those joins are still
live source-of-truth today.

### 3.5 Materialized ancestor path (scaling — deferred to a follow-up migration)
The subtask tree (`parent_task_id`) means a subtask inherits grants/visibility
from ancestors. A read-time recursive CTE is correct and is what the **first
cut** uses. For the list/filter hot path at scale, a follow-up migration adds a
materialized ancestor `path` (`uuid[]` GIN-indexed) to `tasks`, maintained on
insert/move via recursive CTE (the rare write), so resolution avoids per-row
recursion:
- inherited grants: `task_grants WHERE task_id = ANY(path)`
- nearest visibility override: deepest ancestor in `path` with one set.

This is an optimization, not a semantic change — kept out of the first migration
to avoid shipping trigger machinery before the model is proven.

---

## 4. Resolution semantics

A task is reachable through a **set of surfaces**. The two rules that sound
contradictory operate on different things:

### 4.1 Effective access = MAX over reachable surfaces (most-permissive)
Effective access for member X on a resource = the **maximum** level X obtains via
any of:

1. **house-at-large** — the resource's own `visibility`, **iff** X is a house
   member — but **capped at resolve time by the MIN visibility of its
   containers** (ancestor tasks and containing projects). This cap is what
   makes "a task in a private project is private to the house" hold even if the
   task's own `visibility` was never explicitly lowered: a `read` task whose
   only project is `none` resolves its house-at-large surface to `none`. The
   write-time guardrail (§4.2) keeps `visibility` from being *set* above the
   cap; the resolver re-applies the cap on read so attaching to a private
   project after the fact still hides it. (Implemented in `policy.taskAccess`.)
2. **each project** the task belongs to (directly via `project_tasks`, or via an
   *ancestor task's* projects) — X's effective access to that project. This is
   **additive**: a member of a containing project keeps their access even when
   surface 1 is capped to `none` by a *different*, more-private project. (So a
   task in both a public and a private project is visible to the whole house
   via the public one — "see it through any project you have access to".)
3. **grants** — `task_grants` along the ancestor chain ∪ `project_grants` of
   reachable projects, matching X **or any group X belongs to**. Additive:
   MAX across all matches (this is the superset/overlap resolver — group-read +
   group-full ⇒ full).
4. **owner** (creator-fallback) and **house admin** ⇒ always `full`.

> Adding a task to more projects only ever *widens* who can see it.
> **"You can see a task if you have access to any project it's in."**

### 4.2 The umbrella guardrail = MIN of containers (write-time validation)
A resource's **own `visibility`** (only the house-at-large surface — not grants)
**cannot be set more permissive than the MIN visibility among its containers**:
its projects and its parent task.

- A task in a private project can't be made house-public (no leaking up).
- A subtask can't out-expose its parent — **umbrella privacy**.
- A free-floating top-level task has no container; it's bounded by the rule that
  free-floating tasks stay house-visible and can't be set private.

This is the *only* place a rule is restrictive; everything else is permissive.
Grants always *add* access on top — the guardrail constrains the default surface,
not who you can explicitly grant in.

**Effective access** = `MAX(`house-default-if-member, additive grants,
owner/admin⇒full`)`. One Go function, fed by a few indexed lookups.

### 4.3 Worked example
Task **T** in **public** Project A and **private** (`none`) Project B:
- T's own `visibility` floors to `none` (guardrail: MIN(public, none) = none) — a
  house member in neither project sees nothing.
- A member of A sees T via A (MAX picks up A's level).
- A member of B sees T via B.
This is exactly "add a task to multiple projects to get visibility differences."

---

## 5. Aggregates: hide the task, not that you hid it

Rollups (`% complete`, hours, counts, future analytics) compute **server-side
over ALL rows, ignoring the viewer's visibility** — private tasks count toward a
project's totals. **Content retrieval applies the visibility filter.**

API consequence: list endpoints return visible rows **plus a count of hidden
ones** — `{ items: [...], hidden_count: N }` — so the *existence* of hidden work
is never itself concealed. This shape is baked into the CSIL response types
(§6), not bolted on per-handler.

---

## 6. CSIL / API changes (next implementation phase)

Spec edits to `csil/longhouse.csil`, then `./csil/regenerate.sh`, then implement
handlers. Listed here so the doc is the single plan; **not yet applied** (the
spec change + regen + handler impl is one coherent build-breaking pass).

**Common types** — note `AccessLevel` **already exists** in `longhouse.csil` as
`AccessLevel = "read"` (single value, reserved for `shares`). **Widen** it
rather than adding a new type, so tasks/projects and shares share one
vocabulary:
```
AccessLevel  = "none" / "read" / "edit" / "full"   ;; widened from "read"
GranteeType  = "member" / "group"
Grant        = { grantee_type: GranteeType, grantee_id: UUID,
                 access_level: AccessLevel }
```
(`shares` only ever uses `read`/`edit`, a subset — no Share change needed.)

**New house setting** — add to `EffectiveSettings` (house-layer, admin-writable):
```
? default_project_visibility: AccessLevel .default "read"
```
The create-project handler stamps a new project's `visibility` from this key
(falling back to `read` when unset). Lets a house choose private-by-default
(`none`) or content-open-by-default (`edit`/`full`).

**`Task`** — add `visibility: AccessLevel`. (owner already present.)
**`Project`** — add `visibility: AccessLevel`, `? created_by_member_id: MemberID`.

**List responses** — wrap to carry the hidden count, e.g.
```
TaskList = { tasks: [* Task], hidden_count: uint }
```
and switch `list-tasks` / `list-project-tasks` to return it.

**`TaskService`** — add
```
set-task-visibility:  SetVisibilityRequest -> Task / ServiceError
list-task-grants:     TaskRef -> [* Grant] / ServiceError
put-task-grant:       PutGrantRequest -> EmptyResponse / ServiceError
delete-task-grant:    DeleteGrantRequest -> EmptyResponse / ServiceError
```
**`ProjectService`** — the same four for projects (`set-project-visibility`,
`list/put/delete-project-grant`).

All new mutations require **`full`** on the target (governance). Resolution
(§4) gates every existing read/mutation per the field matrix (§7).

---

## 7. Field matrix — `edit` vs `full` (FIRST CUT, adjustable)

`read` = view + comment. `edit` = content. `full` = governance + delete.

### Tasks
| Field / action | Min level |
|---|---|
| title, description, status, due_at, tag, estimate_minutes | edit |
| assignee (member / skill) | edit |
| recurrence fields | edit |
| create a comment | read |
| edit / delete **own** comment | read (author) |
| delete **others'** comments (moderation) | full |
| **owner_member_id** (reassign owner) | full |
| **project membership** (`add`/`remove-project-task`) | full |
| **parent_task_id** (re-parent) | full |
| **visibility** + **task_grants** (manage access) | full |
| **delete task** | full |

### Projects
| Field / action | Min level |
|---|---|
| name, description, category, status | edit |
| add / remove **tasks** to the project | edit |
| **owner(s)** / created_by | full |
| **visibility** + **project_grants** (manage access) | full |
| archive project | full |
| **delete project** | full |

Known edges:
- Adding a task to a project (an `edit`-on-project action) changes that task's
  surfaces and can *widen* who sees the task — accept it, but surface the
  cross-resource effect in API docs + UI. The task's own umbrella guardrail
  still holds (its own default can't exceed the new container's visibility).
- Group grants resolve by MAX across overlapping/superset groups.

---

## 8. Scaling notes

- **House-scoping is the scalability mechanism**, not the grant table. Every
  authz query carries `house_id`; the working set is one house, never the global
  billions. `house_id` leads every index and is the natural **shard key** for
  Vitess/Multigres — grant resolution stays single-shard. Invariant to protect:
  **no authz query escapes its house** (no cross-house joins).
- **Hierarchy**: recursive CTE for single-resource resolve and for move-time
  maintenance; materialized `path` (§3.5) for the list hot path.
- Both grant tables stay small per house and are covered by the leading-
  `house_id` and reverse `(house_id, grantee_type, grantee_id)` indexes.

---

## 9. Migration plan

**`coredb/migrations/000011_rbac.sql` (this PR — strictly additive, inert):**
1. `access_level` + `grantee_type` enums.
2. `visibility access_level NOT NULL DEFAULT 'read'` on `tasks` & `projects`
   (matches today's everyone-can-see — so the migration changes no behaviour
   until the resolver ships).
3. `projects.created_by_member_id`.
4. `task_grants`, `project_grants` + indexes.

It does **not** touch `project_members` / `project_owners`, and does **not**
backfill grants — those tables are still the live source of truth for
`ProjectService`, `BugService`, and comment notification fan-out. Dropping or
shadowing them before the handlers move would break the running app.

No `project_tasks` work (already M:N). No task-owner backfill (already creator).

**Handler phase (`000012_rbac_foldin.sql` — DONE):**
- Folds `project_owners` → `project_grants` (`full`) and `project_members` →
  `project_grants` (`edit`), backfills `projects.created_by_member_id` from the
  earliest owner, then drops the two metadata tables.
- **Deviation from the original plan:** the six `Add/Remove/List
  ProjectMember/Owner` store methods + CSIL ops were *kept* as a thin
  compatibility facade over `project_grants` (owners ⇔ member grantee @ full,
  members ⇔ member grantee @ edit), rather than removed. This avoided a large
  SPA rewrite of the project-detail members/owners UI while still making
  `project_grants` the single source of truth. `BugService` and `comment.go`
  read through the facade unchanged.
- create-project now seeds the creator as an owner (full) grant and stamps
  `visibility` from `default_project_visibility`.

**Follow-up (deferred):** materialized `path` + maintenance (§3.5);
surfacing `hidden_count` in the SPA (the value is plumbed end-to-end through
the API and TS types but the wrapper currently discards it — no "N hidden"
affordance in the UI yet).

---

## 10. Decisions (settled) & remaining unknowns

**Settled (2026-05-30):**
1. **New-project default visibility** is a **house setting**
   (`default_project_visibility`, §3.2/§6), defaulting to `read`. Tasks have no
   meaningful default to choose (free-floating ⇒ `read`; in-project ⇒ floored to
   container). ✅
2. **`edit`-vs-`full` field matrix** — §7 first cut stands (`edit` = content,
   `full` = governance + delete). Adjustable later. ✅
3. **`project_members` folds in as `edit`** (owners as `full`). Note: this
   *grants* edit on fold-in — intentional, members are treated as content
   collaborators. ✅
4. **Group grants ship in the first cut** (`grantee_type='group'`). The resolver
   expands the caller's group memberships and MAXes across member + group grants
   (§4.1.3). This is the headline feature, not a follow-up. ✅
5. **Effective visibility is computed from the ancestor walk on read** for the
   first cut. Add an `effective_visibility` denormalized column + maintenance
   only if real-data profiling shows the list path needs it. ✅

**Remaining unknowns (don't block the build):**
- `effective_visibility` / materialized `path` — revisit under real data load.
- External `shares` (deferred): reuse `access_level` (`read`/`edit`) on the
  existing `shares` table once linkkeys domain-trust lands.
