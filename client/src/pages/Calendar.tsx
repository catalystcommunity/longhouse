import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { ChevronLeft, ChevronRight, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { DateTimePicker } from "~/components/DateTimePicker";
import { RecurrenceFields, recurrenceLabel, toRecurrence, type RecurrenceFreq } from "~/components/RecurrenceFields";
import { eventClient } from "~/data/clients";
import { eventTone, timeLabel, ymdLocal } from "~/lib/derive";
import { useCurrentHouseId } from "~/stores/auth";
import { buildCells, groupSingleByDate, placeSpans } from "~/lib/month";
import type { Event as ApiEvent } from "~/api/types.gen";

const WEEKDAYS_MON_FIRST = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const MONTH_NAMES = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

// Time-grid configuration. Hours rendered from 6am to 11pm — enough for
// a working day plus evenings, without scrolling away the empty pre-dawn
// hours. Each cell is one hour tall; drag-select snaps to hour bounds.
const GRID_START_HOUR = 6;
const GRID_END_HOUR = 23;
const GRID_HOURS = GRID_END_HOUR - GRID_START_HOUR + 1;
const HOUR_HEIGHT = 44; // px per hour row

type ViewMode = "month" | "week" | "3day" | "day";
const VIEW_DAYS: Record<Exclude<ViewMode, "month">, number> = { week: 7, "3day": 3, day: 1 };

export const CalendarPage = () => {
  const houseId = useCurrentHouseId();
  const [allEvents, { refetch: refetchEvents }] = createResource(
    () => houseId(),
    async (h) => eventClient.listEvents({ houseId: h }),
  );

  // Composer state. `{ mode: "create" }` opens a blank form; with optional
  // `start`/`end` ISO strings to pre-fill the time range (cell-click or
  // drag-select). `{ mode: "edit", event }` pre-fills from an existing row.
  type ComposerState =
    | { mode: "create"; start?: string; end?: string; allDay?: boolean }
    | { mode: "edit"; event: ApiEvent };
  const [composer, setComposer] = createSignal<ComposerState | null>(null);

  const [view, setView] = createSignal<ViewMode>("month");
  // anchor = the date that determines what's visible. Month view: anchor's
  // month is shown. Week view: anchor's week (Mon-first). Day view: just
  // the anchor. Prev/Next nav adjusts by one view-unit at a time.
  const [anchor, setAnchor] = createSignal(new Date());

  const goPrev = () => setAnchor((a) => shiftAnchor(a, view(), -1));
  const goNext = () => setAnchor((a) => shiftAnchor(a, view(), 1));
  const goToday = () => setAnchor(new Date());

  // Visible events come straight from the server now — the recurrence
  // worker spawns real Event rows for each occurrence up to a 2-year
  // horizon, so there's nothing to expand client-side. We still tag
  // children with the 🔁 indicator so users know they're part of a series.
  const visibleEvents = createMemo(() => allEvents() ?? []);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Calendar <em>— the month at a glance</em></h2>
        <p class="lead">
          Switch views in the toolbar. Click an empty cell (month view) or drag
          across hours (day/week views) to start a new event.
        </p>
      </div>

      <Show when={composer()}>
        {(state) => (
          <EventComposer
            houseId={houseId()!}
            state={state()}
            onClose={() => setComposer(null)}
            onSaved={async () => { setComposer(null); await refetchEvents(); }}
          />
        )}
      </Show>

      <section class="cal reveal d1">
        <div class="cal-hd">
          <div class="cal-title">
            <h3 class="cal-month">{viewTitle(view(), anchor())}</h3>
            <span class="cal-sub">{(allEvents() ?? []).length} events in this house</span>
          </div>
          <div class="cal-controls">
            <div class="cal-nav">
              <button class="icon-btn" onClick={goPrev} aria-label="Previous"><ChevronLeft /></button>
              <button class="icon-btn today" onClick={goToday} aria-label="Today">Today</button>
              <button class="icon-btn" onClick={goNext} aria-label="Next"><ChevronRight /></button>
            </div>
            <div class="seg" role="tablist" style="display:inline-flex;border:1px solid var(--line);border-radius:var(--r-md);overflow:hidden">
              <For each={[
                ["day",   "Day"],
                ["3day",  "3-day"],
                ["week",  "Week"],
                ["month", "Month"],
              ] as const}>
                {([v, label]) => (
                  <button
                    onClick={() => setView(v)}
                    class={view() === v ? "on" : ""}
                    style={`padding:5px 12px;background:${view() === v ? "var(--ocean-1)" : "var(--paper)"};color:${view() === v ? "var(--paper)" : "var(--ink)"};border:0;border-right:1px solid var(--line);font-size:12px;cursor:pointer`}
                  >
                    {label}
                  </button>
                )}
              </For>
            </div>
            <button class="btn-quiet" onClick={() => setComposer({ mode: "create" })}>
              <Plus /> New event
            </button>
          </div>
        </div>

        <Show when={view() === "month"}>
          <MonthView
            anchor={anchor()}
            events={visibleEvents()}
            onCreate={(ymd) => setComposer({ mode: "create", start: dateAt(ymd, 9, 0), end: dateAt(ymd, 10, 0) })}
            onEdit={(e) => setComposer({ mode: "edit", event: e })}
          />
        </Show>
        <Show when={view() !== "month"}>
          <TimeGridView
            anchor={anchor()}
            days={VIEW_DAYS[view() as Exclude<ViewMode, "month">]}
            events={visibleEvents()}
            onCreate={(startIso, endIso) =>
              setComposer({ mode: "create", start: startIso, end: endIso })
            }
            onEdit={(e) => setComposer({ mode: "edit", event: e })}
          />
        </Show>
      </section>
    </AuthGate>
  );
};

