# Task & project dependencies (design + reference)

Status: **implemented.** Migration `000013_dependencies.sql`, the CSIL
`DependencyService`, the Go handler (`api/internal/csilservices/dependency.go`),
the store methods (`postgres_store.go`), and the SPA `DependenciesSection`
component are all wired and tested.

This document is the source of truth for how Longhouse models **dependencies
between work items** — "this task/project can't proceed until those
tasks/projects are done" — and the access required to read or change them.

**Implementation map:**
- Spec: `csil/longhouse.csil` (`DependencyNodeType`, `DependencyRef`,
  `DependencyTarget`, `DependencyNode`, `DependencyGraph`, `DependencyService`).
- Handler: `api/internal/csilservices/dependency.go`.
- Store: `dependencies` table + `Add/Remove/List Dependencies`,
  `ListDependents`, `DependencyPathExists`, `RemoveDependenciesForNode` in
  `postgres_store.go`; model in `store/postgres/models/dependency.go`.
- SPA: `client/src/components/DependenciesSection.tsx` (used by
  `TaskDetailEditor.tsx` and `pages/ProjectDetail.tsx`).
- Tests: `csilservices/dependency_test.go` (unit, fake store),
  `cmd/dependency_postgres_test.go` (`-tags=integration`, real Postgres),
  `client/src/components/DependenciesSection.test.ts` (picker helpers).

---

## 1. Goals & principles

- **Either end is a task or a project.** A dependency edge connects two work
  items; both the dependent and the dependency may independently be a task or a
  project. There are four shapes: task→task, task→project, project→task,
  project→project.
- **Multiple dependencies per item.** An item may depend on any number of
  others, and be depended on by any number of others.
- **Stored one direction only.** The data model is deliberately minimal: one
  row per edge, flowing **from the dependent** (the item that has the
  dependency) **to the dependency** (the item it requires). The reverse view —
  "what depends on X" — is **never stored**; it is computed by querying the same
  table with the columns swapped.
- **No cycles.** An edge that would create a dependency cycle (direct or
  transitive) is rejected. The check runs in the database (recursive CTE).
- **Enforced by the API, never the UI.** The SPA only edits the forward
  direction, but every access and integrity check is server-side.

---

## 2. Data model

One polymorphic table (`coredb/migrations/000013_dependencies.sql`):

```sql
CREATE TYPE dependency_node_type AS ENUM ('task', 'project');

CREATE TABLE dependencies (
    house_id        uuid                 NOT NULL REFERENCES houses ON DELETE CASCADE,
    dependent_type  dependency_node_type NOT NULL,  -- the item that HAS the dependency
    dependent_id    uuid                 NOT NULL,
    dependency_type dependency_node_type NOT NULL,  -- the item it REQUIRES
    dependency_id   uuid                 NOT NULL,
    created_at      timestamptz          NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (dependent_type, dependent_id, dependency_type, dependency_id)
);

CREATE INDEX dependencies_dependency_idx ON dependencies (dependency_type, dependency_id);
CREATE INDEX dependencies_house_idx      ON dependencies (house_id);
```

Design notes:

- **No foreign key to `tasks`/`projects`.** The columns are polymorphic (a
  single column can't reference two tables), and tasks are *soft-deleted*
  (`deleted_at`), so a CASCADE wouldn't fire on the common delete path anyway.
  Integrity is handled instead at the edges:
  - dangling/soft-deleted endpoints are **filtered at read time** (a
    soft-deleted task or a missing row is treated as "gone" and omitted);
  - project **hard-deletes** clear their edges explicitly
    (`DeleteProject` → `RemoveDependenciesForNode`, both directions);
  - **house deletion** cascades via the `house_id` FK.
- **`house_id` scopes everything.** Both endpoints of an edge always live in the
  same house (enforced by the handler), so `house_id` is unambiguous and powers
  bulk cleanup + tenant isolation.
- **The PK is the whole edge**, so a duplicate add is a no-op
  (`AddDependency` uses `ON CONFLICT DO NOTHING`).
- **`dependencies_dependency_idx`** serves both the reverse lookup
  ("what depends on X") and the cycle check, which walks forward from the
  proposed dependency along `dependent → dependency` edges.

### API fields that are NOT columns

The wire types carry more than the table stores. These are computed/enriched
per request and have **no storage behind them**:

- `DependencyGraph.dependents` — the reverse direction, assembled by querying
  the table with the columns swapped.
- `DependencyNode.title` / `DependencyNode.status` — joined from the underlying
  task/project at read time (`@receive-only` in the spec).

---

## 3. CSIL surface

```
DependencyNodeType = "task" / "project"

DependencyRef    = { dependent_type, dependent_id, dependency_type, dependency_id }
DependencyTarget = { type, id }
DependencyNode   = { type, id, @receive-only title, @receive-only ?status }
DependencyGraph  = { dependencies: [*DependencyNode], dependents: [*DependencyNode] }

service DependencyService {
    add-dependency:    DependencyRef    -> EmptyResponse   / ServiceError,
    remove-dependency: DependencyRef    -> EmptyResponse   / ServiceError,
    get-dependencies:  DependencyTarget -> DependencyGraph / ServiceError
}
```

- **`add-dependency`** records that `dependent` depends on `dependency`.
- **`remove-dependency`** drops that edge (idempotent — removing a
  non-existent edge succeeds).
- **`get-dependencies`** returns, for one target, both `dependencies`
  (what it depends on — the stored direction) and `dependents` (what depends on
  it — the computed reverse). Each list is filtered to nodes the caller may
  read; deleted/missing endpoints are dropped.

---

## 4. Access required

Access reuses the RBAC model in [`rbac.md`](./rbac.md): the four levels
`none < read < edit < full`, resolved per resource by
`policy.taskAccess` / `policy.projectAccess`.

| Operation | On the **dependent** | On the **dependency** / **target** |
|---|---|---|
| `add-dependency` | **edit** | **read** |
| `remove-dependency` | **edit** | — (not checked) |
| `get-dependencies` | **read** (the target) | per-node **read** to appear in the result |

Rationale:

- **Adding/removing an edge mutates the dependent's metadata**, so it requires
  `edit` on the dependent — the same bar as editing the dependent's other
  content fields.
- **The dependency only needs to be readable.** You may link to a task/project
  you can see but not edit. Requiring only `read` (not `edit`) lets a
  collaborator declare "my task is blocked by that other team's project"
  without write access to it.
- **Privacy is not leaked.** If the caller can't read the dependency, the add is
  rejected as **`404 not found`** (not `403`), so the op never confirms the
  existence of something the caller shouldn't see.
- **`get-dependencies` filters silently.** Nodes the caller can't read are
  omitted from both lists with no count or placeholder — consistent with "don't
  surface private titles." (This differs from list endpoints that report a
  `hidden_count`; dependency reads do not.)

