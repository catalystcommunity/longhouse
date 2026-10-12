import { For, Show, createEffect, createMemo, createResource, createSignal } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import { HeroScene } from "~/components/HeroScene";
import { Check, Pin, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { DayStrip } from "~/components/DayStrip";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { TaskFeedList } from "~/components/TaskFeedList";
import { eventClient, memberClient, projectClient, taskClient } from "~/data/clients";
import {
  dashboardKicker,
  displayName,
  dueLabel,
  eventTone,
  initial,
  isTaskClosed,
  memberHandle,
  memberStatus,
  memberSwatch,
  partOfDayGreeting,
  taskGroup,
  timeLabel,
  toAvatar,
} from "~/lib/derive";
import { PROJECT_LIST_LIMIT, loadMyProjectIds, loadProjectTasks, openTaskCount } from "~/lib/projects";
import { createTaskFeed, dueBoundaries, dueGroupStarts } from "~/lib/taskFeed";
import { currentMemberId, useCurrentHouseId, useSession } from "~/stores/auth";
import type { Member, Project, Task, TaskListRequest } from "@longhouse/client";

/** Rows shown in the "Your projects" card before the "All projects" link. */
const MY_PROJECTS_CAP = 6;

// Group labels in due order: the order of the list and of dueBoundaries().
const DASHBOARD_GROUP_LABELS = ["Overdue", "Today", "This week", "Later", "No due date"];

export const Dashboard = () => {
  const houseId = useCurrentHouseId();
  const session = useSession();
  const navigate = useNavigate();

  // Open the calendar's day view at a given date, optionally jumping the
  // user straight into the event editor. The Calendar page reads these
  // query params on mount.
  const openInCalendar = (dateLocal: Date, eventId?: string) => {
    const y = dateLocal.getFullYear();
    const m = String(dateLocal.getMonth() + 1).padStart(2, "0");
    const d = String(dateLocal.getDate()).padStart(2, "0");
    const qs = new URLSearchParams({ view: "day", date: `${y}-${m}-${d}` });
    if (eventId) qs.set("event", eventId);
    navigate(`/calendar?${qs.toString()}`);
  };

  const list = (r: TaskListRequest) => taskClient.listTasks(r);
  // The Tasks card: my open tasks that are in no project, paged by the
  // server in due-date order. Project work is summarized per project in the
  // "Your projects" card instead.
  const miscFeed = createTaskFeed();
  createEffect(() => {
    const h = houseId();
    if (!h) {
      miscFeed.reset(null);
      return;
    }
    const bounds = dueBoundaries();
    miscFeed.reset((offset, limit, withCounts) =>
      list({ houseId: h, state: "open", mine: true, loose: true, offset, limit, ...(withCounts ? { dueBefore: bounds } : {}) }),
    );
  });
  // Recently completed: the newest five, and the done count from the total.
  const [recent, { refetch: refetchRecent }] = createResource(
    () => houseId(),
    (h) => list({ houseId: h, state: "closed", mine: true, loose: true, limit: 5 }),
  );
  // Hero "due today" counts ALL my open tasks due today, project work
  // included — the hero is the whole-day summary, not the Tasks card.
  const [dueToday, { refetch: refetchDueToday }] = createResource(
    () => houseId(),
    (h) => {
      const from = new Date();
      from.setHours(0, 0, 0, 0);
      const to = new Date(from);
      to.setDate(to.getDate() + 1);
      return list({ houseId: h, state: "open", mine: true, dueFrom: from.toISOString(), dueTo: to.toISOString(), limit: 1 });
    },
  );
  const allDueToday = () => (dueToday.state === "ready" ? Number(dueToday().total ?? dueToday().tasks.length) : 0);
  const tasksRecent = () => (recent.state === "ready" ? recent().tasks : []);
  const doneCount = () => (recent.state === "ready" ? Number(recent().total ?? recent().tasks.length) : 0);

  const [members] = createResource(
    () => houseId(),
    async (h) => memberClient.listMembers({ houseId: h }),
  );
  const [events] = createResource(
    () => houseId(),
    async (h) => eventClient.listEvents({ houseId: h }),
  );
  const [projects, { refetch: refetchProjects }] = createResource(
    () => houseId(),
    async (h) => (await projectClient.listProjects({ houseId: h, limit: PROJECT_LIST_LIMIT })).projects,
  );

  // Which projects are mine (owner via created_by, explicit owner, or
  // member), and the task list of each, for the open counts. Tolerant of
  // per-project failures (see lib/projects.ts).
  const [projectIndex] = createResource(
    () => {
      const h = houseId();
      const me = currentMemberId();
      const ps = projects();
      if (!h || !me || !ps) return null;
      return { h, me, projects: ps };
    },
    async ({ h, me, projects: ps }) => {
      const mine = await loadMyProjectIds(projectClient, ps, me);
      const tasksByProject = await loadProjectTasks(projectClient, h, ps.filter((p) => mine.has(p.projectId)));
      return { mine, tasksByProject };
    },
  );
  // The index as last loaded. Uses `latest` so a refetch keeps the card
  // populated instead of flashing, and never throws on an errored load.
  const index = () =>
    projectIndex.state === "ready" || projectIndex.state === "refreshing"
      ? projectIndex.latest
      : undefined;
  // An errored load (or no project list to fan out over) counts as done.
  const indexSettled = () =>
    !!index() ||
    projectIndex.state === "errored" ||
    projects.state === "errored" ||
    (projects.state === "ready" && !currentMemberId());

  // Projects I own or belong to, in list order, with their open task count.
  const myProjects = createMemo((): { project: Project; open: number }[] => {
    const idx = index();
    if (!idx) return [];
    return (projects() ?? [])
      .filter((p) => idx.mine.has(p.projectId))
      .map((p) => ({
        project: p,
        open: openTaskCount(idx.tasksByProject.get(p.projectId)?.tasks ?? []),
      }));
  });

  // A task change can move it between the cards (into a project, closed,
  // due today), so a row's mutate hook refreshes every source. A new
  // project list (a Personal project made on first use) re-runs the
  // project index through its source.
  const refresh = async () => {
    await Promise.all([miscFeed.refresh(), refetchRecent(), refetchDueToday(), refetchProjects()]);
  };

  const memberById = createMemo(() => {
    const map = new Map<string, Member>();
    for (const m of members() ?? []) map.set(m.memberId, m);
    return map;
  });

  const miscGroups = createMemo(() => dueGroupStarts(miscFeed.dueCounts(), DASHBOARD_GROUP_LABELS));
  const miscFirstLoad = () => !miscFeed.ready();

  const activeMembers = createMemo(
    () => (members() ?? []).filter((m) => memberStatus(m) === "active").length,
  );

  // Upcoming events only — anything whose end (or start, if no end) is still
  // in the future. Past events never surface in dashboard lists/counts; they
  // remain findable on the calendar.
  const upcomingEvents = createMemo(() => {
    const now = Date.now();
    return (events() ?? []).filter((e) => {
      const ref = e.endsAt ?? e.startsAt;
      return ref ? Date.parse(ref) >= now : false;
    });
  });

  // Next event = the earliest upcoming event. Falls back to undefined when
  // there are no future events.
  const nextEvent = createMemo(() => {
    const futures = [...upcomingEvents()];
    futures.sort((a, b) => {
      const ka = Date.parse(a.startsAt ?? a.endsAt ?? "");
      const kb = Date.parse(b.startsAt ?? b.endsAt ?? "");
      return ka - kb;
    });
    return futures[0];
  });

  // The estimate needs every open row, so it shows once the list is loaded.
  const footHours = createMemo(() => {
    if (!miscFeed.allLoaded()) return 0;
    return miscFeed.loadedRows().reduce((acc, t) => acc + (t.estimateMinutes ?? 0), 0) / 60;
  });

  const [composerOpen, setComposerOpen] = createSignal(false);

  const greetingName = () => session()?.displayName?.split(/\s+/)[0] ?? "there";

  return (
    <AuthGate>
      <section class="hero reveal d1">
        <HeroScene />
        <div class="hero-content">
          <span class="kicker">{dashboardKicker()}</span>
          <h1 class="greet">
            {partOfDayGreeting()},<br />
            {greetingName()}.
          </h1>
          <p class="greet-sub">
            You have <b>{allDueToday()} {allDueToday() === 1 ? "task" : "tasks"} due today</b>{" "}
            and {nextEventCopy(upcomingEvents().length)}.{" "}
            {activeMembers()} {activeMembers() === 1 ? "member is" : "members are"} active right now.
          </p>
          <div class="hero-stats">
            <Stat n={allDueToday()} label="tasks due" onClick={() => navigate("/tasks")} />
            <Stat n={upcomingEvents().length} label="upcoming events" onClick={() => navigate("/calendar")} />
            <Stat n={activeMembers()} label="active members" onClick={() => navigate("/members")} />
            <Stat n={(projects() ?? []).length} label="projects" onClick={() => navigate("/projects")} />
          </div>
        </div>
      </section>

      <div class="grid">
        <section class="card reveal d2" data-area="tasks" aria-label="Tasks">
          <div class="card-hd">
            <div>
              <h2>Tasks</h2>
              <div class="sub">Yours, not in a project.</div>
            </div>
          </div>

          <Show when={!miscFirstLoad()} fallback={<p style="padding:18px;color:var(--ink-mute)">Loading…</p>}>
            <div class="tasks">
              <Show when={miscFeed.estimatedTotal() > 0} fallback={<EmptyTasks />}>
                <TaskFeedList
                  feed={miscFeed}
                  groups={miscGroups()}
                  renderTask={(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={refresh} />}
                />
              </Show>
              {/* Shown even with no open tasks: the latest finished work is
                  still worth seeing. */}
              <Show when={tasksRecent().length > 0}>
                <div class="group-lbl" style="margin-top:18px;color:var(--ink-mute)">
                  Recently completed
                </div>
                <For each={tasksRecent()}>
                  {(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={refresh} />}
                </For>
              </Show>
            </div>
          </Show>

          <Show when={composerOpen()}>
            <PersonalTaskComposer
              houseId={houseId()!}
              onCancel={() => setComposerOpen(false)}
              onCreated={async () => {
                setComposerOpen(false);
                await refresh();
              }}
            />
          </Show>

          <div class="tasks-foot">
            <span>
              {doneCount()} done · {miscFeed.estimatedTotal()} remaining
              <Show when={footHours() > 0}>
                {" "}— estimated{" "}
                <b style="color:var(--grass-4);font-weight:600">
                  {footHours().toFixed(1)} hours
                </b>
              </Show>
            </span>
            <button class="btn-quiet" type="button" onClick={() => setComposerOpen((v) => !v)}>
              <Plus /> New task
            </button>
          </div>
        </section>

        <aside class="side">
          <section class="card myprojects reveal d3" aria-label="Your projects">
            <div class="card-hd" style="padding:0 0 12px;border:0">
              <div>
                <h2>Your projects</h2>
                <div class="sub">Projects you own or belong to.</div>
              </div>
            </div>
            <Show when={indexSettled()} fallback={<p class="proj-empty">Loading…</p>}>
              <Show
                when={myProjects().length > 0}
                fallback={
                  <p class="proj-empty">
                    You are not in a project yet. <A href="/projects">See all projects</A>
                  </p>
                }
              >
                <ul class="proj-list">
                  <For each={myProjects().slice(0, MY_PROJECTS_CAP)}>
                    {(row) => (
                      <li>
                        <A href={`/projects/${row.project.projectId}`} class="proj-row">
                          <span class="proj-name">
                            {row.project.name}
                            <Show when={row.project.personal}>
                              <span style="margin-left:6px;font-size:11px;color:var(--ink-mute)">private</span>
                            </Show>
                          </span>
                          <span class="proj-count">
                            {row.open} open {row.open === 1 ? "task" : "tasks"}
                          </span>
                        </A>
                      </li>
                    )}
                  </For>
                </ul>
                <div class="proj-foot">
                  <A href="/projects">
                    All projects
                    <Show when={myProjects().length > MY_PROJECTS_CAP}>
                      {` (${myProjects().length - MY_PROJECTS_CAP} more of yours)`}
                    </Show>
                  </A>
                </div>
              </Show>
            </Show>
          </section>

          <section class="card gather reveal d4">
            <div class="card-hd" style="padding:0 0 12px;border:0">
              <div>
                <h2>Next event</h2>
                <div class="sub">
                  <Show when={nextEvent()} fallback="—">
                    {(e) => <>{relativeWhen(e().startsAt)}<Show when={e().location}>{(loc) => <> · {loc()}</>}</Show></>}
                  </Show>
                </div>
              </div>
              <Show when={nextEvent()}>
                {(e) => (
                  <span
                    class="kicker plain"
                    data-tone={eventTone(e().ownerMemberId)}
                  >
                    {timeLabel(e())}
                  </span>
                )}
              </Show>
            </div>
            <Show when={nextEvent()} fallback={<p style="color:var(--ink-mute)">No upcoming events.</p>}>
              {(e) => {
                const day = () => new Date(e().startsAt!);
                return (
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => openInCalendar(day(), e().eventId)}
                    onKeyDown={(ev) => {
                      if (ev.key === "Enter" || ev.key === " ") {
                        ev.preventDefault();
                        openInCalendar(day(), e().eventId);
                      }
                    }}
                    style="cursor:pointer"
                    aria-label={`Open ${e().title} in the calendar`}
                  >
                    <DayStrip
                      date={day()}
                      events={events() ?? []}
                      onPick={(ev) => openInCalendar(new Date(ev.startsAt!), ev.eventId)}
                    />
                    <div style="margin-top:8px;font-size:13px;color:var(--ink-mute)">
                      <b style="color:var(--ink)">{e().title}</b>
                      <Show when={e().location}>{(loc) => <> · <span style="display:inline-flex;align-items:center;gap:2px"><Pin /> {loc()}</span></>}</Show>
                    </div>
                  </div>
                );
              }}
            </Show>
          </section>

          <section class="card folks reveal d5">
            <div class="card-hd" style="padding:0 0 12px;border:0">
              <div>
                <h2>Active members</h2>
                <div class="sub">Currently online or recently active</div>
              </div>
            </div>
            <Show
              when={(members() ?? []).filter((m) => m.memberId !== ownMemberId()).length > 0}
              fallback={<p style="color:var(--ink-mute);padding:8px 0">Just you here so far.</p>}
            >
              <For each={(members() ?? []).filter((m) => m.memberId !== ownMemberId())}>
                {(m) => (
                  <div class={`folk ${memberStatus(m) === "away" ? "away" : ""}`}>
                    <span class={`a lg ${memberSwatch(m.memberId)}`}>{initial(m)}</span>
                    <div>
                      <div class="who-name">{displayName(m)}</div>
                      <Show when={memberHandle(m)}>
                        {(h) => <div class="doing">{h()}</div>}
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </Show>
          </section>
        </aside>
      </div>
    </AuthGate>
  );
};

// ownMemberId pulls the caller's member id (in the current house) out of the
// auth store. Used so the "active members" panel doesn't echo the caller
// back to themselves. Sourced from /me's HouseSummary now that it carries
// the per-house member_id.
const ownMemberId = (): string | undefined => currentMemberId() ?? undefined;

const EmptyTasks = () => (
  <div style="padding:24px;color:var(--ink-mute);text-align:center">
    <p style="margin:0"><b>No open tasks.</b></p>
  </div>
);

const Stat = (props: { n: number; label: string; onClick?: () => void }) => (
  <button
    type="button"
    class="stat"
    onClick={props.onClick}
    aria-label={`${props.n} ${props.label} — open`}
  >
    <span class="n">{props.n}</span>
    <span class="lbl">{props.label}</span>
  </button>
);

const TaskRow = (props: {
  task: Task;
  byId: Map<string, Member>;
  members: Member[];
  onMutate: () => Promise<unknown>;
}) => {
  const [busy, setBusy] = createSignal(false);
  const [showDetail, setShowDetail] = createSignal(false);
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

  const assignees = () =>
    (props.task.assignees ?? [])
      .map((mid) => props.byId.get(mid))
      .filter((m): m is Member => Boolean(m))
      .map(toAvatar);

  return (
    <div>
      <div class={`task ${closed() ? "done" : ""}`} style={busy() ? "opacity:0.5;pointer-events:none" : ""}>
        <button
          class="check"
          title={closed() ? "Mark as open" : "Mark as done"}
          onClick={(ev) => { ev.stopPropagation(); toggleDone(); }}
          style="cursor:pointer"
        >
          <Check />
        </button>
        <div
          onClick={() => setShowDetail((v) => !v)}
          title="Click to view / edit"
          style="cursor:pointer;min-width:0"
        >
          <div class="t-title">{props.task.title}</div>
          <div class="t-meta">
            <Show when={props.task.tag}>
              {(tag) => <span class={`tag ${(tag() ?? "").toLowerCase()}`}>{cap(tag()!)}</span>}
            </Show>
            <Show when={dueLabel(props.task)}>
              {(label) => <><span class="dot" />{label()}</>}
            </Show>
          </div>
        </div>
        <div class="t-end">
          <Show when={assignees().length > 0}>
            <AvatarStack bits={assignees()} />
          </Show>
        </div>
      </div>

      <Show when={showDetail()}>
        <TaskDetailEditor
          task={props.task}
          members={props.members}
          onClose={() => setShowDetail(false)}
          onSaved={async () => { await props.onMutate(); }}
          onDelete={async () => { await taskClient.deleteTask(props.task.taskId); await props.onMutate(); }}
        />
      </Show>
    </div>
  );
};

const nextEventCopy = (n: number): string => {
  if (n === 0) return "nothing on the calendar yet";
  if (n === 1) return "one event on the calendar";
  return `${n} events on the calendar`;
};

const relativeWhen = (iso?: string): string => {
  if (!iso) return "—";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  const d = new Date(t);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) return "Today";
  const tomorrow = new Date(now); tomorrow.setDate(now.getDate() + 1);
  if (d.toDateString() === tomorrow.toDateString()) return "Tomorrow";
  return d.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" });
};

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

/**
 * The dashboard's quick "New task": a private task for the caller. A loose
 * task cannot be private (docs/rbac.md), so the server puts it in the
 * caller's private "Personal" project, made on first use.
 */
const PersonalTaskComposer = (props: { houseId: string; onCancel: () => void; onCreated: () => Promise<void> }) => {
  const [title, setTitle] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);
  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    const t = title().trim();
    if (!t || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await taskClient.createPersonalTask({
        taskId: "",
        houseId: props.houseId,
        ownerMemberId: "",
        title: t,
        assignees: [],
        createdAt: "",
        updatedAt: "",
      } as unknown as Task);
      setTitle("");
      await props.onCreated();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form onSubmit={submit} style="display:flex;flex-direction:column;gap:6px;padding:12px 18px;border-top:1px dashed var(--line)">
      <div style="display:flex;gap:8px;align-items:center">
        <input
          type="text"
          value={title()}
          onInput={(e) => setTitle(e.currentTarget.value)}
          placeholder="What needs doing?"
          aria-label="New private task"
          autofocus
          style="flex:1;min-width:0;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);color:var(--ink);font-size:14px"
        />
        <button class="btn btn-primary" type="submit" disabled={busy() || !title().trim()}>
          {busy() ? "Adding…" : "Add"}
        </button>
        <button class="btn btn-ghost" type="button" onClick={() => props.onCancel()}>
          Cancel
        </button>
      </div>
      <span style="font-size:12px;color:var(--ink-mute)">
        Private to you. It goes in your Personal project.
      </span>
      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>
    </form>
  );
};
