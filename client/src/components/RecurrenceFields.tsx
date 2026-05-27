import { For, Show, type Accessor } from "solid-js";
import { DateTimePicker } from "./DateTimePicker";

/**
 * Reusable recurrence form fragment for task + event composers. Mirrors
 * the api's recurrence model:
 *   * recurrence_freq + recurrence_interval        — "every N <unit>"
 *   * recurrence_by_weekday                        — weekday filter
 *   * recurrence_by_setpos                         — Nth (or "last")
 *                                                    matching weekday in
 *                                                    monthly/quarterly/yearly
 *   * next_recurrence_at                           — spawn anchor
 *
 * Callers own the signals; this component just renders the controls and
 * surfaces `toRecurrence()` for the createTask/createEvent body.
 */

export type RecurrenceFreq =
  | ""
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

const WEEKDAYS: { label: string; value: number }[] = [
  { label: "S", value: 0 },
  { label: "M", value: 1 },
  { label: "T", value: 2 },
  { label: "W", value: 3 },
  { label: "T", value: 4 },
  { label: "F", value: 5 },
  { label: "S", value: 6 },
];

const SETPOS_OPTIONS: { label: string; value: number }[] = [
  { label: "first",  value: 1 },
  { label: "second", value: 2 },
  { label: "third",  value: 3 },
  { label: "fourth", value: 4 },
  { label: "last",   value: -1 },
];

const PERIOD_LABEL: Record<RecurrenceFreq, string> = {
  "":          "",
  daily:       "",
  weekly:      "",
  monthly:     "month",
  quarterly:   "quarter",
  yearly:      "year",
};

const WEEKDAY_NAMES = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

interface Props {
  freq: Accessor<RecurrenceFreq>;
  setFreq: (v: RecurrenceFreq) => void;
  interval: Accessor<number>;
  setInterval: (v: number) => void;
  /** ISO timestamp for the next spawn. Empty string = "use task's due_at". */
  nextAt: Accessor<string>;
  setNextAt: (v: string) => void;
  /** Weekday filter, 0=Sun..6=Sat. Empty = no filter. */
  byWeekday: Accessor<number[]>;
  setByWeekday: (v: number[]) => void;
  /** Setpos for monthly/quarterly/yearly. 0/null = none. */
  bySetpos: Accessor<number>;
  setBySetpos: (v: number) => void;
  /** Compact density for the project-view composer; not used elsewhere. */
  compact?: boolean;
}

// "Weekdays" / "Weekends" are UI-only presets: server-side they're just
// weekly + interval=1 + byWeekday=[1..5] (Mon-Fri) or [0,6] (Sun+Sat).
// We display them as their own dropdown options so the user doesn't
// have to think about how to express them with the underlying model.
const WEEKDAYS_PRESET = [1, 2, 3, 4, 5] as const;
const WEEKENDS_PRESET = [0, 6] as const;

const sameWeekdaySet = (a: readonly number[], b: readonly number[]) => {
  if (a.length !== b.length) return false;
  const sa = [...a].sort();
  const sb = [...b].sort();
  return sa.every((x, i) => x === sb[i]);
};

