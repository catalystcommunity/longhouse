import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { HeroScene } from "~/components/HeroScene";
import { Check, Pin, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { DayStrip } from "~/components/DayStrip";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { eventClient, memberClient, projectClient, taskClient } from "~/data/clients";
import {
  dashboardKicker,
  displayName,
  dueLabel,
  eventTone,
  initial,
  isTaskClosed,
  memberStatus,
  memberSwatch,
  partOfDayGreeting,
  taskGroup,
  timeLabel,
  toAvatar,
} from "~/lib/derive";
import { currentMemberId, useCurrentHouseId, useSession } from "~/stores/auth";
import type { Member, Task } from "@longhouse/client";

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

  const [tasks, { refetch: refetchTasks }] = createResource(
    () => houseId(),
    async (h) => (await taskClient.listTasks({ houseId: h })).tasks,
  );
  const [members] = createResource(
    () => houseId(),
    async (h) => memberClient.listMembers({ houseId: h }),
  );
  const [events] = createResource(
    () => houseId(),
    async (h) => eventClient.listEvents({ houseId: h }),
  );
  const [projects] = createResource(
    () => houseId(),
    async (h) => (await projectClient.listProjects({ houseId: h })).projects,
  );

  const memberById = createMemo(() => {
    const map = new Map<string, Member>();
    for (const m of members() ?? []) map.set(m.memberId, m);
    return map;
  });

  const tasksOpen = createMemo(() => (tasks() ?? []).filter((t) => !t.deletedAt && !isTaskClosed(t)));
  // Dashboard task buckets, in display order. Overdue and today come first
  // (action items), then anything due this week, then undated work the
  // member should still see ("things I just need to get done"), then later
  // work last so the dashboard doesn't hide it but also doesn't shout it.
  const tasksOverdue = createMemo(() => tasksOpen().filter((t) => taskGroup(t) === "overdue"));
  const tasksToday   = createMemo(() => tasksOpen().filter((t) => taskGroup(t) === "today"));
  const tasksWeek    = createMemo(() => tasksOpen().filter((t) => taskGroup(t) === "week"));
  const tasksNoDate  = createMemo(() => tasksOpen().filter((t) => taskGroup(t) === "noDate"));
  const tasksLater   = createMemo(() => tasksOpen().filter((t) => taskGroup(t) === "later"));
  // Recently completed — newest 5 by updated_at, so the dashboard always
  // shows what was just finished. status === "done" only (cancelled gets
  // its own bucket if we ever surface one).
  const tasksRecent  = createMemo(() => (tasks() ?? [])
    .filter((t) => !t.deletedAt && t.status === "done")
    .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))
    .slice(0, 5));

  const activeMembers = createMemo(
    () => (members() ?? []).filter((m) => memberStatus(m) === "active").length,
  );

  // Next event = the earliest event whose end is in the future (or has no
  // end and starts in the future). Falls back to undefined when there are
  // no future events.
  const nextEvent = createMemo(() => {
    const now = Date.now();
    const futures = (events() ?? []).filter((e) => {
      const ref = e.endsAt ?? e.startsAt;
      return ref ? Date.parse(ref) >= now : false;
    });
    futures.sort((a, b) => {
      const ka = Date.parse(a.startsAt ?? a.endsAt ?? "");
      const kb = Date.parse(b.startsAt ?? b.endsAt ?? "");
      return ka - kb;
    });
    return futures[0];
  });

  const taskFootEstimate = createMemo(() => {
    const ts = tasks() ?? [];
    const done = ts.filter((t) => t.status === "done").length;
    const remaining = ts.filter((t) => !t.deletedAt && !isTaskClosed(t));
    const minutes = remaining.reduce((acc, t) => acc + (t.estimateMinutes ?? 0), 0);
    return { done, remaining: remaining.length, hours: minutes / 60 };
  });

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
            You have <b>{tasksToday().length} {tasksToday().length === 1 ? "task" : "tasks"} due today</b>{" "}
            and {nextEventCopy(events()?.length ?? 0)}.{" "}
            {activeMembers()} {activeMembers() === 1 ? "member is" : "members are"} active right now.
          </p>
          <div class="hero-stats">
            <Stat n={tasksToday().length} label="tasks due" />
            <Stat n={(events() ?? []).length} label="events" />
            <Stat n={activeMembers()} label="active members" />
            <Stat n={(projects() ?? []).length} label="projects" />
          </div>
        </div>
      </section>

      <div class="grid">
        <section class="card reveal d2" data-area="tasks" aria-label="Tasks">
          <div class="card-hd">
            <div>
              <h2>Tasks</h2>
              <div class="sub">Assigned to you or watched by you.</div>
            </div>
          </div>

          <Show when={!tasks.loading} fallback={<p style="padding:18px;color:var(--ink-mute)">Loading…</p>}>
            <Show when={tasksOpen().length > 0} fallback={<EmptyTasks />}>
              <div class="tasks">
                <Show when={tasksOverdue().length > 0}>
                  <div class="group-lbl">Overdue</div>
                  <For each={tasksOverdue()}>{(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}</For>
                </Show>
                <Show when={tasksToday().length > 0}>
                  <div class="group-lbl">Today</div>
                  <For each={tasksToday()}>{(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}</For>
                </Show>
                <Show when={tasksWeek().length > 0}>
                  <div class="group-lbl">This week</div>
                  <For each={tasksWeek()}>{(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}</For>
                </Show>
                <Show when={tasksNoDate().length > 0}>
                  <div class="group-lbl">No due date</div>
                  <For each={tasksNoDate()}>{(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}</For>
                </Show>
                <Show when={tasksLater().length > 0}>
                  <div class="group-lbl">Later</div>
                  <For each={tasksLater()}>{(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}</For>
                </Show>
                <Show when={tasksRecent().length > 0}>
                  <div class="group-lbl" style="margin-top:18px;color:var(--ink-mute)">
                    Recently completed
                  </div>
                  <For each={tasksRecent()}>
                    {(t) => <TaskRow task={t} byId={memberById()} members={members() ?? []} onMutate={async () => { await refetchTasks(); }} />}
                  </For>
                </Show>
              </div>
            </Show>
          </Show>

          <div class="tasks-foot">
            <span>
              {taskFootEstimate().done} done · {taskFootEstimate().remaining} remaining
              <Show when={taskFootEstimate().hours > 0}>
                {" "}— estimated{" "}
                <b style="color:var(--grass-4);font-weight:600">
                  {taskFootEstimate().hours.toFixed(1)} hours
                </b>
              </Show>
            </span>
            <button class="btn-quiet"><Plus /> New task</button>
          </div>
        </section>

        <aside class="side">
          <section class="card gather reveal d3">
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

          <section class="card folks reveal d4">
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
                      <div class="doing">{m.linkkeysUserId}@{m.linkkeysDomain}</div>
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

const Stat = (props: { n: number; label: string }) => (
  <div class="stat">
    <span class="n">{props.n}</span>
    <span class="lbl">{props.label}</span>
  </div>
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