Both endpoints must belong to the **same house** (`400` otherwise). Admins and
resource owners resolve to `full` and therefore always pass the gates within
their house.

---

## 5. Integrity rules

Enforced by `add-dependency`, in order:

1. **Valid shape** — `dependent_type`/`dependency_type` ∈ {task, project},
   ids non-empty → else `400`.
2. **No self-edge** — an item cannot depend on itself (same type *and* id) →
   `400`.
3. **Both endpoints exist and are live** — a missing or soft-deleted endpoint →
   `404` (dependent) or `404` (dependency; also used for the read-gate so
   existence isn't leaked).
4. **Same house** → else `400`.
5. **Access gates** — `edit` on dependent, `read` on dependency (§4).
6. **No cycle** — if a dependency path already runs from the proposed
   *dependency* back to the proposed *dependent*, adding the edge would close a
   loop → **`409 conflict`**.

### Cycle detection (in the database)

`DependencyPathExists(from, to)` answers "following `dependent → dependency`
edges, is `to` reachable from `from`?" via a recursive CTE:

```sql
WITH RECURSIVE reach AS (
        SELECT ?::dependency_node_type AS t, ?::uuid AS id        -- seed: `from`
    UNION
        SELECT d.dependency_type, d.dependency_id
        FROM dependencies d
        JOIN reach r ON d.dependent_type = r.t AND d.dependent_id = r.id
)
SELECT EXISTS (SELECT 1 FROM reach WHERE t = ?::dependency_node_type AND id = ?::uuid);
```

To add `X depends-on Y`, the handler checks `DependencyPathExists(Y, X)` — if Y
can already reach X, the new edge would close a cycle. `UNION` (not
`UNION ALL`) dedupes visited nodes, so the walk terminates even on
pre-existing cyclic data.

---

## 6. Webapp behavior

The SPA edits **only the forward direction** ("what does this depend on"). The
reverse `dependents` list is returned by the API but is intentionally **not
surfaced** in the UI today.

`DependenciesSection` (in `TaskDetailEditor` and the project detail page) shows
the current dependencies as removable chips and offers a picker of the house's
other tasks/projects to add. It is self-contained: it fetches the dependency
graph and the candidate pool (`listTasks` + `listProjects`, minus itself)
itself, given only `(nodeType, nodeId, houseId)`. Server errors (e.g. the
`409` cycle rejection) surface inline.

---

## 7. Decisions (settled) & remaining unknowns

**Settled (2026-06-07):**
1. **One-directional storage**, reverse computed. The simplest model that meets
   the requirement; the reverse view is cheap via `dependencies_dependency_idx`. ✅
2. **Dedicated `DependencyService`** (not methods split across Task/Project) —
   the edge is polymorphic on both ends, so one service avoids duplicating the
   logic. ✅
3. **`edit` on dependent, `read` on dependency** for writes (§4). ✅
4. **Full cycle detection in the DB** via recursive CTE, not just self-loop
   rejection. ✅
5. **Silent filtering** of unreadable nodes in `get-dependencies` (no
   `hidden_count`). ✅

**Remaining unknowns (don't block the build):**
- **Reverse-direction UI** — `dependents` is delivered but unused; surface it if
  a "blocks" view is wanted.
- **Status rollups** — e.g. "this task is blocked because a dependency is still
  open." The data is present (`DependencyNode.status`); no derived
  blocked-state is computed yet.
- **Scale** — resolution does per-node lookups when enriching titles. Fine at
  house scale; revisit (batch fetch) only if profiling shows the read path
  needs it, mirroring the rbac.md §3.5 stance.
