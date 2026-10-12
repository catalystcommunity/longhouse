import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { A } from "@solidjs/router";
import { projectClient } from "~/data/clients";
import {
  PROJECT_LIST_LIMIT,
  loadProjectTasks,
  projectsContainingTask,
} from "~/lib/projects";
import type { Task } from "@longhouse/client";

/**
 * Task-side "Add to project" control. Lives in the task detail editor so a
 * task can be attached without going to the project page first (the
 * project-side attach picker in ProjectDetail does the reverse).
 *
 * Loads the house's projects plus each project's task list (tolerant of
 * per-project failures) so it can show which projects already hold the
 * task, leave those out of the picker, and append at the end of the
 * chosen project's list. The server enforces authz (edit on the project);
 * any error is shown inline.
 */
export const AddToProject = (props: { task: Task; onAdded: () => Promise<unknown> }) => {
  const [index, { refetch }] = createResource(
    () => props.task.houseId,
    async (houseId) => {
      const projects = (await projectClient.listProjects({ houseId, limit: PROJECT_LIST_LIMIT })).projects;
      const tasksByProject = await loadProjectTasks(projectClient, houseId, projects);
      return { projects, tasksByProject };
    },
  );
  const [pick, setPick] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const ready = () => (index.state === "ready" || index.state === "refreshing" ? index.latest : undefined);

  const containing = createMemo(() => {
    const idx = ready();
    if (!idx) return [];
    const ids = projectsContainingTask(idx.tasksByProject, props.task.taskId);
    return idx.projects.filter((p) => ids.has(p.projectId));
  });
  const choices = createMemo(() => {
    const idx = ready();
    if (!idx) return [];
    const inIds = new Set(containing().map((p) => p.projectId));
    return idx.projects.filter((p) => !inIds.has(p.projectId));
  });

  const add = async () => {
    const pid = pick();
    const idx = ready();
    if (!pid || !idx || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      // Append after every row the project holds, including rows the
      // caller can't see. Fetch on demand when the fan-out missed it.
      let pt = idx.tasksByProject.get(pid);
      if (!pt) {
        const res = await projectClient.listProjectTasks({
          houseId: props.task.houseId,
          projectId: pid,
          limit: PROJECT_LIST_LIMIT,
        });
        pt = { tasks: res.tasks, hiddenCount: Number(res.hiddenCount ?? 0) };
      }
      await projectClient.addProjectTask({
        projectId: pid,
        taskId: props.task.taskId,
        position: pt.tasks.length + pt.hiddenCount,
      });
      setPick("");
      await Promise.all([refetch(), props.onAdded()]);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Show when={ready() && ready()!.projects.length > 0}>
      <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
        <span style="font-size:11px;color:var(--ink-mute);margin-right:4px">Projects</span>
        <For each={containing()}>
          {(p) => (
            <A
              href={`/projects/${p.projectId}`}
              style="display:inline-flex;align-items:center;padding:3px 10px;border-radius:9999px;font-size:12px;border:1px solid var(--grass-4);background:color-mix(in oklab, var(--grass-1) 30%, var(--paper));color:var(--ink);text-decoration:none"
            >
              {p.name}
            </A>
          )}
        </For>
        <Show when={choices().length > 0}>
          <span style="display:inline-flex;gap:6px;align-items:center;flex-wrap:wrap">
            <select
              aria-label="Add to project"
              value={pick()}
              onChange={(e) => setPick(e.currentTarget.value)}
              disabled={busy()}
              style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);color:var(--ink);font-size:13px;max-width:220px"
            >
              <option value="">Add to project…</option>
              <For each={choices()}>{(p) => <option value={p.projectId}>{p.name}</option>}</For>
            </select>
            <button
              type="button"
              class="btn btn-ghost"
              onClick={add}
              disabled={!pick() || busy()}
              style="padding:6px 12px"
            >
              {busy() ? "Adding…" : "Add"}
            </button>
          </span>
        </Show>
      </div>
      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>
    </Show>
  );
};
