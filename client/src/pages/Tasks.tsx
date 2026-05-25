import { For, Show, batch, createMemo, createResource, createSignal } from "solid-js";
import { Check, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { memberClient, taskClient } from "~/data/clients";
import { displayName, dueLabel, initial, isTaskClosed, memberSwatch, taskGroup, toAvatar } from "~/lib/derive";
import { currentMemberId, useCurrentHouseId } from "~/stores/auth";
import type { Member, Task } from "~/api/types.gen";

export const TasksPage = () => {
  const houseId = useCurrentHouseId();
  const [tasks, { refetch: refetchTasks }] = createResource(
    () => houseId(),
    async (h) => taskClient.listTasks({ houseId: h }),
  );
  const [members] = createResource(
    () => houseId(),
    async (h) => memberClient.listMembers({ houseId: h }),
  );

  const memberById = createMemo(() => {
    const map = new Map<string, Member>();
    for (const m of members() ?? []) map.set(m.memberId, m);
    return map;
  });

  const grouped = createMemo(() => {
    const ts = (tasks() ?? []).filter((t) => !t.deletedAt);
    return {
      overdue: ts.filter((t) => taskGroup(t) === "overdue"),
      today:   ts.filter((t) => taskGroup(t) === "today"),
      week:    ts.filter((t) => taskGroup(t) === "week"),
      later:   ts.filter((t) => taskGroup(t) === "later"),
      noDate:  ts.filter((t) => taskGroup(t) === "noDate"),
    };
  });

  const openCount  = createMemo(() => (tasks() ?? []).filter((t) => !isTaskClosed(t) && !t.deletedAt).length);
  const totalCount = createMemo(() => (tasks() ?? []).filter((t) => !t.deletedAt).length);

  const [composerOpen, setComposerOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Tasks <em>everything on the slate</em></h2>
        <p class="lead">Everything assigned to you, watched by you, or shared with you.</p>
      </div>

      <section class="card reveal d1" style="margin-top: 0">
        <div class="card-hd">
          <div>
            <h2>Open tasks</h2>
            <div class="sub">
              {tasks() ? `${openCount()} open · ${totalCount()} total` : "…"}
            </div>
          </div>
          <button class="btn-quiet" onClick={() => setComposerOpen((v) => !v)}>
            <Plus /> New task
          </button>
        </div>

        <Show when={composerOpen()}>
          <TaskComposer
            houseId={houseId()!}
            members={members() ?? []}
            tasks={(tasks() ?? []).filter((t) => !t.deletedAt)}
            onCancel={() => setComposerOpen(false)}
            onCreated={async () => {
              setComposerOpen(false);
              await refetchTasks();
            }}
          />
        </Show>

        <Show
          when={!tasks.loading}
          fallback={<p style="padding:20px;color:var(--ink-mute)">Loading tasks…</p>}
        >
          <Show when={totalCount() > 0} fallback={<EmptyTasks onCreate={() => setComposerOpen(true)} />}>
            <div class="tasks">
              <GroupSection label="Overdue" items={grouped().overdue} byId={memberById()} onMutate={async () => { await refetchTasks(); }} />
              <GroupSection label="Today" items={grouped().today} byId={memberById()} onMutate={async () => { await refetchTasks(); }} />
              <GroupSection label="This week" items={grouped().week} byId={memberById()} onMutate={async () => { await refetchTasks(); }} />
              <GroupSection label="Later" items={grouped().later} byId={memberById()} onMutate={async () => { await refetchTasks(); }} />
              <GroupSection label="No date" items={grouped().noDate} byId={memberById()} onMutate={async () => { await refetchTasks(); }} />
            </div>
          </Show>
        </Show>
      </section>
    </AuthGate>
  );
};

// ─── Composer ─────────────────────────────────────────────────────────

const TaskComposer = (props: {
  houseId: string;
  members: Member[];
  tasks: Task[];
  onCancel: () => void;
  onCreated: () => Promise<void> | void;
}) => {
  const [title, setTitle] = createSignal("");
  const [tag, setTag] = createSignal("");
  const [due, setDue] = createSignal("");
  const [estimate, setEstimate] = createSignal("");
  const [parentId, setParentId] = createSignal("");
  // assignees is undefined while the user hasn't changed the default
  // ("inherit caller / parent on the server"). Toggling any chip flips
  // to a concrete array we send on the wire.
  const [assignees, setAssignees] = createSignal<string[] | null>(null);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  // Default chip set for the picker. When the parent picker is set, show
  // the parent's current assignees as the implicit default so the user
  // can opt into "same as parent" by leaving them untouched. Otherwise
  // show the caller as the implicit default. The chip toggle materializes
  // those into the concrete list on first interaction.
  const defaultAssigneeIds = createMemo(() => {
    const pid = parentId();
    if (pid) {
      const parent = props.tasks.find((t) => t.taskId === pid);
      return parent?.assignees ?? [];
    }
    const me = currentMemberId();
    return me ? [me] : [];
  });

  // The selected list shown to the user: explicit selection if any,
  // otherwise the implicit default.
  const effectiveAssignees = () => assignees() ?? defaultAssigneeIds();

  const toggleAssignee = (id: string) => {
    setAssignees((prev) => {
      const base = prev ?? defaultAssigneeIds();
      const next = base.includes(id) ? base.filter((x) => x !== id) : [...base, id];
      return next;
    });
  };

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      // If the user never touched the chip set, omit assignees so the
      // server applies its default-assignee policy (caller for top-level,
      // parent's for subtasks). If they touched it, send the explicit list
      // — including the empty list for "nobody yet."
      const body: any = {
        houseId: props.houseId,
        ownerMemberId: "",
        title: title().trim(),
        tag: tag().trim() || undefined,
        dueAt: due() ? new Date(due()).toISOString() : undefined,
        estimateMinutes: estimate() ? Number(estimate()) : undefined,
      };
      if (parentId()) body.parentTaskId = parentId();
      if (assignees() !== null) body.assignees = assignees();
      await taskClient.createTask(body);
      batch(() => {
        setTitle("");
        setTag("");
        setDue("");
        setEstimate("");
        setParentId("");
        setAssignees(null);
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
      style="display:flex;flex-direction:column;gap:10px;padding:14px 18px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
    >
      <div style="display:flex;flex-wrap:wrap;gap:10px;align-items:center">
        <input
          type="text"
          placeholder="What needs doing?"
          value={title()}
          onInput={(e) => setTitle(e.currentTarget.value)}
          style="flex:2 1 320px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          autofocus
          required
        />
        <input
          type="text"
          placeholder="tag (house, barn, …)"
          value={tag()}
          onInput={(e) => setTag(e.currentTarget.value)}
          style="flex:1 1 140px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
        <input
          type="datetime-local"
          value={due()}
          onInput={(e) => setDue(e.currentTarget.value)}
          title="Due (optional)"
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
        <input
          type="number"
          placeholder="est (min)"
          min="0"
          value={estimate()}
          onInput={(e) => setEstimate(e.currentTarget.value)}
          style="width:96px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </div>

      <div style="display:flex;flex-wrap:wrap;gap:12px;align-items:center">
        <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
          Subtask of
          <select
            value={parentId()}
            onChange={(e) => {
              setParentId(e.currentTarget.value);
              // resetting parent → fall back to the implicit default again
              setAssignees(null);
            }}
            style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          >
            <option value="">— top-level —</option>
            <For each={props.tasks.filter((t) => !isTaskClosed(t))}>
              {(t) => <option value={t.taskId}>{t.title}</option>}
            </For>
          </select>
        </label>

        <div style="display:flex;flex-wrap:wrap;align-items:center;gap:6px">
          <span style="font-size:12px;color:var(--ink-mute)">Assignees</span>
          <For each={props.members}>
            {(m) => {
              const selected = () => effectiveAssignees().includes(m.memberId);
              return (
                <button
                  type="button"
                  onClick={() => toggleAssignee(m.memberId)}
                  style={`display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;border-radius:9999px;font-size:12px;cursor:pointer;border:1px solid ${selected() ? "var(--grass-4)" : "var(--line)"};background:${selected() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`}
                >
                  <span class={`a sm ${memberSwatch(m.memberId)}`} style="width:18px;height:18px;font-size:10px">{initial(m)}</span>
                  {displayName(m)}
                </button>
              );
            }}
          </For>
          <Show when={assignees() === null}>
            <span style="font-size:11px;color:var(--ink-mute);font-style:italic">
              ({parentId() ? "inherits from parent" : "defaults to you"})
            </span>
          </Show>
        </div>
      </div>

      <Show when={err()}>
        {(m) => <span style="color:var(--rust);font-size:13px">{m()}</span>}
      </Show>

      <div style="display:flex;gap:10px;justify-content:flex-end">
        <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>
          Cancel
        </button>
        <button class="btn btn-primary" disabled={busy() || !title().trim()} type="submit">
          {busy() ? "Saving…" : "Add"}
        </button>
      </div>
    </form>
  );
};

const EmptyTasks = (props: { onCreate: () => void }) => (
  <div style="padding:32px;color:var(--ink-mute);text-align:center">
    <p style="margin:0 0 8px"><b>No tasks yet.</b></p>
    <p style="margin:0 0 16px">Create your first task to get the slate started.</p>
    <button class="btn btn-primary" onClick={props.onCreate}><Plus /> New task</button>
  </div>
);

// ─── Group + Row ──────────────────────────────────────────────────────

const GroupSection = (props: {
  label: string;
  items: Task[];
  byId: Map<string, Member>;
  onMutate: () => Promise<unknown>;
}) => (
  <Show when={props.items.length > 0}>
    <div class="group-lbl">{props.label}</div>
    <For each={props.items}>
      {(t) => <Row task={t} byId={props.byId} onMutate={props.onMutate} />}
    </For>
  </Show>
);

const Row = (props: {
  task: Task;
  byId: Map<string, Member>;
  onMutate: () => Promise<unknown>;
}) => {
  const [busy, setBusy] = createSignal(false);
  const closed = () => isTaskClosed(props.task);

  const toggleDone = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      await taskClient.updateTask({
        ...props.task,
        status: closed() ? "open" : "done",
      } as any);
      await props.onMutate();
    } finally {
      setBusy(false);
    }
  };

  const remove = async (e: MouseEvent) => {
    e.stopPropagation();
    if (busy()) return;
    if (!confirm(`Delete "${props.task.title}"?`)) return;
    setBusy(true);
    try {
      await taskClient.deleteTask(props.task.taskId);
      await props.onMutate();
    } finally {
      setBusy(false);
    }
  };

  const assignees = () =>
    (props.task.assignees ?? [])
      .map((mid) => props.byId.get(mid))
      .filter((m): m is Member => Boolean(m))
      .map(toAvatar);

  return (
    <div class={`task ${closed() ? "done" : ""}`} style={busy() ? "opacity:0.5;pointer-events:none" : ""}>
      <button
        class="check"
        title={closed() ? "Mark as open" : "Mark as done"}
        onClick={toggleDone}
        style="background:transparent;border:0;padding:0;cursor:pointer"
      >
        <Check />
      </button>
      <div>
        <div class="t-title">{props.task.title}</div>
        <div class="t-meta">
          <Show when={props.task.tag}>
            {(tag) => <span class={`tag ${(tag() ?? "").toLowerCase()}`}>{cap(tag()!)}</span>}
          </Show>
          <Show when={dueLabel(props.task)}>
            {(label) => <><span class="dot" />{label()}</>}
          </Show>
          <Show when={props.task.estimateMinutes !== undefined && props.task.estimateMinutes! > 0}>
            <span class="dot" />
            {props.task.estimateMinutes}m est.
          </Show>
        </div>
      </div>
      <div class="t-end" style="display:flex;align-items:center;gap:10px">
        <Show
          when={assignees().length > 0}
          fallback={
            <Show when={ownerMember(props.task, props.byId)}>
              {(m) => (
                <span class={`a sm ${memberSwatch(m().memberId)}`} title={`owner: ${displayName(m())}`}>
                  {initial(m())}
                </span>
              )}
            </Show>
          }
        >
          <AvatarStack bits={assignees()} />
        </Show>
        <button
          aria-label="Delete task"
          title="Delete task"
          onClick={remove}
          style="background:transparent;border:0;color:var(--ink-mute);font-size:18px;line-height:1;padding:4px 8px;cursor:pointer"
        >
          ×
        </button>
      </div>
    </div>
  );
};

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

const ownerMember = (t: Task, byId: Map<string, Member>): Member | undefined => {
  if (!t.ownerMemberId) return undefined;
  return byId.get(t.ownerMemberId);
};