// ─── Month view ───────────────────────────────────────────────────────

const MonthView = (props: {
  anchor: Date;
  events: ApiEvent[];
  onCreate: (ymd: string) => void;
  onEdit: (e: ApiEvent) => void;
}) => {
  const year = () => props.anchor.getFullYear();
  const month = () => props.anchor.getMonth();
  const todayYMD = (() => {
    const d = new Date();
    return ymdLocal(d.toISOString());
  })();
  const cells = createMemo(() => buildCells(year(), month(), todayYMD));
  const single = createMemo(() => groupSingleByDate(props.events));
  const spans = createMemo(() => placeSpans(props.events, cells()));

  return (
    <>
      <div class="cal-week" aria-hidden="true">
        <For each={WEEKDAYS_MON_FIRST}>
          {(d, i) => <div class={i() >= 5 ? "we" : ""}>{d}</div>}
        </For>
      </div>
      <div class="cal-grid">
        <For each={cells()}>
          {(c) => (
            <div
              class={[
                "cal-cell",
                c.inMonth ? "" : "other",
                c.isWeekend ? "we" : "",
                c.isToday ? "today" : "",
              ].filter(Boolean).join(" ")}
              onClick={(e) => {
                if ((e.target as HTMLElement).closest(".evt, .evt-span")) return;
                props.onCreate(c.ymd);
              }}
              style="cursor:pointer"
            >
              <span class="day">{c.date.getDate()}</span>
              <Show when={single().get(c.ymd)}>
                {(evts) => (
                  <div class="cal-events">
                    <For each={evts().slice(0, 3)}>
                      {(e) => (
                        <div
                          class="evt"
                          data-tone={eventTone(e.ownerMemberId)}
                          title={e.title}
                          onClick={(ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); }}
                        >
                          <time>{timeLabel(e)}</time>
                          <span class="title">{e.title}</span>
                          <Show when={isInstance(e)}>
                            <span title="recurring instance" style="margin-left:auto;font-size:10px;opacity:0.6">🔁</span>
                          </Show>
                        </div>
                      )}
                    </For>
                    <Show when={evts().length > 3}>
                      <div class="evt-more">+{evts().length - 3} more</div>
                    </Show>
                  </div>
                )}
              </Show>
            </div>
          )}
        </For>
        <For each={spans()}>
          {(s) => (
            <div
              class="evt-span"
              data-tone={eventTone(s.evt.ownerMemberId)}
              style={{
                "grid-row": s.row,
                "grid-column": `${s.col} / span ${s.span}`,
              } as any}
              title={s.evt.title}
              onClick={(ev) => { ev.stopPropagation(); props.onEdit(rootEvent(s.evt)); }}
            >
              <span class="title">{s.evt.title}</span>
            </div>
          )}
        </For>
      </div>
      <Show when={props.events.length === 0}>
        <p style="padding:18px;text-align:center;color:var(--ink-mute)">
          No events in this range. Click any day or "New event" to add one.
        </p>
      </Show>
    </>
  );
};

