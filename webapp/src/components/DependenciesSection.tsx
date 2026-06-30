import { For, Show, createResource, createSignal } from "solid-js";
import { dependencyClient, projectClient, taskClient } from "~/data/clients";
import type { DependencyNode, DependencyNodeType } from "@longhouse/client";

/**
 * "Depends on" editor for a single work item (task or project). Shows the
 * items this one depends on (the stored, forward direction) and lets the
 * caller add/remove dependencies. The reverse direction ("what depends on
 * this") is intentionally NOT surfaced here — the API returns it, but the
 * product only edits the forward edges.
 *
 * The component is self-contained: it fetches the current dependency graph
 * and the house's tasks + projects (the add candidates) itself, so it drops
 * straight into any detail panel with just (nodeType, nodeId, houseId).
 */

interface Props {
  nodeType: "task" | "project";
  nodeId: string;
  houseId: string;
}

export interface Candidate {
  type: "task" | "project";
  id: string;
  label: string;
}

export const nodeKey = (type: string, id: string) => `${type}:${id}`;

/** Parse a "type:id" picker value (ids may themselves contain ':'). */
export const splitNodeValue = (v: string): { type: DependencyNodeType; id: string } => {
  const i = v.indexOf(":");
  // Node values are built as `${type}:${id}` where type is always "task"|"project".
  return { type: v.slice(0, i) as DependencyNodeType, id: v.slice(i + 1) };
};

/** Candidates not already in the dependency list. */
export const filterAddable = (
  candidates: Candidate[],
  dependencies: { type: unknown; id: string }[],
): Candidate[] => {
  const have = new Set(dependencies.map((d) => nodeKey(String(d.type), d.id)));
  return candidates.filter((c) => !have.has(nodeKey(c.type, c.id)));
};

export const DependenciesSection = (props: Props) => {
  const [graph, { refetch }] = createResource(
    () => ({ type: props.nodeType, id: props.nodeId }),
    async (t) => dependencyClient.getDependencies({ type: t.type, id: t.id }),
  );

  // Candidate pool: every task + project in the house, minus this item itself.
  const [candidates] = createResource(
    () => props.houseId,
    async (h): Promise<Candidate[]> => {
      const [tasks, projects] = await Promise.all([
        taskClient.listTasks({ houseId: h }).then((r) => r.tasks),
        projectClient.listProjects({ houseId: h }).then((r) => r.projects),
      ]);
      const out: Candidate[] = [];
      for (const t of tasks) {
        if (props.nodeType === "task" && t.taskId === props.nodeId) continue;
        out.push({ type: "task", id: t.taskId, label: t.title });
      }
      for (const p of projects) {
        if (props.nodeType === "project" && p.projectId === props.nodeId) continue;
        out.push({ type: "project", id: p.projectId, label: p.name });
      }
      return out;
    },
  );

  const [selected, setSelected] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  // Candidates not already depended on, so the picker hides them.
  const addable = () => filterAddable(candidates() ?? [], graph()?.dependencies ?? []);

  const add = async () => {
    const v = selected();
    if (!v || busy()) return;
    const { type, id } = splitNodeValue(v);
    setBusy(true);
    setErr(null);
    try {
      await dependencyClient.addDependency({
        dependentType: props.nodeType,
        dependentId: props.nodeId,
        dependencyType: type,
        dependencyId: id,
      });
      setSelected("");
      await refetch();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (d: DependencyNode) => {
    setBusy(true);
    setErr(null);
    try {
      await dependencyClient.removeDependency({
        dependentType: props.nodeType,
        dependentId: props.nodeId,
        dependencyType: d.type,
        dependencyId: d.id,
      });
      await refetch();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style="display:flex;flex-direction:column;gap:8px;padding-top:6px;border-top:1px solid var(--line)">
      <span style="font-size:11px;color:var(--ink-mute)">Depends on</span>

      <Show
        when={(graph()?.dependencies ?? []).length > 0}
        fallback={<span style="font-size:12px;color:var(--ink-mute)">Nothing yet.</span>}
      >
        <div style="display:flex;flex-wrap:wrap;gap:6px">
          <For each={graph()!.dependencies}>
            {(d) => (
              <span style="display:inline-flex;align-items:center;gap:6px;padding:3px 6px 3px 10px;border-radius:9999px;font-size:12px;border:1px solid var(--line);background:var(--paper);color:var(--ink)">
                <span style="font-size:10px;color:var(--ink-mute);text-transform:uppercase">{String(d.type)}</span>
                {d.title}
                <button
                  type="button"
                  title="Remove dependency"
                  onClick={() => remove(d)}
                  disabled={busy()}
                  style="border:none;background:none;cursor:pointer;color:var(--ink-mute);font-size:14px;line-height:1;padding:0 2px"
                >
                  ×
                </button>
              </span>
            )}
          </For>
        </div>
      </Show>

      <div style="display:flex;gap:6px;align-items:center">
        <select
          value={selected()}
          onChange={(e) => setSelected(e.currentTarget.value)}
          disabled={busy()}
          style="flex:1;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        >
          <option value="">Add a dependency…</option>
          <For each={addable()}>
            {(c) => (
              <option value={nodeKey(c.type, c.id)}>
                {c.type === "project" ? "◆ " : ""}
                {c.label}
              </option>
            )}
          </For>
        </select>
        <button
          type="button"
          class="btn btn-ghost"
          onClick={add}
          disabled={busy() || !selected()}
          style="padding:6px 12px"
        >
          Add
        </button>
      </div>

      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>
    </div>
  );
};
