import { Show, type Accessor } from "solid-js";
import { DateTimePicker } from "./DateTimePicker";

/**
 * Reusable "this task repeats every N <unit>" form fragment used by every
 * task composer. The recurrence model lives entirely in the api Task type
 * — `recurrence_freq`, `recurrence_interval`, `next_recurrence_at` — and
 * the recurrence worker handles spawning the children server-side.
 *
 * Callers own the signals; this component just renders the inputs and
 * surfaces a `toRecurrence()` helper that produces the payload fields to
 * merge into a CreateTask body.
 */

export type RecurrenceFreq =
  | ""           // no repeat
  | "daily"
  | "weekly"
  | "monthly"
  | "quarterly"
  | "yearly";

export const FREQ_LABELS: Record<RecurrenceFreq, string> = {
  "":          "Never",
  daily:       "Day(s)",
  weekly:      "Week(s)",
  monthly:     "Month(s)",
  quarterly:   "Quarter(s)",
  yearly:      "Year(s)",
};

interface Props {
  freq: Accessor<RecurrenceFreq>;
  setFreq: (v: RecurrenceFreq) => void;
  interval: Accessor<number>;
  setInterval: (v: number) => void;
  /** ISO timestamp for the next spawn. Empty string = "use task's due_at". */
  nextAt: Accessor<string>;
  setNextAt: (v: string) => void;
  /** Compact density for the project-view composer; not used elsewhere. */
  compact?: boolean;
}

export const RecurrenceFields = (props: Props) => {
  const inputBase =
    "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px";
  return (
    <div
      style={`display:flex;flex-wrap:wrap;gap:${props.compact ? "8px" : "12px"};align-items:center`}
    >
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
        Repeat
        <select
          value={props.freq()}
          onChange={(e) => props.setFreq(e.currentTarget.value as RecurrenceFreq)}
          style={inputBase}
        >
          <option value="">— never —</option>
          <option value="daily">Daily</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
          <option value="quarterly">Quarterly</option>
          <option value="yearly">Yearly</option>
        </select>
      </label>
      <Show when={props.freq() !== ""}>
        <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
          every
          <input
            type="number"
            min="1"
            value={String(props.interval())}
            onInput={(e) => props.setInterval(Math.max(1, Number(e.currentTarget.value) || 1))}
            style={`${inputBase};width:64px`}
          />
          {FREQ_LABELS[props.freq()]}
        </label>
        <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
          first on
          <DateTimePicker
            value={props.nextAt()}
            onChange={props.setNextAt}
            title="When the first occurrence should spawn (defaults to the task's due date)"
          />
        </label>
      </Show>
    </div>
  );
};

/** Build the recurrence-related fields for the createTask body. Returns
 *  an empty object when no repeat is selected. */
export function toRecurrence(
  freq: RecurrenceFreq,
  interval: number,
  nextAtIso: string,
  fallbackDueAt?: string,
): Record<string, unknown> {
  if (!freq) return {};
  // Prefer the explicit "first on" picker; fall back to due_at when the
  // user left it blank, so "monthly, due May 1" still spawns correctly.
  // DateTimePicker emits ISO directly so no further normalization needed.
  const firstIso = nextAtIso || fallbackDueAt || "";
  return {
    recurrenceFreq: freq,
    recurrenceInterval: interval,
    nextRecurrenceAt: firstIso || undefined,
  };
}

/** Display string for an existing task's recurrence, or undefined when
 *  it isn't a recurring task. Used in task-row meta. */
const FREQ_SINGULAR: Record<string, string> = {
  daily:     "day",
  weekly:    "week",
  monthly:   "month",
  quarterly: "quarter",
  yearly:    "year",
};

export function recurrenceLabel(t: {
  recurrenceFreq?: unknown;
  recurrenceInterval?: number;
}): string | undefined {
  const freq = typeof t.recurrenceFreq === "string" ? t.recurrenceFreq : undefined;
  if (!freq) return undefined;
  const unit = FREQ_SINGULAR[freq] ?? freq;
  const n = t.recurrenceInterval ?? 1;
  if (n === 1) return `every ${unit}`;
  return `every ${n} ${unit}s`;
}