// ─── Time-grid view (day / 3-day / week) ──────────────────────────────

interface DragState {
  dayIdx: number;
  startHour: number;
  endHour: number; // inclusive (mouse-up snaps end to that hour cell)
}

const TimeGridView = (props: {
  anchor: Date;
  days: number;
  events: ApiEvent[];
  onCreate: (startIso: string, endIso: string) => void;
  onEdit: (e: ApiEvent) => void;
}) => {
  // Compute the visible day list (Date objects, midnight local).
  const visibleDays = createMemo(() => {
    const start = startOfRange(props.anchor, props.days === 7 ? "week" : props.days === 3 ? "3day" : "day");
    return Array.from({ length: props.days }, (_, i) => {
      const d = new Date(start);
      d.setDate(start.getDate() + i);
      return d;
    });
  });

  // Bucket events by their local YYYY-MM-DD. Multi-day events appear in
  // every day they touch within the visible range.
  const eventsByDay = createMemo(() => {
    const map = new Map<string, ApiEvent[]>();
    for (const e of props.events) {
      if (e.allDay) continue;
      if (!e.startsAt) continue;
      const s = new Date(e.startsAt);
      const en = new Date(e.endsAt ?? e.startsAt);
      // Walk across days for spans (capped at a sane number to avoid
      // pathological multi-year events filling the bucket).
      const max = 30;
      for (let i = 0; i < max; i++) {
        const d = new Date(s);
        d.setDate(s.getDate() + i);
        if (d.getTime() > en.getTime() && i > 0) break;
        const key = ymdLocal(d.toISOString());
        const arr = map.get(key) ?? [];
        arr.push(e);
        map.set(key, arr);
        if (d.toDateString() === en.toDateString()) break;
      }
    }
    return map;
  });

  // All-day events grouped by day for the top strip.
  const allDayByDay = createMemo(() => {
    const map = new Map<string, ApiEvent[]>();
    for (const e of props.events) {
      if (!e.allDay) continue;
      if (!e.startsAt) continue;
      const key = ymdLocal(e.startsAt);
      const arr = map.get(key) ?? [];
      arr.push(e);
      map.set(key, arr);
    }
    return map;
  });

  // Drag-to-create state.
  const [drag, setDrag] = createSignal<DragState | null>(null);
  const onMouseDown = (dayIdx: number, hour: number) => {
    setDrag({ dayIdx, startHour: hour, endHour: hour });
    // Wire up window-level move/up so a drag that leaves the cell still
    // resolves cleanly. The handlers self-remove on mouseup.
    const onMove = (ev: MouseEvent) => {
      const target = ev.target as HTMLElement | null;
      const cell = target?.closest("[data-hour]") as HTMLElement | null;
      if (!cell) return;
      const dayAttr = cell.getAttribute("data-day");
      const hourAttr = cell.getAttribute("data-hour");
      if (dayAttr === null || hourAttr === null) return;
      const dIdx = Number(dayAttr);
      const h = Number(hourAttr);
      // We only let drag extend within the original day column so the
      // resulting event is a single contiguous time range.
      setDrag((d) => (d ? { ...d, endHour: dIdx === d.dayIdx ? h : d.endHour } : d));
    };
    const onUp = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      const d = drag();
      setDrag(null);
      if (!d) return;
      const day = visibleDays()[d.dayIdx];
      if (!day) return;
      const lo = Math.min(d.startHour, d.endHour);
      const hi = Math.max(d.startHour, d.endHour);
      // End at the bottom of the last selected hour, so dragging 9–10
      // gives a 9:00–11:00 event (selection spans two hour-blocks).
      const startIso = atHour(day, lo, 0).toISOString();
      const endIso = atHour(day, hi + 1, 0).toISOString();
      props.onCreate(startIso, endIso);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const isCellInDrag = (dayIdx: number, hour: number): boolean => {
    const d = drag();
    if (!d) return false;
    if (d.dayIdx !== dayIdx) return false;
    const lo = Math.min(d.startHour, d.endHour);
    const hi = Math.max(d.startHour, d.endHour);
    return hour >= lo && hour <= hi;
  };

  const eventTopOffset = (e: ApiEvent, day: Date): number => {
    const s = new Date(e.startsAt!);
    // Clip to the visible day's window so a multi-day span starts at the
    // top of the column it didn't begin on.
    const sameDay = s.toDateString() === day.toDateString();
    if (!sameDay) return 0;
    const minutesFromTop = (s.getHours() - GRID_START_HOUR) * 60 + s.getMinutes();
    return Math.max(0, (minutesFromTop / 60) * HOUR_HEIGHT);
  };
  const eventHeight = (e: ApiEvent, day: Date): number => {
    const s = new Date(e.startsAt!);
    const en = new Date(e.endsAt ?? e.startsAt!);
    const dayStart = atHour(day, GRID_START_HOUR, 0);
    const dayEnd = atHour(day, GRID_END_HOUR + 1, 0);
    const visStart = s < dayStart ? dayStart : s;
    const visEnd = en > dayEnd ? dayEnd : en;
    const minutes = Math.max(30, (visEnd.getTime() - visStart.getTime()) / 60000);
    return (minutes / 60) * HOUR_HEIGHT;
  };

  return (
    <div style="display:flex;flex-direction:column;border-top:1px solid var(--line)">
      {/* Day header */}
      <div
        style={{
          display: "grid",
          "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
          "border-bottom": "1px solid var(--line)",
          background: "var(--paper)",
        }}
      >
        <div />
        <For each={visibleDays()}>
          {(d) => (
            <div style="padding:8px 10px;text-align:center;border-left:1px solid var(--line)">
              <div style="font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:var(--ink-mute)">
                {d.toLocaleDateString(undefined, { weekday: "short" })}
              </div>
              <div style={`font-size:18px;font-weight:600;color:${d.toDateString() === new Date().toDateString() ? "var(--ocean-2)" : "var(--ink)"}`}>
                {d.getDate()}
              </div>
            </div>
          )}
        </For>
      </div>

      {/* All-day strip */}
      <Show when={Array.from(allDayByDay().values()).flat().length > 0}>
        <div
          style={{
            display: "grid",
            "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
            "border-bottom": "1px solid var(--line)",
            background: "color-mix(in oklab, var(--paper) 96%, var(--ocean-1) 4%)",
          }}
        >
          <div style="padding:6px 8px;font-size:11px;color:var(--ink-mute);text-align:right">all-day</div>
          <For each={visibleDays()}>
            {(d) => {
              const items = () => allDayByDay().get(ymdLocal(d.toISOString())) ?? [];
              return (
                <div style="padding:4px;display:flex;flex-direction:column;gap:3px;border-left:1px solid var(--line);min-height:24px">
                  <For each={items()}>
                    {(e) => (
                      <div
                        class="evt"
                        data-tone={eventTone(e.ownerMemberId)}
                        title={e.title}
                        style="padding:2px 6px;font-size:11px;cursor:pointer"
                        onClick={(ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); }}
                      >
                        <span class="title">{e.title}</span>
                      </div>
                    )}
                  </For>
                </div>
              );
            }}
          </For>
        </div>
      </Show>

      {/* Time grid */}
      <div
        style={{
          display: "grid",
          "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
          position: "relative",
        }}
      >
        {/* Hour gutter */}
        <div>
          <For each={Array.from({ length: GRID_HOURS }, (_, i) => GRID_START_HOUR + i)}>
            {(h) => (
              <div style={`height:${HOUR_HEIGHT}px;padding:2px 8px;text-align:right;font-size:11px;color:var(--ink-mute);border-top:1px dashed var(--line)`}>
                {formatHour(h)}
              </div>
            )}
          </For>
        </div>
        {/* Day columns */}
        <For each={visibleDays()}>
          {(day, dayIdx) => {
            const dayEvents = () => eventsByDay().get(ymdLocal(day.toISOString())) ?? [];
            return (
              <div style="position:relative;border-left:1px solid var(--line)">
                <For each={Array.from({ length: GRID_HOURS }, (_, i) => GRID_START_HOUR + i)}>
                  {(h) => (
                    <div
                      data-day={dayIdx()}
                      data-hour={h}
                      onMouseDown={(e) => { e.preventDefault(); onMouseDown(dayIdx(), h); }}
                      style={`height:${HOUR_HEIGHT}px;border-top:1px dashed var(--line);cursor:cell;background:${isCellInDrag(dayIdx(), h) ? "color-mix(in oklab, var(--paper) 78%, var(--grass-1) 22%)" : "transparent"}`}
                    />
                  )}
                </For>
                <For each={dayEvents()}>
                  {(e) => (
                    <div
                      class="evt"
                      data-tone={eventTone(e.ownerMemberId)}
                      title={e.title}
                      style={`position:absolute;left:4px;right:4px;top:${eventTopOffset(e, day)}px;height:${eventHeight(e, day) - 2}px;overflow:hidden;padding:4px 6px;font-size:12px;border-radius:var(--r-sm);cursor:pointer;z-index:1`}
                      onClick={(ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); }}
                    >
                      <div style="font-weight:600;line-height:1.1">{e.title}</div>
                      <div style="font-size:10px;opacity:0.85">{timeLabel(e)}</div>
                      <Show when={isInstance(e)}>
                        <span title="recurring" style="position:absolute;top:2px;right:4px;font-size:10px;opacity:0.6">🔁</span>
                      </Show>
                    </div>
                  )}
                </For>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
};

// ─── Event composer (date/time + recurrence) ──────────────────────────

const EventComposer = (props: {
  houseId: string;
  state:
    | { mode: "create"; start?: string; end?: string; allDay?: boolean }
    | { mode: "edit"; event: ApiEvent };
  onClose: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const editing = () => props.state.mode === "edit";
  const initial = () => (props.state.mode === "edit" ? props.state.event : null);

  // Seed defaults: edit mode reads the event row; create mode prefers the
  // start/end the caller passed (drag-select or cell-click), else tomorrow
  // 9–10am.
  const seedStart = (): string => {
    if (initial()?.startsAt) return initial()!.startsAt!;
    if (props.state.mode === "create" && props.state.start) return props.state.start;
    const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(9, 0, 0, 0);
    return d.toISOString();
  };
  const seedEnd = (): string => {
    if (initial()?.endsAt) return initial()!.endsAt!;
    if (props.state.mode === "create" && props.state.end) return props.state.end;
    const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(10, 0, 0, 0);
    return d.toISOString();
  };

  const [title, setTitle]         = createSignal(initial()?.title ?? "");
  const [description, setDesc]    = createSignal(initial()?.description ?? "");
  const [location, setLocation]   = createSignal(initial()?.location ?? "");
  const [allDay, setAllDay]       = createSignal(
    initial()?.allDay ?? (props.state.mode === "create" ? Boolean(props.state.allDay) : false),
  );
  const [startsAt, setStartsAt]   = createSignal(seedStart());
  const [endsAt, setEndsAt]       = createSignal(seedEnd());
  // Recurrence — Event recurrence is client-expanded; server stores freq +
  // interval and starts_at is the anchor.
  const initialRecFreq = (): RecurrenceFreq => {
    const f = initial()?.recurrenceFreq;
    return typeof f === "string" ? (f as RecurrenceFreq) : "";
  };
  const [recFreq, setRecFreq] = createSignal<RecurrenceFreq>(initialRecFreq());
  const [recInterval, setRecInterval] = createSignal(initial()?.recurrenceInterval ?? 1);
  // For events the "first on" is starts_at itself, so we don't expose a
  // separate nextAt input. Pass empty to toRecurrence so it falls back to
  // starts_at — toRecurrence only sets nextRecurrenceAt, which the server
  // ignores for events (events have no next_recurrence_at column).
  const [recNextAt, setRecNextAt] = createSignal("");
  const [busy, setBusy]   = createSignal(false);
  const [err, setErr]     = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      const body: any = {
        houseId: props.houseId,
        ownerMemberId: editing() ? (props.state as { event: ApiEvent }).event.ownerMemberId : "",
        title: title().trim(),
        description: description().trim() || undefined,
        location: location().trim() || undefined,
        startsAt: startsAt() || undefined,
        endsAt:   endsAt()   || undefined,
        allDay: allDay(),
      };
      // Recurrence: send the freq + interval. Empty freq = clear an
      // existing recurrence on update.
      body.recurrenceFreq = recFreq() || "";
      if (recFreq()) body.recurrenceInterval = recInterval();
      // Suppress unused-var lint without coupling to setRecNextAt usage.
      void recNextAt;
      if (editing()) {
        await eventClient.updateEvent({
          ...(props.state as { event: ApiEvent }).event,
          ...body,
        } as any);
      } else {
        await eventClient.createEvent(body as any);
      }
      await props.onSaved();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (andFuture: boolean) => {
    if (props.state.mode !== "edit" || busy()) return;
    const t = props.state.event.title;
    const msg = andFuture
      ? `Delete "${t}" and every later occurrence in this series?`
      : `Delete "${t}"?`;
    if (!confirm(msg)) return;
    setBusy(true);
    setErr(null);
    try {
      if (andFuture) {
        await eventClient.deleteEventAndFuture(props.state.event.eventId);
      } else {
        await eventClient.deleteEvent(props.state.event.eventId);
      }
      await props.onSaved();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style="margin:0 0 16px;padding:18px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)"
    >
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
        <h3 style="margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          {editing() ? "Edit event" : "New event"}
        </h3>
        <button type="button" class="btn btn-ghost" onClick={props.onClose} disabled={busy()}>
          Cancel
        </button>
      </div>

      <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:12px;color:var(--ink-mute)">Title</span>
          <input
            type="text" value={title()} onInput={(e) => setTitle(e.currentTarget.value)}
            required autofocus
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:12px;color:var(--ink-mute)">Description (optional)</span>
          <textarea
            value={description()} onInput={(e) => setDesc(e.currentTarget.value)} rows="2"
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:12px;color:var(--ink-mute)">Location (optional)</span>
          <input
            type="text" value={location()} onInput={(e) => setLocation(e.currentTarget.value)}
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:12px;color:var(--ink-mute)">Starts</span>
          <DateTimePicker value={startsAt()} onChange={setStartsAt} title="Start" />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:12px;color:var(--ink-mute)">Ends</span>
          <DateTimePicker value={endsAt()} onChange={setEndsAt} title="End" />
        </label>
        <label style="display:flex;gap:8px;align-items:center;grid-column:1/-1">
          <input
            type="checkbox" checked={allDay()}
            onChange={(e) => setAllDay(e.currentTarget.checked)}
          />
          <span style="font-size:14px">All day</span>
        </label>
        <div style="grid-column:1/-1">
          <RecurrenceFields
            freq={recFreq}
            setFreq={setRecFreq}
            interval={recInterval}
            setInterval={setRecInterval}
            nextAt={recNextAt}
            setNextAt={setRecNextAt}
          />
        </div>
      </div>

      <Show when={err()}>
        {(m) => <p style="color:var(--rust);font-size:13px;margin:10px 0 0">{m()}</p>}
      </Show>

      <div style="display:flex;justify-content:space-between;gap:10px;margin-top:14px;flex-wrap:wrap">
        <Show when={editing()}>
          <div style="display:flex;gap:8px;flex-wrap:wrap">
            <button type="button" class="btn btn-ghost" onClick={() => remove(false)} disabled={busy()} style="color:var(--rust)">
              Delete
            </button>
            <Show when={isRecurring(initial())}>
              <button
                type="button" class="btn btn-ghost" onClick={() => remove(true)} disabled={busy()}
                style="color:var(--rust);border:1px dashed var(--rust)"
                title="Drop this occurrence and every later one. Clears the series's recurrence so the spawner stops respawning."
              >
                Delete this & future
              </button>
            </Show>
          </div>
        </Show>
        <div style="display:flex;gap:10px;margin-left:auto">
          <button class="btn btn-primary" disabled={busy() || !title().trim()} type="submit">
            {busy() ? "Saving…" : editing() ? "Save changes" : "Create event"}
          </button>
        </div>
      </div>
    </form>
  );
};

// ─── helpers ──────────────────────────────────────────────────────────

function viewTitle(view: ViewMode, anchor: Date): string {
  if (view === "month") return `${MONTH_NAMES[anchor.getMonth()]} ${anchor.getFullYear()}`;
  if (view === "day") {
    return anchor.toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric", year: "numeric" });
  }
  const start = startOfRange(anchor, view);
  const end = new Date(start); end.setDate(start.getDate() + (view === "week" ? 6 : 2));
  const sameMonth = start.getMonth() === end.getMonth();
  if (sameMonth) {
    return `${start.toLocaleDateString(undefined, { month: "long", day: "numeric" })}–${end.getDate()}, ${start.getFullYear()}`;
  }
  return `${start.toLocaleDateString(undefined, { month: "short", day: "numeric" })} – ${end.toLocaleDateString(undefined, { month: "short", day: "numeric" })}, ${start.getFullYear()}`;
}

function viewRange(view: ViewMode, anchor: Date): { start: Date; end: Date } {
  if (view === "month") {
    const first = new Date(anchor.getFullYear(), anchor.getMonth(), 1);
    // Mon-first 6-week window covers any month + spill-over days.
    const day = first.getDay();
    const diff = day === 0 ? -6 : 1 - day;
    const gridStart = new Date(first); gridStart.setDate(first.getDate() + diff);
    const gridEnd = new Date(gridStart); gridEnd.setDate(gridStart.getDate() + 42);
    return { start: gridStart, end: gridEnd };
  }
  const start = startOfRange(anchor, view);
  const days = VIEW_DAYS[view];
  const end = new Date(start); end.setDate(start.getDate() + days);
  return { start, end };
}

function startOfRange(anchor: Date, view: Exclude<ViewMode, "month">): Date {
  if (view === "day") {
    const d = new Date(anchor); d.setHours(0, 0, 0, 0); return d;
  }
  if (view === "3day") {
    const d = new Date(anchor); d.setHours(0, 0, 0, 0); return d;
  }
  // week — Mon-first
  const d = new Date(anchor); d.setHours(0, 0, 0, 0);
  const day = d.getDay();
  const diff = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + diff);
  return d;
}

function shiftAnchor(anchor: Date, view: ViewMode, dir: -1 | 1): Date {
  const d = new Date(anchor);
  if (view === "month")  d.setMonth(d.getMonth() + dir);
  else if (view === "day")  d.setDate(d.getDate() + dir);
  else if (view === "3day") d.setDate(d.getDate() + dir * 3);
  else if (view === "week") d.setDate(d.getDate() + dir * 7);
  return d;
}

function atHour(day: Date, h: number, m: number): Date {
  const d = new Date(day);
  d.setHours(h, m, 0, 0);
  return d;
}

function dateAt(ymd: string, h: number, m: number): string {
  const [y, mo, d] = ymd.split("-").map(Number);
  return new Date(y, mo - 1, d, h, m, 0, 0).toISOString();
}

function formatHour(h: number): string {
  if (h === 0) return "12 am";
  if (h === 12) return "noon";
  if (h < 12) return `${h} am`;
  return `${h - 12} pm`;
}

// ─── Recurring-event expansion ────────────────────────────────────────
//
// The api stores recurring events as a single root row with `recurrence_freq`
// + `recurrence_interval`. We expand that into per-occurrence pseudo-events
// at render time, only within the visible range. Each instance carries the
// same metadata as the root but with shifted starts_at/ends_at; an internal
// flag distinguishes them so the edit click routes back to the root row.

// Either a recurring root (has recurrence_freq) or a spawned child (has
// recurrence_root_event_id) is considered "recurring" for UI purposes —
// both should expose the delete-and-future option.
function isRecurring(e: ApiEvent | null): boolean {
  if (!e) return false;
  const rootSet = typeof e.recurrenceRootEventId === "string" && e.recurrenceRootEventId !== "";
  const freqSet = typeof e.recurrenceFreq === "string" && e.recurrenceFreq !== "";
  return rootSet || freqSet;
}

// A row is a recurring-series instance when the server has set
// recurrence_root_event_id (the worker tags each spawned child this way).
// Editing one needs to walk back to the root so changes apply to the
// whole series instead of silently diverging.
function isInstance(e: ApiEvent): boolean {
  return typeof e.recurrenceRootEventId === "string" && e.recurrenceRootEventId !== "";
}

// rootEvent returns the row to edit when the user clicks any occurrence:
// the root for an instance (so series edits land in one place), or the
// row itself otherwise. The actual root fetch happens server-side via
// GetEvent; we just return the id for the edit composer to resolve.
function rootEvent(e: ApiEvent): ApiEvent {
  return e; // editor uses event_id directly; the API does the right thing.
}

// Re-export so recurrenceLabel stays usable in the time-grid event chips
// without an extra import bookkeeping line.
export { recurrenceLabel };
