import { For, Show } from "solid-js";
import type { Event as ApiEvent } from "~/api/types.gen";

/**
 * Compact one-day timeline. Renders a vertical strip from `startHour` to
 * `endHour` with hour labels on the left and event blocks positioned by
 * their `startsAt`/`endsAt` (or a short default duration when an event
 * has no end). All-day events appear in a small ribbon above the strip.
 *
 * Visual mirror of Calendar.tsx's day view, sized for a sidebar card
 * rather than a full page. Click handler fires per event block, so the
 * parent can route to /calendar?view=day&date=…&event=… for full detail.
 */

interface Props {
  /** The day this strip represents (any time during it). */
  date: Date;
  /** All events in the house — the strip filters to the ones overlapping
   *  `date`. */
  events: ApiEvent[];
  onPick?: (e: ApiEvent) => void;
  /** Inclusive hours bracket. Defaults match Calendar's day view. */
  startHour?: number;
  endHour?: number;
  /** Hour-row height in px. Smaller = more compact. */
  hourHeight?: number;
  /** When an event has no endsAt, draw the block this many minutes tall. */
  defaultDurationMin?: number;
}

const sameLocalDay = (a: Date, b: Date) =>
  a.getFullYear() === b.getFullYear() &&
  a.getMonth() === b.getMonth() &&
  a.getDate() === b.getDate();

export const DayStrip = (props: Props) => {
  const startHour = props.startHour ?? 6;
  const endHour = props.endHour ?? 22;
  const hourHeight = props.hourHeight ?? 18;
  const defaultDuration = props.defaultDurationMin ?? 60;
  const totalMinutes = (endHour - startHour + 1) * 60;
  const stripHeight = (endHour - startHour + 1) * hourHeight;

  const dayDate = () => props.date;
  const eventsToday = () => {
    const target = dayDate();
    return (props.events ?? []).filter((e) => {
      const s = e.startsAt ? new Date(e.startsAt) : null;
      const en = e.endsAt ? new Date(e.endsAt) : s;
      if (!s) return false;
      // An event belongs to this day if its start OR end lands on it (or
      // any minute in between for multi-day spans).
      if (sameLocalDay(s, target)) return true;
      if (en && sameLocalDay(en, target)) return true;
      if (en && s < target && target < en) return true;
      return false;
    });
  };

  const allDay = () => eventsToday().filter((e) => e.allDay);
  const timed = () => eventsToday().filter((e) => !e.allDay && e.startsAt);

  const blockGeometry = (e: ApiEvent) => {
    const s = new Date(e.startsAt!);
    const en = e.endsAt ? new Date(e.endsAt) : new Date(s.getTime() + defaultDuration * 60_000);
    // Clamp to the visible window.
    const stripStart = new Date(dayDate());
    stripStart.setHours(startHour, 0, 0, 0);
    const stripEnd = new Date(dayDate());
    stripEnd.setHours(endHour + 1, 0, 0, 0);
    const sMin = Math.max(0, (s.getTime() - stripStart.getTime()) / 60_000);
    const eMin = Math.min(totalMinutes, (en.getTime() - stripStart.getTime()) / 60_000);
    const top = Math.max(0, (sMin / 60) * hourHeight);
    const height = Math.max(hourHeight * 0.55, ((eMin - sMin) / 60) * hourHeight);
    return { top, height };
  };

  return (
    <div style="display:flex;flex-direction:column;gap:6px">
      <Show when={allDay().length > 0}>
        <div style="display:flex;flex-wrap:wrap;gap:4px">
          <For each={allDay()}>
            {(e) => (
              <button
                type="button"
                onClick={() => props.onPick?.(e)}
                title={e.title}
                style="padding:4px 8px;background:color-mix(in oklab, var(--ocean-1) 35%, var(--paper));border:1px solid var(--line);border-radius:9999px;font-size:12px;color:var(--ink);cursor:pointer;text-align:left;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"
              >
                All day · {e.title}
              </button>
            )}
          </For>
        </div>
      </Show>

      <div
        style={`position:relative;display:grid;grid-template-columns:32px 1fr;gap:6px;height:${stripHeight}px`}
        aria-label={`Schedule for ${dayDate().toDateString()}`}
      >
        {/* Hour labels */}
        <div style="display:flex;flex-direction:column">
          <For each={Array.from({ length: endHour - startHour + 1 }, (_, i) => startHour + i)}>
            {(h) => (
              <div style={`height:${hourHeight}px;font-size:10px;color:var(--ink-mute);text-align:right;line-height:1;padding-top:1px`}>
                {h === 12 ? "12p" : h > 12 ? `${h - 12}p` : `${h}a`}
              </div>
            )}
          </For>
        </div>
        {/* Hour grid + event blocks */}
        <div style="position:relative;background:color-mix(in oklab, var(--paper) 80%, var(--surface) 20%);border:1px solid var(--line);border-radius:6px;overflow:hidden">
          {/* Hour separators */}
          <For each={Array.from({ length: endHour - startHour }, (_, i) => i + 1)}>
            {(i) => (
              <div style={`position:absolute;left:0;right:0;top:${i * hourHeight}px;height:1px;background:var(--line);opacity:0.5`} />
            )}
          </For>
          <For each={timed()}>
            {(e) => {
              const g = blockGeometry(e);
              return (
                <button
                  type="button"
                  onClick={() => props.onPick?.(e)}
                  title={e.title}
                  style={`position:absolute;left:3px;right:3px;top:${g.top}px;height:${g.height}px;background:color-mix(in oklab, var(--grass-3) 32%, var(--paper));border:1px solid var(--grass-4);border-radius:5px;padding:2px 6px;font-size:11px;color:var(--ink);text-align:left;cursor:pointer;overflow:hidden;text-overflow:ellipsis;white-space:nowrap`}
                >
                  {e.title}
                </button>
              );
            }}
          </For>
        </div>
      </div>
    </div>
  );
};
