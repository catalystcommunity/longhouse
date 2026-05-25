import { A, useNavigate, useParams } from "@solidjs/router";
import { For, Show, batch, createMemo, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { memberClient, projectClient } from "~/data/clients";
import { displayName, initial, memberSwatch, toAvatar } from "~/lib/derive";
import { hasRole } from "~/stores/auth";
import type { Member, Milestone, Project } from "~/api/types.gen";

const MILESTONE_STATES = ["future", "current", "done"] as const;

interface DetailBundle {
  project: Project;
  houseMembers: Member[];
  members: Member[];
  owners: Member[];
  milestones: Milestone[];
  doneTasks: number;
  totalTasks: number;
}

export const ProjectDetail = () => {
  const params = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const projectId = () => params.slug;

  const [bundle, { refetch }] = createResource(projectId, async (id): Promise<DetailBundle | null> => {
    try {
      const project = await projectClient.getProject(id);
      const [houseMembers, members, owners, milestones, tasks] = await Promise.all([
        memberClient.listMembers({ houseId: project.houseId }),
        projectClient.listProjectMembers(id),
        projectClient.listProjectOwners(id),
        projectClient.listMilestones(id),
        projectClient.listProjectTasks({ houseId: project.houseId, projectId: id }),
      ]);
      const totalTasks = tasks.length;
      const doneTasks = tasks.filter((t) => t.status === "done").length;
      return { project, houseMembers, members, owners, milestones, totalTasks, doneTasks };
    } catch {
      return null;
    }
  });

  const progress = createMemo(() => {
    const b = bundle();
    if (!b || b.totalTasks === 0) return 0;
    return Math.round((b.doneTasks / b.totalTasks) * 100);
  });

  const [editing, setEditing] = createSignal(false);
  const isAdmin = () => hasRole("admin");

  const onDeleteProject = async () => {
    const b = bundle();
    if (!b) return;
    if (!confirm(`Delete project "${b.project.name}"? This is permanent.`)) return;
    try {
      await projectClient.deleteProject(b.project.projectId);
      navigate("/projects", { replace: true });
    } catch (e) {
      alert(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <AuthGate>
      <Show
        when={!bundle.loading}
        fallback={<p style="padding:48px;text-align:center;color:var(--ink-mute)">Loading…</p>}
      >
        <Show
          when={bundle()}
          fallback={
            <div style="padding:80px 0;text-align:center;color:var(--ink-mute)">
              <p>Project not found.</p>
              <p><A href="/projects" style="color:var(--ocean-2)">Back to projects</A></p>
            </div>
          }
        >
          {(b) => {
            const p = () => b().project;
            return (
              <>
                <div class="divider">
                  <span class="rule" />
                  <span class="label">project detail</span>
                  <span class="rule" />
                </div>

                <article class="room-detail reveal">
                  <div class="room-banner">
                    <span class="banner-tag">
                      {p().category ?? "Project"} · {progress()}% complete
                    </span>
                  </div>

                  <div class="room-meta">
                    <div class="room-title">
                      <div class="crumbs">
                        <A href="/projects">Projects</A>
                        <i />
                        <Show when={p().category}>{(c) => <>{c()}<i /></>}</Show>
                        <span>{p().name}</span>
                      </div>

                      <Show
                        when={!editing()}
                        fallback={
                          <ProjectEdit
                            project={p()}
                            onCancel={() => setEditing(false)}
                            onSaved={async () => { setEditing(false); await refetch(); }}
                          />
                        }
                      >
                        <h2>{p().name}</h2>
                        <Show when={p().description}>{(d) => <p class="lede">{d()}</p>}</Show>
                        <Show when={isAdmin()}>
                          <div style="margin-top:12px;display:flex;gap:10px">
                            <button class="btn btn-ghost" onClick={() => setEditing(true)}>Edit</button>
                            <button class="btn btn-ghost" onClick={onDeleteProject} style="color:var(--rust)">
                              Delete project
                            </button>
                          </div>
                        </Show>
                      </Show>
                    </div>

                    <aside class="facts">
                      <dl>
                        <dt>Owners</dt>
                        <dd>
                          <PeopleEditor
                            kind="owner"
                            project={p()}
                            assigned={b().owners}
                            houseMembers={b().houseMembers}
                            canEdit={isAdmin()}
                            onChanged={async () => { await refetch(); }}
                          />
                        </dd>
                        <dt>Members</dt>
                        <dd>
                          <PeopleEditor
                            kind="member"
                            project={p()}
                            assigned={b().members}
                            houseMembers={b().houseMembers}
                            canEdit={isAdmin()}
                            onChanged={async () => { await refetch(); }}
                          />
                        </dd>
                        <dt>Started</dt>
                        <dd>{new Date(p().createdAt).toLocaleDateString(undefined, { month: "long", day: "numeric", year: "numeric" })}</dd>
                        <dt>Tasks</dt>
                        <dd>{b().doneTasks} of {b().totalTasks} done</dd>
                      </dl>
                    </aside>
                  </div>

                  <MilestoneList
                    project={p()}
                    milestones={b().milestones}
                    canEdit={isAdmin()}
                    onChanged={async () => { await refetch(); }}
                  />
                </article>
              </>
            );
          }}
        </Show>
      </Show>
    </AuthGate>
  );
};

// ─── Project header edit ──────────────────────────────────────────────

const ProjectEdit = (props: {
  project: Project;
  onCancel: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const [name, setName] = createSignal(props.project.name);
  const [category, setCategory] = createSignal(props.project.category ?? "");
  const [description, setDesc] = createSignal(props.project.description ?? "");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await projectClient.updateProject({
        ...props.project,
        name: name().trim() || props.project.name,
        category: category().trim() || undefined,
        description: description().trim() || undefined,
      } as any);
      await props.onSaved();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style="display:grid;grid-template-columns:2fr 1fr;gap:10px;margin-top:8px"
    >
      <label style="grid-column:1/-1;display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Name</span>
        <input
          type="text" value={name()} onInput={(e) => setName(e.currentTarget.value)}
          autofocus
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Category</span>
        <input
          type="text" value={category()} onInput={(e) => setCategory(e.currentTarget.value)}
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <label style="grid-column:1/-1;display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Description</span>
        <textarea
          value={description()} onInput={(e) => setDesc(e.currentTarget.value)}
          rows="3"
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
        />
      </label>
      <Show when={err()}>{(m) => <p style="color:var(--rust);grid-column:1/-1;margin:0">{m()}</p>}</Show>
      <div style="grid-column:1/-1;display:flex;justify-content:flex-end;gap:10px">
        <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
        <button class="btn btn-primary" disabled={busy()} type="submit">
          {busy() ? "Saving…" : "Save"}
        </button>
      </div>
    </form>
  );
};

// ─── Members / Owners picker ──────────────────────────────────────────

const PeopleEditor = (props: {
  kind: "member" | "owner";
  project: Project;
  assigned: Member[];
  houseMembers: Member[];
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}) => {
  const [selected, setSelected] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const available = createMemo(() => {
    const taken = new Set(props.assigned.map((m) => m.memberId));
    return props.houseMembers.filter((m) => !taken.has(m.memberId));
  });

  const add = async () => {
    const mid = selected();
    if (!mid || busy()) return;
    setBusy(true);
    try {
      if (props.kind === "owner") {
        await projectClient.addProjectOwner({ projectId: props.project.projectId, memberId: mid });
      } else {
        await projectClient.addProjectMember({ projectId: props.project.projectId, memberId: mid });
      }
      setSelected("");
      await props.onChanged();
    } catch (e) {
      alert(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (m: Member) => {
    if (busy()) return;
    setBusy(true);
    try {
      if (props.kind === "owner") {
        await projectClient.removeProjectOwner({ projectId: props.project.projectId, memberId: m.memberId });
      } else {
        await projectClient.removeProjectMember({ projectId: props.project.projectId, memberId: m.memberId });
      }
      await props.onChanged();
    } catch (e) {
      alert(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style="display:flex;flex-direction:column;gap:8px">
      <Show
        when={props.assigned.length > 0}
        fallback={<span style="color:var(--ink-mute);font-size:13px">None yet.</span>}
      >
        <Show
          when={props.canEdit}
          fallback={
            <Show when={props.kind === "owner"}>
              <span>{props.assigned.map(displayName).join(", ")}</span>
            </Show>
          }
        >
          <div style="display:flex;flex-wrap:wrap;gap:8px">
            <For each={props.assigned}>
              {(m) => (
                <span style="display:inline-flex;align-items:center;gap:6px;padding:4px 8px 4px 4px;background:color-mix(in oklab, var(--paper) 90%, var(--ocean-1) 10%);border:1px solid var(--line);border-radius:9999px">
                  <span class={`a sm ${memberSwatch(m.memberId)}`}>{initial(m)}</span>
                  <span style="font-size:13px">{displayName(m)}</span>
                  <button
                    type="button"
                    onClick={() => remove(m)}
                    disabled={busy()}
                    title="Remove"
                    style="background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:14px;line-height:1;padding:0 2px"
                  >
                    ×
                  </button>
                </span>
              )}
            </For>
          </div>
        </Show>

        <Show when={!props.canEdit && props.kind === "member"}>
          <AvatarStack bits={props.assigned.map(toAvatar)} />
        </Show>
      </Show>

      <Show when={props.canEdit && available().length > 0}>
        <div style="display:flex;gap:6px;align-items:center">
          <select
            value={selected()}
            onChange={(e) => setSelected(e.currentTarget.value)}
            style="flex:1;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          >
            <option value="">Add a {props.kind}…</option>
            <For each={available()}>{(m) => <option value={m.memberId}>{displayName(m)}</option>}</For>
          </select>
          <button
            type="button"
            class="btn-quiet"
            onClick={add}
            disabled={busy() || !selected()}
          >
            Add
          </button>
        </div>
      </Show>
    </div>
  );
};

// ─── Milestones ───────────────────────────────────────────────────────

const MilestoneList = (props: {
  project: Project;
  milestones: Milestone[];
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}) => {
  const [adding, setAdding] = createSignal(false);
  return (
    <div style="margin-top:24px">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
        <h3 style="margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)">Milestones</h3>
        <Show when={props.canEdit}>
          <button class="btn-quiet" onClick={() => setAdding((v) => !v)}>
            {adding() ? "Cancel" : "+ Add milestone"}
          </button>
        </Show>
      </div>

      <Show when={adding()}>
        <MilestoneForm
          project={props.project}
          onCancel={() => setAdding(false)}
          onSaved={async () => { setAdding(false); await props.onChanged(); }}
        />
      </Show>

      <Show
        when={props.milestones.length > 0}
        fallback={
          <p style="color:var(--ink-mute);font-size:13px;margin:0">
            <Show when={props.canEdit} fallback={<>No milestones yet.</>}>
              No milestones yet — add the first to start the timeline.
            </Show>
          </p>
        }
      >
        <div class="ribbon">
          <For each={props.milestones}>
            {(m) => (
              <MilestoneItem milestone={m} canEdit={props.canEdit} onChanged={props.onChanged} />
            )}
          </For>
        </div>
      </Show>
    </div>
  );
};

const MilestoneItem = (props: {
  milestone: Milestone;
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}) => {
  const [editing, setEditing] = createSignal(false);
  const remove = async () => {
    if (!confirm(`Delete milestone "${props.milestone.label}"?`)) return;
    try {
      await projectClient.deleteMilestone(props.milestone.milestoneId);
      await props.onChanged();
    } catch (e) {
      alert(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <Show
      when={!editing()}
      fallback={
        <MilestoneForm
          project={{ projectId: props.milestone.projectId } as Project}
          existing={props.milestone}
          onCancel={() => setEditing(false)}
          onSaved={async () => { setEditing(false); await props.onChanged(); }}
        />
      }
    >
      <div class={`rib ${props.milestone.state}`} style="position:relative">
        <span class="when">{props.milestone.whenLabel}</span>
        <span class="what">{props.milestone.label}</span>
        <Show when={props.canEdit}>
          <div style="display:flex;gap:6px;margin-top:6px">
            <button class="btn-quiet" style="font-size:11px;padding:2px 8px" onClick={() => setEditing(true)}>edit</button>
            <button class="btn-quiet" style="font-size:11px;padding:2px 8px;color:var(--rust)" onClick={remove}>delete</button>
          </div>
        </Show>
      </div>
    </Show>
  );
};

const MilestoneForm = (props: {
  project: Project;
  existing?: Milestone;
  onCancel: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const [label, setLabel] = createSignal(props.existing?.label ?? "");
  const [whenLabel, setWhenLabel] = createSignal(props.existing?.whenLabel ?? "");
  const [state, setState] = createSignal((props.existing?.state as string) ?? "future");
  const [position, setPosition] = createSignal(props.existing?.position ?? 0);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!label().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      if (props.existing) {
        await projectClient.updateMilestone({
          ...props.existing,
          label: label().trim(),
          whenLabel: whenLabel().trim(),
          state: state() as any,
          position: position(),
        });
      } else {
        await projectClient.createMilestone({
          milestoneId: "",
          projectId: props.project.projectId,
          label: label().trim(),
          whenLabel: whenLabel().trim(),
          state: state() as any,
          position: position(),
          createdAt: "",
          updatedAt: "",
        });
      }
      batch(() => { setLabel(""); setWhenLabel(""); });
      await props.onSaved();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style="margin:0 0 12px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:2fr 1fr 1fr 80px auto;gap:10px;align-items:end"
    >
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Label</span>
        <input
          type="text" value={label()} onInput={(e) => setLabel(e.currentTarget.value)}
          required autofocus
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">When</span>
        <input
          type="text" value={whenLabel()} onInput={(e) => setWhenLabel(e.currentTarget.value)}
          placeholder="May · current"
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">State</span>
        <select
          value={state()} onChange={(e) => setState(e.currentTarget.value)}
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        >
          <For each={MILESTONE_STATES}>{(s) => <option value={s}>{s}</option>}</For>
        </select>
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Order</span>
        <input
          type="number"
          value={position()}
          min="0"
          onInput={(e) => setPosition(Number(e.currentTarget.value))}
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;width:100%"
        />
      </label>
      <div style="display:flex;gap:6px">
        <button class="btn btn-primary" disabled={busy() || !label().trim()} type="submit">
          {busy() ? "…" : props.existing ? "Save" : "Add"}
        </button>
        <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
      </div>
      <Show when={err()}>{(m) => <p style="color:var(--rust);grid-column:1/-1;margin:0">{m()}</p>}</Show>
    </form>
  );
};
