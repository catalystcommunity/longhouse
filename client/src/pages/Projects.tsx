import { A } from "@solidjs/router";
import { For, Show, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { projectClient } from "~/data/clients";
import { useCurrentHouseId } from "~/stores/auth";
import type { Project } from "~/api/types.gen";

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
    async (h) => projectClient.listProjects({ houseId: h }),
  );

  const [composerOpen, setComposerOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Projects <em>in flight</em></h2>
        <p class="lead">Longer pieces of work with their own members, tasks, and discussion.</p>
        <div style="margin-top:12px">
          <button class="btn btn-primary" onClick={() => setComposerOpen((v) => !v)}>
            {composerOpen() ? "Cancel" : "New project"}
          </button>
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
        when={!projects.loading}
        fallback={<p style="padding:24px;color:var(--ink-mute)">Loading…</p>}
      >
        <Show when={(projects() ?? []).length > 0} fallback={<EmptyProjects />}>
          <div class="rooms">
            <For each={projects()!}>{(p, i) => <ProjectCard p={p} delay={2 + (i() % 4)} />}</For>
          </div>
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

export const ProjectCard = (props: { p: Project; delay?: number }) => (
  <A
    href={`/projects/${props.p.projectId}`}
    class={`room ${swatchFor(props.p.projectId)} reveal ${props.delay ? `d${props.delay}` : ""}`}
  >
    <Show when={props.p.category}>{(c) => <span class="corner">{c()}</span>}</Show>
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
      await projectClient.createProject({
        houseId: props.houseId,
        name: name().trim(),
        description: description().trim() || undefined,
        category: category().trim() || undefined,
      } as any);
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
