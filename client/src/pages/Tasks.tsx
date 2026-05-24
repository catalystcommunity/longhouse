import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { Check, Plus } from "~/components/Icons";
import { AvatarStack } from "~/components/Avatar";
import { useRepo } from "~/data/RepoContext";
import { memberById } from "~/data/mocks";
import type { Task } from "~/data/types";

export const TasksPage = () => {
  const repo = useRepo();
  const [tasks] = createResource(() => repo.listTasks());
  const [done, setDone] = createSignal<Set<string>>(new Set());

  // initialize done set from data
  createMemo(() => {
    const ts = tasks();
    if (ts && done().size === 0 && ts.some((t) => t.done)) {
      setDone(new Set(ts.filter((t) => t.done).map((t) => t.id)));
    }
  });

  const toggle = (id: string) =>
    setDone((s) => {
      const next = new Set(s);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const grouped = createMemo(() => {
    const ts = tasks() ?? [];
    return {
      today: ts.filter((t) => t.groupLabel === "today"),
      week:  ts.filter((t) => t.groupLabel === "week"),
      later: ts.filter((t) => t.groupLabel === "later"),
    };
  });

  return (
    <>
      <div class="section-hd reveal">
        <h2>Tasks <em>everything on the slate</em></h2>
        <p class="lead">Everything assigned to you, watched by you, or shared with you.</p>
      </div>

      <section class="card reveal d1" style="margin-top: 0">
        <div class="card-hd">
          <div>
            <h2>Open tasks</h2>
            <div class="sub">
              {tasks() ? `${tasks()!.length - done().size} of ${tasks()!.length} remaining` : "…"}
            </div>
          </div>
          <button class="btn-quiet"><Plus /> New task</button>
        </div>

        <div class="tasks">
          <Show when={grouped().today.length}>
            <div class="group-lbl">Today</div>
            <For each={grouped().today}>
              {(t) => <Row task={t} done={done().has(t.id)} onToggle={() => toggle(t.id)} />}
            </For>
          </Show>
          <Show when={grouped().week.length}>
            <div class="group-lbl">This week</div>
            <For each={grouped().week}>
              {(t) => <Row task={t} done={done().has(t.id)} onToggle={() => toggle(t.id)} />}
            </For>
          </Show>
          <Show when={grouped().later.length}>
            <div class="group-lbl">Later</div>
            <For each={grouped().later}>
              {(t) => <Row task={t} done={done().has(t.id)} onToggle={() => toggle(t.id)} />}
            </For>
          </Show>
        </div>
      </section>
    </>
  );
};

const Row = (props: { task: Task; done: boolean; onToggle: () => void }) => {
  const assignees = createMemo(() =>
    props.task.assignees
      .map((id) => memberById.get(id))
      .filter((m): m is NonNullable<typeof m> => Boolean(m)),
  );
  return (
    <div class={`task ${props.done ? "done" : ""}`} onClick={props.onToggle}>
      <div class="check"><Check /></div>
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
