import { Show, createSignal, onCleanup, onMount } from "solid-js";
import { ClockTimePicker } from "./ClockTimePicker";

/**
 * Side-by-side date + optional time inputs that emit a single ISO string
 * via `onChange`. Designed to replace `<input type="datetime-local">`
 * everywhere a value is optional or "just the date" is fine — that
 * native control requires both halves AND has the spinbutton UX on some
 * browsers, which is the issue we're routing around.
 *
 * The time half opens a Material-style clock-face picker (tap + drag)
 * in a popover. Falls back to typed input via the same "HH:mm" string
 * value so the rest of the form stays controlled.
 *
 * Behavior:
 *   * Empty date  → empty ISO emitted. Time input is disabled.
 *   * Date only   → ISO at `defaultTime` (default 12:00 local).
 *   * Date + time → ISO at the explicit local time.
 *
 * Everything is interpreted in the viewer's local timezone before being
 * serialized to a UTC ISO string for the wire — same convention the
 * old datetime-local inputs used.
 */

interface Props {
  value: string;                  // ISO string, "" = none
  onChange: (iso: string) => void;
  /** Used when the user picks a date but no time. Default "12:00". */
  defaultTime?: string;
  required?: boolean;
  /** Title attribute forwarded to both inputs for tooltip context. */
  title?: string;
}

const pad = (n: number) => String(n).padStart(2, "0");

function decompose(iso: string): { date: string; time: string } {
  if (!iso) return { date: "", time: "" };
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return { date: "", time: "" };
  return {
    date: `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`,
    time: `${pad(d.getHours())}:${pad(d.getMinutes())}`,
  };
}

function compose(date: string, time: string): string {
  if (!date) return "";
  const safeTime = time || "12:00";
  const [y, mo, d] = date.split("-").map(Number);
  const [h, mi] = safeTime.split(":").map(Number);
  const local = new Date(y, mo - 1, d, h, mi, 0, 0);
  return local.toISOString();
}

const inputBase =
  "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px";

export const DateTimePicker = (props: Props) => {
  const parts = () => decompose(props.value);
  const [open, setOpen] = createSignal(false);
  let wrapRef: HTMLSpanElement | undefined;

  const onDate = (date: string) => {
    if (!date) {
      // Clearing the date means "no due date" — wipe the whole value so
      // the form treats it as unset (the time half is then unreachable).
      props.onChange("");
      return;
    }
    props.onChange(compose(date, parts().time || props.defaultTime || ""));
  };

  const onTime = (time: string) => {
    const { date } = parts();
    if (!date) return; // ignore stray time edits before a date is picked
    props.onChange(compose(date, time));
  };

  // Close the clock popover on outside click / Escape so it behaves
  // like a normal dropdown.
  const onDocClick = (e: MouseEvent) => {
    if (!open()) return;
    if (wrapRef && !wrapRef.contains(e.target as Node)) setOpen(false);
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") setOpen(false);
  };
  onMount(() => {
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKey);
  });
  onCleanup(() => {
    document.removeEventListener("click", onDocClick);
    document.removeEventListener("keydown", onKey);
  });

  const timeButtonStyle = () =>
    inputBase +
    ";min-width:96px;cursor:pointer;text-align:left;font-variant-numeric:tabular-nums" +
    (parts().date ? "" : ";opacity:0.55;cursor:not-allowed");

  // Human-friendly display: "9:00 AM" instead of "09:00". Keeps the
  // controlled value canonical 24h while showing the same convention the
  // clock picker uses.
  const displayTime = () => {
    const t = parts().time;
    if (!t) return "—:—";
    const [h, m] = t.split(":").map(Number);
    const period = h >= 12 ? "PM" : "AM";
    const h12 = h % 12 === 0 ? 12 : h % 12;
    return `${h12}:${pad(m)} ${period}`;
  };

  return (
    <span ref={wrapRef!} style="display:inline-flex;gap:6px;align-items:center;position:relative">
      <input
        type="date"
        value={parts().date}
        required={props.required}
        title={props.title}
        onInput={(e) => onDate(e.currentTarget.value)}
        style={inputBase}
      />
      <button
        type="button"
        disabled={!parts().date}
        title={parts().date ? "Pick a time" : "Pick a date first"}
        aria-haspopup="dialog"
        aria-expanded={open() ? "true" : "false"}
        onClick={() => parts().date && setOpen((v) => !v)}
        style={timeButtonStyle()}
      >
        {displayTime()}
      </button>
      <Show when={open()}>
        <div
          role="dialog"
          aria-label="Pick a time"
          style="position:absolute;top:calc(100% + 6px);right:0;z-index:60"
        >
          <ClockTimePicker
            value={parts().time || props.defaultTime || "12:00"}
            onChange={onTime}
            disabled={!parts().date}
          />
        </div>
      </Show>
    </span>
  );
};