export const RecurrenceFields = (props: Props) => {
  const inputBase =
    "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px";

  // Display value for the dropdown — "weekdays"/"weekends" when the
  // underlying model happens to match the preset, otherwise the raw freq.
  type DisplayFreq = RecurrenceFreq | "weekdays" | "weekends";
  const displayFreq = (): DisplayFreq => {
    if (props.freq() === "weekly" && props.interval() === 1) {
      if (sameWeekdaySet(props.byWeekday(), WEEKDAYS_PRESET)) return "weekdays";
      if (sameWeekdaySet(props.byWeekday(), WEEKENDS_PRESET)) return "weekends";
    }
    return props.freq();
  };

  // Inverse: when the user picks something in the dropdown, expand
  // presets to their underlying (freq, interval, byWeekday) triple.
  const onSelectFreq = (next: DisplayFreq) => {
    if (next === "weekdays") {
      props.setFreq("weekly");
      props.setInterval(1);
      props.setByWeekday([...WEEKDAYS_PRESET]);
      props.setBySetpos(0);
      return;
    }
    if (next === "weekends") {
      props.setFreq("weekly");
      props.setInterval(1);
      props.setByWeekday([...WEEKENDS_PRESET]);
      props.setBySetpos(0);
      return;
    }
    // Switching to a real freq from a preset clears the preset's
    // by_weekday filter so the user doesn't inherit "Mon-Fri" into a
    // monthly schedule by mistake.
    const wasPreset = displayFreq() === "weekdays" || displayFreq() === "weekends";
    props.setFreq(next);
    if (wasPreset) {
      props.setByWeekday([]);
    }
  };

  const isPreset = () => displayFreq() === "weekdays" || displayFreq() === "weekends";

  const isPeriodic = () => {
    const f = props.freq();
    return f === "monthly" || f === "quarterly" || f === "yearly";
  };

  const toggleWeekday = (d: number) => {
    const cur = props.byWeekday();
    if (cur.includes(d)) {
      props.setByWeekday(cur.filter((x) => x !== d));
    } else {
      props.setByWeekday([...cur, d].sort((a, b) => a - b));
    }
  };

  // For monthly/quarterly/yearly we only want one weekday selected at a
  // time (the standard "the Nth Tuesday" pattern). Use a single-pick
  // setter so the strip behaves like radio buttons in that mode.
  const setSingleWeekday = (d: number) => {
    props.setByWeekday([d]);
  };

  return (
    <div
      style={`display:flex;flex-wrap:wrap;gap:${props.compact ? "8px" : "12px"};align-items:center`}
    >
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
        Repeat
        <select
          value={displayFreq()}
          onChange={(e) => onSelectFreq(e.currentTarget.value as DisplayFreq)}
          style={inputBase}
        >
          <option value="">— never —</option>
          <option value="daily">Daily</option>
          <option value="weekdays">Weekdays (Mon–Fri)</option>
          <option value="weekends">Weekends (Sat+Sun)</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
          <option value="quarterly">Quarterly</option>
          <option value="yearly">Yearly</option>
        </select>
      </label>

      <Show when={props.freq() !== "" && !isPreset()}>
        <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
          every
          <input
            type="number"
            min="1"
            value={String(props.interval())}
            onInput={(e) =>
              props.setInterval(Math.max(1, Number(e.currentTarget.value) || 1))
            }
            style={`${inputBase};width:64px`}
          />
          {FREQ_LABELS[props.freq()]}
        </label>
      </Show>

      {/* Weekly: free multi-select weekday strip. Hidden under presets,
          where the weekdays/weekends choice already pins the set. */}
      <Show when={props.freq() === "weekly" && !isPreset()}>
        <span style="display:flex;align-items:center;gap:6px;flex-basis:100%">
          <span style="font-size:12px;color:var(--ink-mute)">on</span>
          <span style="display:inline-flex;gap:4px">
            <For each={WEEKDAYS}>
              {(w) => {
                const on = () => props.byWeekday().includes(w.value);
                return (
                  <button
                    type="button"
                    aria-pressed={on()}
                    aria-label={WEEKDAY_NAMES[w.value]}
                    onClick={() => toggleWeekday(w.value)}
                    style={`width:28px;height:28px;border-radius:9999px;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-2) 35%, var(--paper))" : "var(--paper)"};color:var(--ink);font-size:12px;font-weight:600;cursor:pointer`}
                  >
                    {w.label}
                  </button>
                );
              }}
            </For>
          </span>
        </span>
      </Show>

      {/* Monthly/quarterly/yearly: "the Nth <weekday> of the <period>". */}
      <Show when={isPeriodic()}>
        <span style="display:flex;align-items:center;flex-wrap:wrap;gap:6px;flex-basis:100%;font-size:12px;color:var(--ink-mute)">
          on the
          <select
            value={String(props.bySetpos() || 1)}
            onChange={(e) => props.setBySetpos(Number(e.currentTarget.value))}
            style={inputBase}
          >
            <For each={SETPOS_OPTIONS}>
              {(o) => <option value={o.value}>{o.label}</option>}
            </For>
          </select>
          <span style="display:inline-flex;gap:4px">
            <For each={WEEKDAYS}>
              {(w) => {
                const on = () => props.byWeekday()[0] === w.value;
                return (
                  <button
                    type="button"
                    aria-pressed={on()}
                    aria-label={WEEKDAY_NAMES[w.value]}
                    onClick={() => setSingleWeekday(w.value)}
                    style={`width:28px;height:28px;border-radius:9999px;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-2) 35%, var(--paper))" : "var(--paper)"};color:var(--ink);font-size:12px;font-weight:600;cursor:pointer`}
                  >
                    {w.label}
                  </button>
                );
              }}
            </For>
          </span>
          of the {PERIOD_LABEL[props.freq()]}
        </span>
      </Show>

      <Show when={props.freq() !== ""}>
        <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)">
          first on
          <DateTimePicker
            value={props.nextAt()}
            onChange={props.setNextAt}
            title="When the first occurrence should spawn (defaults to the task's due date or the event's start)"
          />
        </label>
      </Show>
    </div>
  );
};

