import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { HeroScene } from "~/components/HeroScene";
import { AvatarStack } from "~/components/Avatar";
import { Check, Cloud, Pin, Plus } from "~/components/Icons";
import { useRepo } from "~/data/RepoContext";
import { memberById } from "~/data/mocks";
import type { Task } from "~/data/types";

const TODAY_LABEL = "Monday · 18 May · dashboard";

export const Dashboard = () => {
  const repo = useRepo();
  const [tasks] = createResource(() => repo.listTasks());
  const [members] = createResource(() => repo.listMembers());
  // local mutable view of done state — toggles in memory; will become
  // a mutation through repo.* when wired to the live API.
  const [doneIds, setDoneIds] = createSignal<Set<string>>(new Set());

  // initialize doneIds from the resource once it resolves
  const _ = createMemo(() => {
    const ts = tasks();
    if (ts && doneIds().size === 0 && ts.some((t) => t.done)) {
      setDoneIds(new Set(ts.filter((t) => t.done).map((t) => t.id)));
    }
  });

  const toggleDone = (id: string) =>
    setDoneIds((s) => {
      const next = new Set(s);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const isDone = (t: Task) => doneIds().has(t.id);

  const stats = createMemo(() => {
    const ts = tasks() ?? [];
    const totalDue = ts.filter((t) => t.groupLabel === "today").length;
    const active = (members() ?? []).filter((m) => m.status === "active").length;
    return { totalDue, active };
  });

  const taskFootEstimate = createMemo(() => {
    const ts = tasks() ?? [];
    const done = ts.filter(isDone).length;
    const remaining = ts.filter((t) => !isDone(t));
    const hours = remaining.reduce((acc, t) => acc + (t.estimateMinutes ?? 0), 0) / 60;
    return { done, remaining: remaining.length, hours };
  });

  const tasksToday = createMemo(() => (tasks() ?? []).filter((t) => t.groupLabel === "today"));
  const tasksWeek  = createMemo(() => (tasks() ?? []).filter((t) => t.groupLabel === "week"));

  const nextEvent = "Quarterly retreat planning";

  return (
    <>
      <section class="hero reveal d1">
        <HeroScene />
        <div class="hero-content">
          <span class="kicker">{TODAY_LABEL}</span>
          <h1 class="greet">Good morning,<br />Tod.</h1>
          <p class="greet-sub">
            You have <b>{stats().totalDue} tasks due today</b> and two events on the calendar.{" "}
            {stats().active} {stats().active === 1 ? "member is" : "members are"} active right now.
          </p>
          <div class="hero-stats">
            <Stat n={stats().totalDue} label="tasks due" />
            <Stat n={2} label="events" />
            <Stat n={stats().active} label="active members" />
            <Stat n={11} label="across projects" />
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
            <div class="card-hd-end">
              <Segmented options={["Today", "This week", "All"]} />
            </div>
          </div>

          <div class="tasks">
            <div class="group-lbl">Today</div>
            <For each={tasksToday()}>
              {(t) => <TaskRow task={t} done={isDone(t)} onToggle={() => toggleDone(t.id)} />}
            </For>

            <div class="group-lbl">This week</div>
            <For each={tasksWeek()}>
              {(t) => <TaskRow task={t} done={isDone(t)} onToggle={() => toggleDone(t.id)} />}
            </For>
          </div>

          <div class="tasks-foot">
            <span>
              {taskFootEstimate().done} done · {taskFootEstimate().remaining} remaining — estimated{" "}
              <b style="color:var(--grass-4);font-weight:600">
                {taskFootEstimate().hours.toFixed(1)} hours
              </b>
            </span>
            <button class="btn-quiet">
              <Plus />
              New task
            </button>
          </div>
        </section>

        <aside class="side">
          <section class="card gather reveal d3">
            <div class="card-hd" style="padding:0 0 12px;border:0">
              <div>
                <h2>Next event</h2>
                <div class="sub">Today · main meeting room</div>
              </div>
              <span
                class="kicker plain"
                style="background: color-mix(in oklab, var(--moss) 28%, var(--paper)); color: color-mix(in oklab, var(--moss), black 30%); border-color: color-mix(in oklab, var(--moss) 40%, transparent)"
              >
                6:30 pm
              </span>
            </div>
            <div class="when">Monday standing dinner</div>
            <div class="what">
              Quarterly <em>{nextEvent.split(" ").slice(1).join(" ") || "planning"}</em>
            </div>
            <div class="where">
              <Pin />
              Main meeting room · long table
            </div>
            <div class="going">
              <AvatarStack
                members={[
                  { initials: "T", swatch: "a1" },
                  { initials: "L", swatch: "a2" },
                  { initials: "S", swatch: "a3" },
                  { initials: "M", swatch: "a4" },
                ]}
              />
              <div class="count">
                <b>6 attending</b> · 2 tentative
              </div>
            </div>
            <div class="actions">
              <button class="btn btn-primary">Attending</button>
              <button class="btn btn-ghost">Add note</button>
            </div>
          </section>

          <section class="card folks reveal d4">
            <div class="card-hd" style="padding:0 0 12px;border:0">
              <div>
                <h2>Active members</h2>
                <div class="sub">Currently online or recently active</div>
              </div>
            </div>
            <Show when={members()}>
              {(ms) => (
                <For each={ms().filter((m) => m.id !== "tod")}>
                  {(m) => (
                    <div class={`folk ${m.status === "away" ? "away" : ""}`}>
                      <span class={`a lg ${m.swatch}`}>{m.initials}</span>
                      <div>
                        <div class="who-name">{m.name}</div>
                        <div class="doing">{m.doing}</div>
                      </div>
                      <span class="ago">{m.lastSeenLabel}</span>
                    </div>
                  )}
                </For>
              )}
            </Show>
          </section>

          <div class="season reveal d5">
            <div class="wx">
              <Cloud />
              <div>
                <b>16°C · light cloud</b>
                <br />
                <span style="font-size:12px;color:var(--ink-mute)">
                  all systems healthy · last sync 2m ago
                </span>
              </div>
            </div>
            <div class="moon">v 0.4.0</div>
          </div>
        </aside>
      </div>
    </>
  );
};

// ─── inline helpers ───────────────────────────────────────────

const Stat = (props: { n: number; label: string }) => (
  <div class="stat">
    <span class="n">{props.n}</span>
    <span class="lbl">{props.label}</span>
  </div>
);

const Segmented = (props: { options: string[] }) => {
  const [active, setActive] = createSignal(0);
  return (
    <div class="seg" role="tablist">
      <For each={props.options}>
        {(opt, i) => (
          <button class={active() === i() ? "on" : ""} onClick={() => setActive(i())}>
            {opt}
          </button>
        )}
      </For>
    </div>
  );
};

const TaskRow = (props: { task: Task; done: boolean; onToggle: () => void }) => {
  const assignees = createMemo(() =>
    props.task.assignees
      .map((id) => memberById.get(id))
      .filter((m): m is NonNullable<typeof m> => Boolean(m)),
  );

  return (
    <div class={`task ${props.done ? "done" : ""}`} onClick={props.onToggle}>
      <div class="check">
        <Check />
      </div>
      <div>
        <div class="t-title">{props.task.title}</div>
        <div class="t-meta">
          <Show when={props.task.tag}>{(tag) => <span class={`tag ${tag()}`}>{cap(tag())}</span>}</Show>
          <Show when={props.task.due}>{(d) => <><span class="dot" />{d()}</>}</Show>
          <Show when={props.task.meta}>{(m) => <><span class="dot" />{m()}</>}</Show>
        </div>
      </div>
      <div class="t-end">
        <AvatarStack members={assignees()} />
      </div>
    </div>
  );
};

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
