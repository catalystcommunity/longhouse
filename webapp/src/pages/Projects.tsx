import { A } from "@solidjs/router";
import { For, Show, createEffect, createMemo, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { projectClient } from "~/data/clients";
import {
  PROJECT_LIST_LIMIT,
  effectiveProjectsFilter,
  loadMyProjectIds,
  readProjectsFilter,
  writeProjectsFilter,
  type ProjectsFilter,
} from "~/lib/projects";
import { normalizeProjectCreate } from "~/lib/taskCreate";
import { currentMemberId, useCurrentHouseId } from "~/stores/auth";
import type { Project } from "@longhouse/client";

const SWATCHES = ["r1", "r2", "r3", "r4"] as const;
const swatchFor = (id: string) => {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) | 0;
  return SWATCHES[Math.abs(h) % SWATCHES.length];
};

export const ProjectsPage = () => {
  const houseId = useCurrentHouseId();
  const [projects, { refetch }] = createResource(
    () => houseId(),
    async (h) => (await projectClient.listProjects({ houseId: h, limit: PROJECT_LIST_LIMIT })).projects,
  );

  // All / Mine filter. Mine = projects I created (created_by fallback owner)
  // or hold an explicit owner/member grant on — same rule as the dashboard
  // (lib/projects.ts). The choice is remembered per member in localStorage;
  // with nothing saved it defaults to Mine when I have any projects.
  const [myIds] = createResource(
    () => {
      const me = currentMemberId();
      const ps = projects.state === "ready" || projects.state === "refreshing" ? projects.latest : undefined;
      if (!me || !ps) return null;
      return { me, ps };
    },
    async ({ me, ps }) => loadMyProjectIds(projectClient, ps, me),
  );
  const [saved, setSaved] = createSignal<ProjectsFilter | undefined>();
  createEffect(() => {
    const me = currentMemberId();
    setSaved(me ? readProjectsFilter(me) : undefined);
  });
  const myIdsLoaded = () => (myIds.state === "ready" || myIds.state === "refreshing" ? myIds.latest : undefined);
  const filter = () => effectiveProjectsFilter(saved(), myIdsLoaded()?.size ?? 0);
  const choose = (v: ProjectsFilter) => {
    setSaved(v);
    const me = currentMemberId();
    if (me) writeProjectsFilter(me, v);
  };
  const shown = createMemo(() => {
    const ps = projects() ?? [];
    if (filter() === "all") return ps;
    const mine = myIdsLoaded() ?? new Set<string>();
    return ps.filter((p) => mine.has(p.projectId));
  });
  // Hold the list until "mine" is known, so the default doesn't flip from
  // All to Mine (or show an empty Mine list) mid-load. A failed grant
  // lookup degrades to the created_by rule inside loadMyProjectIds.
  const filterReady = () =>
    !!myIdsLoaded() || myIds.state === "errored" || !currentMemberId();

  const [composerOpen, setComposerOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Projects <em>in flight</em></h2>
        <p class="lead">Longer pieces of work with their own members, tasks, and discussion.</p>
        <div style="margin-top:12px;display:flex;flex-wrap:wrap;gap:12px;align-items:center">
          <button class="btn btn-primary" onClick={() => setComposerOpen((v) => !v)}>
            {composerOpen() ? "Cancel" : "New project"}
          </button>
          <div class="seg" role="group" aria-label="Show projects">
            <button type="button" class={filter() === "all" ? "on" : ""} aria-pressed={filter() === "all"} onClick={() => choose("all")}>
              All
            </button>
            <button type="button" class={filter() === "mine" ? "on" : ""} aria-pressed={filter() === "mine"} onClick={() => choose("mine")}>
              Mine
            </button>
          </div>
        </div>
      </div>

      <Show when={composerOpen()}>
        <ProjectComposer
          houseId={houseId()!}
          onCancel={() => setComposerOpen(false)}
          onCreated={async () => { setComposerOpen(false); await refetch(); }}
        />
      </Show>

      <Show
        when={!projects.loading && filterReady()}
        fallback={<p style="padding:24px;color:var(--ink-mute)">Loading…</p>}
      >
        <Show when={(projects() ?? []).length > 0} fallback={<EmptyProjects />}>
          <Show when={shown().length > 0} fallback={<EmptyMine onShowAll={() => choose("all")} />}>
            <div class="rooms">
              <For each={shown()}>{(p, i) => <ProjectCard p={p} delay={2 + (i() % 4)} />}</For>
            </div>
          </Show>
        </Show>
      </Show>
    </AuthGate>
  );
};

const EmptyProjects = () => (
  <section
    style="margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)"
  >
    <p style="margin:0 0 4px"><b>No projects yet.</b></p>
    <p style="margin:0">Create a project to gather related tasks, members and milestones in one place.</p>
  </section>
);

const EmptyMine = (props: { onShowAll: () => void }) => (
  <section
    style="margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)"
  >
    <p style="margin:0 0 4px"><b>You are not in a project yet.</b></p>
    <p style="margin:0">
      <button type="button" class="btn-quiet" onClick={props.onShowAll}>Show all projects</button>
    </p>
  </section>
);

export const ProjectCard =(props: { p: Project; delay?: number }) => (
  <A
    href={`/projects/${props.p.projectId}`}
    class={`room ${swatchFor(props.p.projectId)} reveal ${props.delay ? `d${props.delay}` : ""}`}
  >
    <Show
      when={props.p.personal}
      fallback={<Show when={props.p.category}>{(c) => <span class="corner">{c()}</span>}</Show>}
    >
      <span class="corner">Private</span>
    </Show>
    <div class="swatch">{props.p.name.charAt(0)}</div>
    <h3>{props.p.name}</h3>
    <Show when={props.p.description}>{(d) => <p>{d()}</p>}</Show>
  </A>
);

const ProjectComposer = (props: {
  houseId: string;
  onCancel: () => void;
  onCreated: () => Promise<void> | void;
}) => {
  const [name, setName] = createSignal("");
  const [category, setCategory] = createSignal("");
  const [description, setDesc] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!name().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await projectClient.createProject(normalizeProjectCreate({
        houseId: props.houseId,
        name: name().trim(),
        description: description().trim() || undefined,
        category: category().trim() || undefined,
      }));
      await props.onCreated();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style="margin:16px 0;padding:18px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);display:grid;grid-template-columns:2fr 1fr;gap:12px"
    >
      <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
        <span style="font-size:12px;color:var(--ink-mute)">Name</span>
        <input
          type="text"
          value={name()}
          onInput={(e) => setName(e.currentTarget.value)}
          required
          autofocus
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Category (optional)</span>
        <input
          type="text"
          value={category()}
          onInput={(e) => setCategory(e.currentTarget.value)}
          placeholder="Operations, Events, …"
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
        <span style="font-size:12px;color:var(--ink-mute)">Description (optional)</span>
        <textarea
          value={description()}
          onInput={(e) => setDesc(e.currentTarget.value)}
          rows="2"
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
        />
      </label>
      <Show when={err()}>
        {(m) => <p style="color:var(--rust);font-size:13px;grid-column:1/-1;margin:0">{m()}</p>}
      </Show>
      <div style="grid-column:1/-1;display:flex;justify-content:flex-end;gap:10px">
        <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
        <button class="btn btn-primary" disabled={busy() || !name().trim()} type="submit">
          {busy() ? "Saving…" : "Create project"}
        </button>
      </div>
    </form>
  );
};
