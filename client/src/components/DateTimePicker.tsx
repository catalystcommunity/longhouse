/**
 * Side-by-side date + optional time inputs that emit a single ISO string
 * via `onChange`. Designed to replace `<input type="datetime-local">`
 * everywhere a value is optional or "just the date" is fine — that
 * native control requires both halves AND has the spinbutton UX on some
 * browsers, which is the issue we're routing around.
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

  return (
    <span style="display:inline-flex;gap:6px;align-items:center">
      <input
        type="date"
        value={parts().date}
        required={props.required}
        title={props.title}
        onInput={(e) => onDate(e.currentTarget.value)}
        style={inputBase}
      />
      <input
        type="time"
        value={parts().time}
        disabled={!parts().date}
        title={parts().date ? "Time (optional)" : "Pick a date first"}
        onInput={(e) => onTime(e.currentTarget.value)}
        style={inputBase + ";width:108px"}
      />
    </span>
  );
};