/** Build the recurrence-related fields for a CreateTask/CreateEvent body.
 *  Returns an empty object when no repeat is selected. */
export function toRecurrence(
  freq: RecurrenceFreq,
  interval: number,
  nextAtIso: string,
  byWeekday: number[],
  bySetpos: number,
  fallbackIso?: string,
): Record<string, unknown> {
  if (!freq) return {};
  const firstIso = nextAtIso || fallbackIso || "";
  const out: Record<string, unknown> = {
    recurrenceFreq: freq,
    recurrenceInterval: interval,
    nextRecurrenceAt: firstIso || undefined,
  };
  if (byWeekday.length > 0) {
    out.recurrenceByWeekday = byWeekday;
  }
  if ((freq === "monthly" || freq === "quarterly" || freq === "yearly") && bySetpos !== 0) {
    out.recurrenceBySetpos = bySetpos;
  }
  return out;
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

const SETPOS_WORD: Record<number, string> = {
  1: "first",
  2: "second",
  3: "third",
  4: "fourth",
  [-1]: "last",
};

export function recurrenceLabel(t: {
  recurrenceFreq?: unknown;
  recurrenceInterval?: number;
  recurrenceByWeekday?: number[];
  recurrenceBySetpos?: number;
}): string | undefined {
  const freq = typeof t.recurrenceFreq === "string" ? t.recurrenceFreq : undefined;
  if (!freq) return undefined;
  const unit = FREQ_SINGULAR[freq] ?? freq;
  const n = t.recurrenceInterval ?? 1;
  const bw = t.recurrenceByWeekday ?? [];
  const sp = t.recurrenceBySetpos ?? 0;

  // "second Thursday of the month" style
  if ((freq === "monthly" || freq === "quarterly" || freq === "yearly") && bw.length > 0 && sp !== 0) {
    const ord = SETPOS_WORD[sp] ?? String(sp);
    const dayName = WEEKDAY_NAMES[bw[0]] ?? "day";
    const every = n === 1 ? "every" : `every ${n}`;
    return `${every} ${unit}, ${ord} ${dayName}`;
  }
  // Weekly + named days: "every Mon/Wed/Fri". The two common presets
  // get friendlier copy.
  if (freq === "weekly" && bw.length > 0) {
    if (n === 1) {
      const sorted = [...bw].sort();
      if (sorted.join(",") === "1,2,3,4,5") return "every weekday";
      if (sorted.join(",") === "0,6") return "every weekend day";
    }
    const days = bw.map((d) => WEEKDAY_NAMES[d]?.slice(0, 3) ?? "").join("/");
    if (n === 1) return `every ${days}`;
    return `every ${n} weeks, ${days}`;
  }
  if (n === 1) return `every ${unit}`;
  return `every ${n} ${unit}s`;
}
