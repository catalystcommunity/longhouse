import { For, Show, createSignal } from "solid-js";
import { DateTimePicker } from "./DateTimePicker";
import { RecurrenceFields, type RecurrenceFreq } from "./RecurrenceFields";
import { taskClient } from "~/data/clients";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import type { Member, Task } from "~/api/types.gen";

/**
 * Inline detail-and-edit panel for a single task. Lives directly under
 * the task row in any list view (Tasks page, project detail). Surfaces
 * every editable field — title, description, tag, due, estimate, status,
 * assignees, recurrence — plus a delete action.
 *
 * Behavior: every field is an editable form control from the start; there
 * is no "view then edit" two-step. Save pushes a single UpdateTask call;
 * close just collapses without saving. The parent owns the open/closed
 * state via its own signal and re-fetches the task list on save so the
 * row above reflects the new values.
 */

interface Props {
  task: Task;
  members: Member[];
  onClose: () => void;
  onSaved: () => Promise<unknown>;
  /** Optional: confirm-and-delete this task. When omitted, no delete button. */
  onDelete?: () => Promise<unknown>;
}

const TASK_STATUSES = ["open", "in_progress", "done", "cancelled"] as const;

export const TaskDetailEditor = (props: Props) => {
  // We seed each control from the task and own the edit state locally;
  // saving sends the full updated Task back through UpdateTask.
  const [title, setTitle] = createSignal(props.task.title);
  const [description, setDesc] = createSignal(props.task.description ?? "");
  const [tag, setTag] = createSignal(props.task.tag ?? "");
  const [due, setDue] = createSignal(props.task.dueAt ?? "");
  const [estimate, setEstimate] = createSignal(
    props.task.estimateMinutes !== undefined ? String(props.task.estimateMinutes) : "",
  );
  const [status, setStatus] = createSignal<string>(
    typeof props.task.status === "string" ? props.task.status : "open",
  );
  const [assignees, setAssignees] = createSignal<string[]>((props.task.assignees ?? []).slice());
  const initRecFreq = (): RecurrenceFreq =>
    (typeof props.task.recurrenceFreq === "string" ? props.task.recurrenceFreq : "") as RecurrenceFreq;
  const [recFreq, setRecFreq] = createSignal<RecurrenceFreq>(initRecFreq());
  const [recInterval, setRecInterval] = createSignal(props.task.recurrenceInterval ?? 1);
  const [recNextAt, setRecNextAt] = createSignal(props.task.nextRecurrenceAt ?? "");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const toggleAssignee = (id: string) =>
    setAssignees((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  const save = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    setBusy(true);
    setErr(null);
    try {
      const body: any = {
        ...props.task,
        title: title().trim() || props.task.title,
        description: description(),
        tag: tag().trim() || undefined,
        dueAt: due() || undefined,
        estimateMinutes: estimate() ? Number(estimate()) : undefined,
        status: status(),
        assignees: assignees(),
        recurrenceFreq: recFreq() || "",
        recurrenceInterval: recFreq() ? recInterval() : undefined,
        nextRecurrenceAt: recNextAt() || undefined,
      };
      await taskClient.updateTask(body);
      await props.onSaved();
      props.onClose();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!props.onDelete) return;
    if (!confirm(`Delete "${props.task.title}"?`)) return;
    setBusy(true);
    setErr(null);
    try {
      await props.onDelete();
      props.onClose();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={save}
      style="margin:6px 0 12px 38px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-low);display:flex;flex-direction:column;gap:10px"
    >
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px">
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:11px;color:var(--ink-mute)">Title</span>
          <input
            type="text" value={title()} onInput={(e) => setTitle(e.currentTarget.value)}
            required
            style="padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:11px;color:var(--ink-mute)">Description</span>
          <textarea
            rows="2"
            value={description()} onInput={(e) => setDesc(e.currentTarget.value)}
            style="padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;resize:vertical"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:11px;color:var(--ink-mute)">Status</span>
          <select
            value={status()} onChange={(e) => setStatus(e.currentTarget.value)}
            style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          >
            <For each={TASK_STATUSES}>{(s) => <option value={s}>{s.replace("_", " ")}</option>}</For>
          </select>
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:11px;color:var(--ink-mute)">Tag</span>
          <input
            type="text" value={tag()} onInput={(e) => setTag(e.currentTarget.value)}
            placeholder="house, barn, …"
            style="padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:11px;color:var(--ink-mute)">Due</span>
          <DateTimePicker value={due()} onChange={setDue} title="Due (optional)" />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:11px;color:var(--ink-mute)">Estimate (min)</span>
          <input
            type="number" min="0" value={estimate()} onInput={(e) => setEstimate(e.currentTarget.value)}
            style="padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
          />
        </label>
      </div>

      <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
        <span style="font-size:11px;color:var(--ink-mute);margin-right:4px">Assignees</span>
        <For each={props.members}>
          {(m) => {
            const on = () => assignees().includes(m.memberId);
            return (
              <button
                type="button"
                onClick={() => toggleAssignee(m.memberId)}
                style={`display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;border-radius:9999px;font-size:12px;cursor:pointer;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`}
              >
                <span class={`a sm ${memberSwatch(m.memberId)}`} style="width:18px;height:18px;font-size:10px">{initial(m)}</span>
                {displayName(m)}
              </button>
            );
          }}
        </For>
      </div>

      <RecurrenceFields
        freq={recFreq}
        setFreq={setRecFreq}
        interval={recInterval}
        setInterval={setRecInterval}
        nextAt={recNextAt}
        setNextAt={setRecNextAt}
      />

      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>

      <div style="display:flex;gap:8px;justify-content:flex-end">
        <Show when={props.onDelete}>
          <button type="button" class="btn btn-ghost" onClick={remove} disabled={busy()} style="color:var(--rust);margin-right:auto;padding:6px 12px">
            Delete
          </button>
        </Show>
        <button type="button" class="btn btn-ghost" onClick={props.onClose} disabled={busy()} style="padding:6px 12px">Close</button>
        <button type="submit" class="btn btn-primary" disabled={busy() || !title().trim()} style="padding:6px 14px">
          {busy() ? "Saving…" : "Save"}
        </button>
      </div>
    </form>
  );
};
