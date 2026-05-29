import { A, useNavigate, useParams } from "@solidjs/router";
import { For, Show, batch, createMemo, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { Check } from "~/components/Icons";
import { DateTimePicker } from "~/components/DateTimePicker";
import { RecurrenceFields, recurrenceLabel, toRecurrence, type RecurrenceFreq } from "~/components/RecurrenceFields";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { CommentsSection } from "~/components/CommentsSection";
import { memberClient, projectClient, taskClient } from "~/data/clients";
import { displayName, dueLabel, initial, isTaskClosed, memberSwatch, toAvatar } from "~/lib/derive";
import { hasRole } from "~/stores/auth";
import type { Member, Milestone, Project, Task } from "~/api/types.gen";

const MILESTONE_STATES = ["future", "current", "done"] as const;

interface DetailBundle {
  project: Project;
  houseMembers: Member[];
  members: Member[];
  owners: Member[];
  milestones: Milestone[];
  projectTasks: Task[];
  houseTasks: Task[];
}

export const ProjectDetail = () => {
  const params = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const projectId = () => params.slug;

  const [bundle, { refetch }] = createResource(projectId, async (id): Promise<DetailBundle | null> => {
    try {
      const project = await projectClient.getProject(id);
      const [houseMembers, members, owners, milestones, projectTasks, houseTasks] = await Promise.all([
        memberClient.listMembers({ houseId: project.houseId }),
        projectClient.listProjectMembers(id),
        projectClient.listProjectOwners(id),
        projectClient.listMilestones(id),
        projectClient.listProjectTasks({ houseId: project.houseId, projectId: id }),
        taskClient.listTasks({ houseId: project.houseId }),
      ]);
      return { project, houseMembers, members, owners, milestones, projectTasks, houseTasks };
    } catch {
      return null;
    }
  });

  const progress = createMemo(() => {
    const b = bundle();
    if (!b || b.projectTasks.length === 0) return 0;
    const done = b.projectTasks.filter((t) => t.status === "done").length;
    return Math.round((done / b.projectTasks.length) * 100);
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
                        <dd>{b().projectTasks.filter((t) => t.status === "done").length} of {b().projectTasks.length} done</dd>
                      </dl>
                    </aside>
                  </div>

                  <ProjectTasksSection
                    project={p()}
                    projectTasks={b().projectTasks}
                    houseTasks={b().houseTasks}
                    houseMembers={b().houseMembers}
                    canEdit={hasRole("admin", "member") || isAdmin()}
                    onChanged={async () => { await refetch(); }}
                  />

                  <MilestoneList
                    project={p()}
                    milestones={b().milestones}
                    canEdit={isAdmin()}
                    onChanged={async () => { await refetch(); }}
                  />

                  <section style="margin-top:28px">
                    <h3 style="margin:0 0 8px;font-family:var(--display);font-size:20px;color:var(--grass-4)">Discussion</h3>
                    <CommentsSection
                      targetType="project"
                      targetId={p().projectId}
                      houseId={p().houseId}
                      members={b().houseMembers}
                    />
                  </section>
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

// ─── Project tasks ────────────────────────────────────────────────────

const ProjectTasksSection = (props: {
  project: Project;
  projectTasks: Task[];
  houseTasks: Task[];
  houseMembers: Member[];
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}) => {
  const [composerOpen, setComposerOpen] = createSignal(false);
  const [attachId, setAttachId] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  // House tasks not yet attached to this project — what the "Attach
  // existing" picker offers. Excludes closed/deleted tasks.
  const attachable = createMemo(() => {
    const attached = new Set(props.projectTasks.map((t) => t.taskId));
    return props.houseTasks.filter(
      (t) => !attached.has(t.taskId) && !t.deletedAt && !isTaskClosed(t),
    );
  });

  const memberById = createMemo(() => {
    const map = new Map<string, Member>();
    for (const m of props.houseMembers) map.set(m.memberId, m);
    return map;
  });

  const attach = async () => {
    const id = attachId();
    if (!id || busy()) return;
    setBusy(true);
    try {
      await projectClient.addProjectTask({
        projectId: props.project.projectId,
        taskId: id,
        position: props.projectTasks.length,
      });
      setAttachId("");
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  return (
    <div class="section-padded" style="margin-top:24px;margin-bottom:18px">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
        <h3 style="margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)">Tasks</h3>
        <Show when={props.canEdit}>
          <button class="btn-quiet" onClick={() => setComposerOpen((v) => !v)}>
            {composerOpen() ? "Cancel" : "+ New task"}
          </button>
        </Show>
      </div>

      <Show when={composerOpen()}>
        <ProjectTaskComposer
          project={props.project}
          nextPosition={props.projectTasks.length}
          onCancel={() => setComposerOpen(false)}
          onCreated={async () => { setComposerOpen(false); await props.onChanged(); }}
        />
      </Show>

      <Show when={props.canEdit && attachable().length > 0}>
        <div style="display:flex;gap:8px;align-items:center;margin-bottom:12px">
          <span style="font-size:12px;color:var(--ink-mute)">Attach existing:</span>
          <select
            value={attachId()}
            onChange={(e) => setAttachId(e.currentTarget.value)}
            disabled={busy()}
            style="flex:1;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          >
            <option value="">— pick a task —</option>
            <For each={attachable()}>{(t) => <option value={t.taskId}>{t.title}</option>}</For>
          </select>
          <button type="button" class="btn-quiet" onClick={attach} disabled={busy() || !attachId()}>
            Attach
          </button>
        </div>
      </Show>

      <Show
        when={props.projectTasks.length > 0}
        fallback={
          <p style="color:var(--ink-mute);font-size:13px;margin:0">
            <Show when={props.canEdit} fallback={<>No tasks attached to this project yet.</>}>
              No tasks attached yet — create one, or attach an existing house task.
            </Show>
          </p>
        }
      >
        <div class="tasks" style="border:1px solid var(--line);border-radius:var(--r-md);overflow:hidden">
          <For each={props.projectTasks}>
            {(t) => (
              <ProjectTaskRow
                task={t}
                project={props.project}
                byId={memberById()}
                members={props.houseMembers}
                canEdit={props.canEdit}
                nextPosition={props.projectTasks.length}
                onChanged={props.onChanged}
              />
            )}
          </For>
        </div>
      </Show>
    </div>
  );
};

const ProjectTaskComposer = (props: {
  project: Project;
  nextPosition: number;
  onCancel: () => void;
  onCreated: () => Promise<void> | void;
}) => {
  const [title, setTitle] = createSignal("");
  const [tag, setTag] = createSignal("");
  const [due, setDue] = createSignal("");
  const [estimate, setEstimate] = createSignal("");
  const [recFreq, setRecFreq] = createSignal<RecurrenceFreq>("");
  const [recInterval, setRecInterval] = createSignal(1);
  const [recNextAt, setRecNextAt] = createSignal("");
  const [recByWeekday, setRecByWeekday] = createSignal<number[]>([]);
  const [recBySetpos, setRecBySetpos] = createSignal(1);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      // Create the task. `assignees: []` is explicit-empty so the server
      // skips its default-to-caller rule (the project should hand the
      // work out later, not auto-pin it to whoever happened to click +).
      const dueIso = due() || undefined;
      const created = await taskClient.createTask({
        houseId: props.project.houseId,
        ownerMemberId: "",
        title: title().trim(),
        tag: tag().trim() || undefined,
        dueAt: dueIso,
        estimateMinutes: estimate() ? Number(estimate()) : undefined,
        assignees: [],
        ...toRecurrence(recFreq(), recInterval(), recNextAt(), recByWeekday(), recBySetpos(), dueIso),
      } as any);
      // Then attach it to the project at the end of the list.
      await projectClient.addProjectTask({
        projectId: props.project.projectId,
        taskId: created.taskId,
        position: props.nextPosition,
      });
      batch(() => {
        setTitle(""); setTag(""); setDue(""); setEstimate("");
        setRecFreq(""); setRecInterval(1); setRecNextAt("");
        setRecByWeekday([]); setRecBySetpos(1);
      });
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
      style="margin:0 0 12px;padding:14px 16px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:2fr 1fr auto 90px auto auto;gap:10px;align-items:end"
    >
      <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
        <span style="font-size:11px;color:var(--ink-mute)">Title</span>
        <input
          type="text" value={title()} onInput={(e) => setTitle(e.currentTarget.value)}
          required autofocus
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Tag</span>
        <input
          type="text" value={tag()} onInput={(e) => setTag(e.currentTarget.value)}
          placeholder="house, barn, …"
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Due</span>
        <DateTimePicker value={due()} onChange={setDue} title="Due (optional)" />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Est (min)</span>
        <input
          type="number" min="0" value={estimate()} onInput={(e) => setEstimate(e.currentTarget.value)}
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <button class="btn btn-primary" disabled={busy() || !title().trim()} type="submit">
        {busy() ? "…" : "Add"}
      </button>
      <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
      <div style="grid-column:1/-1">
        <RecurrenceFields
          freq={recFreq}
          setFreq={setRecFreq}
          interval={recInterval}
          setInterval={setRecInterval}
          nextAt={recNextAt}
          setNextAt={setRecNextAt}
          byWeekday={recByWeekday}
          setByWeekday={setRecByWeekday}
          bySetpos={recBySetpos}
          setBySetpos={setRecBySetpos}
          compact
        />
      </div>
      <Show when={err()}>{(m) => <p style="color:var(--rust);grid-column:1/-1;margin:0;font-size:12px">{m()}</p>}</Show>
    </form>
  );
};

const ProjectTaskRow = (props: {
  task: Task;
  project: Project;
  byId: Map<string, Member>;
  members: Member[];
  canEdit: boolean;
  nextPosition: number;
  onChanged: () => Promise<unknown>;
}) => {
  const [busy, setBusy] = createSignal(false);
  const [showAssignees, setShowAssignees] = createSignal(false);
  const [showSubtask, setShowSubtask] = createSignal(false);
  const [showDetail, setShowDetail] = createSignal(false);
  const closed = () => isTaskClosed(props.task);

  const toggle = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      await taskClient.updateTask({
        ...props.task,
        status: closed() ? "open" : "done",
      } as any);
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  const detach = async () => {
    if (busy()) return;
    if (!confirm(`Remove "${props.task.title}" from this project? The task itself isn't deleted.`)) return;
    setBusy(true);
    try {
      await projectClient.removeProjectTask({
        projectId: props.project.projectId,
        taskId: props.task.taskId,
      });
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  const saveAssignees = async (next: string[]) => {
    if (busy()) return;
    setBusy(true);
    try {
      await taskClient.updateTask({
        ...props.task,
        assignees: next,
      } as any);
      await props.onChanged();
      setShowAssignees(false);
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  const assignees = () =>
    (props.task.assignees ?? [])
      .map((mid) => props.byId.get(mid))
      .filter((m): m is Member => Boolean(m));

  return (
    <div>
      <div class={`task ${closed() ? "done" : ""}`} style={busy() ? "opacity:0.5;pointer-events:none" : ""}>
        <Show
          when={props.canEdit}
          fallback={<div class="check"><Check /></div>}
        >
          <button
            class="check"
            title={closed() ? "Mark as open" : "Mark as done"}
            onClick={(ev) => { ev.stopPropagation(); toggle(); }}
            style="cursor:pointer"
          >
            <Check />
          </button>
        </Show>
        <div
          onClick={() => setShowDetail((v) => !v)}
          title="Click to view / edit"
          style="cursor:pointer;min-width:0"
        >
          <div class="t-title">{props.task.title}</div>
          <div class="t-meta">
            <Show when={props.task.tag}>
              {(tag) => <span class={`tag ${(tag() ?? "").toLowerCase()}`}>{capWord(tag()!)}</span>}
            </Show>
            <Show when={dueLabel(props.task)}>{(label) => <><span class="dot" />{label()}</>}</Show>
            <Show when={props.task.estimateMinutes !== undefined && props.task.estimateMinutes! > 0}>
              <span class="dot" />{props.task.estimateMinutes}m est.
            </Show>
            <Show when={recurrenceLabel(props.task)}>
              {(label) => <><span class="dot" />🔁 {label()}</>}
            </Show>
          </div>
        </div>
        <div class="t-end" style="display:flex;align-items:center;gap:6px">
          <Show when={props.canEdit} fallback={
            <Show when={assignees().length > 0}>
              <AvatarStack bits={assignees().map(toAvatar)} />
            </Show>
          }>
            <button
              type="button"
              onClick={() => setShowAssignees((v) => !v)}
              title="Edit assignees"
              style="background:transparent;border:0;padding:2px;cursor:pointer;border-radius:9999px"
            >
              <Show
                when={assignees().length > 0}
                fallback={<span style="font-size:11px;color:var(--ink-mute);padding:0 6px">+ assign</span>}
              >
                <AvatarStack bits={assignees().map(toAvatar)} />
              </Show>
            </button>
            <button
              type="button"
              onClick={() => setShowSubtask((v) => !v)}
              title="Add a subtask (also attached to this project)"
              style="background:transparent;border:1px dashed var(--line);color:var(--ink-mute);font-size:11px;padding:2px 8px;border-radius:9999px;cursor:pointer"
            >
              + subtask
            </button>
            <button
              type="button"
              onClick={detach}
              title="Remove from this project (task itself is kept)"
              style="background:transparent;border:0;color:var(--ink-mute);font-size:18px;line-height:1;padding:4px 8px;cursor:pointer"
            >
              ×
            </button>
          </Show>
        </div>
      </div>

      <Show when={showAssignees()}>
        <ProjectAssigneeEditor
          current={(props.task.assignees ?? []).slice()}
          members={props.members}
          onCancel={() => setShowAssignees(false)}
          onSave={saveAssignees}
        />
      </Show>

      <Show when={showSubtask()}>
        <ProjectSubtaskComposer
          parent={props.task}
          project={props.project}
          nextPosition={props.nextPosition}
          parentAssignees={(props.task.assignees ?? []).slice()}
          members={props.members}
          onCancel={() => setShowSubtask(false)}
          onCreated={async () => { setShowSubtask(false); await props.onChanged(); }}
        />
      </Show>

      <Show when={showDetail()}>
        <TaskDetailEditor
          task={props.task}
          members={props.members}
          onClose={() => setShowDetail(false)}
          onSaved={async () => { await props.onChanged(); }}
          onDelete={async () => {
            // From the project view, "delete" detaches + soft-deletes —
            // the row would otherwise stay linked to a soft-deleted task.
            await projectClient.removeProjectTask({
              projectId: props.project.projectId,
              taskId: props.task.taskId,
            });
            await taskClient.deleteTask(props.task.taskId);
            await props.onChanged();
          }}
        />
      </Show>
    </div>
  );
};

const capWord = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

// ─── Project task inline editors ──────────────────────────────────────

const ProjectAssigneeEditor = (props: {
  current: string[];
  members: Member[];
  onCancel: () => void;
  onSave: (next: string[]) => Promise<void> | void;
}) => {
  const [selected, setSelected] = createSignal<string[]>(props.current.slice());
  const [busy, setBusy] = createSignal(false);
  const toggle = (id: string) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  const save = async () => {
    if (busy()) return;
    setBusy(true);
    try { await props.onSave(selected()); }
    finally { setBusy(false); }
  };
  return (
    <div style="margin:6px 0 10px 38px;padding:10px 12px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:flex;flex-wrap:wrap;gap:6px;align-items:center">
      <For each={props.members}>
        {(m) => {
          const on = () => selected().includes(m.memberId);
          return (
            <button
              type="button"
              onClick={() => toggle(m.memberId)}
              style={`display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;border-radius:9999px;font-size:12px;cursor:pointer;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`}
            >
              <span class={`a sm ${memberSwatch(m.memberId)}`} style="width:18px;height:18px;font-size:10px">{initial(m)}</span>
              {displayName(m)}
            </button>
          );
        }}
      </For>
      <button type="button" class="btn btn-primary" onClick={save} disabled={busy()} style="margin-left:auto;padding:4px 12px">
        {busy() ? "…" : "Save"}
      </button>
      <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()} style="padding:4px 10px">
        Cancel
      </button>
    </div>
  );
};

const ProjectSubtaskComposer = (props: {
  parent: Task;
  project: Project;
  nextPosition: number;
  parentAssignees: string[];
  members: Member[];
  onCancel: () => void;
  onCreated: () => Promise<void> | void;
}) => {
  const [title, setTitle] = createSignal("");
  const [tag, setTag] = createSignal("");
  const [due, setDue] = createSignal("");
  const [estimate, setEstimate] = createSignal("");
  const [assignees, setAssignees] = createSignal<string[] | null>(null);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const effective = () => assignees() ?? props.parentAssignees;
  const toggleA = (id: string) =>
    setAssignees((prev) => {
      const base = prev ?? props.parentAssignees;
      return base.includes(id) ? base.filter((x) => x !== id) : [...base, id];
    });

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      const body: any = {
        houseId: props.parent.houseId,
        ownerMemberId: "",
        title: title().trim(),
        tag: tag().trim() || undefined,
        dueAt: due() || undefined,
        estimateMinutes: estimate() ? Number(estimate()) : undefined,
        parentTaskId: props.parent.taskId,
      };
      if (assignees() !== null) body.assignees = assignees();
      const created = await taskClient.createTask(body);
      // Also attach the new subtask to the same project so it shows up
      // here. The api keeps subtasks and project-tasks as independent
      // relations; from the user's POV "+ subtask on a project task"
      // means "in this project," so we wire it explicitly.
      await projectClient.addProjectTask({
        projectId: props.project.projectId,
        taskId: created.taskId,
        position: props.nextPosition,
      });
      await props.onCreated();
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : String(e2)); }
    finally { setBusy(false); }
  };

  return (
    <form
      onSubmit={submit}
      style="margin:6px 0 10px 38px;padding:12px 14px;background:color-mix(in oklab, var(--paper) 92%, var(--grass-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:flex;flex-direction:column;gap:8px"
    >
      <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:center">
        <input
          type="text" placeholder="Subtask title…"
          value={title()} onInput={(e) => setTitle(e.currentTarget.value)}
          required autofocus
          style="flex:2 1 260px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
        <input
          type="text" placeholder="tag" value={tag()} onInput={(e) => setTag(e.currentTarget.value)}
          style="flex:0 1 120px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
        <DateTimePicker value={due()} onChange={setDue} title="Due (optional)" />
        <input
          type="number" min="0" placeholder="est min" value={estimate()} onInput={(e) => setEstimate(e.currentTarget.value)}
          style="width:88px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </div>
      <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
        <span style="font-size:11px;color:var(--ink-mute)">Assignees</span>
        <For each={props.members}>
          {(m) => {
            const on = () => effective().includes(m.memberId);
            return (
              <button
                type="button"
                onClick={() => toggleA(m.memberId)}
                style={`display:inline-flex;align-items:center;gap:4px;padding:2px 8px 2px 2px;border-radius:9999px;font-size:11px;cursor:pointer;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`}
              >
                <span class={`a sm ${memberSwatch(m.memberId)}`} style="width:16px;height:16px;font-size:9px">{initial(m)}</span>
                {displayName(m)}
              </button>
            );
          }}
        </For>
        <Show when={assignees() === null}>
          <span style="font-size:11px;color:var(--ink-mute);font-style:italic">(inherits from parent)</span>
        </Show>
      </div>
      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>
      <div style="display:flex;gap:8px;justify-content:flex-end">
        <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()} style="padding:4px 12px">Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={busy() || !title().trim()} style="padding:4px 12px">
          {busy() ? "…" : "Add subtask"}
        </button>
      </div>
    </form>
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
  // The wrapper inherits .room-detail's `overflow:hidden + border-radius`,
  // so children with no horizontal padding visually meet the rounded
  // corners. Match .room-meta's 32px gutters (22px under 820px) so the
  // section's heading, add button, and empty state all sit inside the
  // rounded shape instead of getting clipped on either end.
  return (
    <div class="section-padded" style="margin-top:4px;padding-bottom:24px">
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
