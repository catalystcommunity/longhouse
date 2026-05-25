import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { ChevronLeft, ChevronRight, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
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

export const CalendarPage = () => {
  const houseId = useCurrentHouseId();
  const [allEvents, { refetch: refetchEvents }] = createResource(
    () => houseId(),
    async (h) => eventClient.listEvents({ houseId: h }),
  );

  // Month cursor — start on today's month so the grid renders meaningful
  // data immediately. Prev/Next bump by one month at a time.
  const [year, setYear] = createSignal(new Date().getFullYear());
  const [month, setMonth] = createSignal(new Date().getMonth());
  const [honor, setHonor] = createSignal(false);

  // Composer state — null means closed. `{ mode: "create" }` opens a blank
  // form; `{ mode: "edit", event }` pre-fills.
  type ComposerState =
    | { mode: "create"; defaultDate?: string }
    | { mode: "edit"; event: ApiEvent };
  const [composer, setComposer] = createSignal<ComposerState | null>(null);

  const todayYMD = (() => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  })();

  const cells = createMemo(() => buildCells(year(), month(), todayYMD));
  const single = createMemo(() => groupSingleByDate(allEvents() ?? []));
  const spans = createMemo(() => placeSpans(allEvents() ?? [], cells()));

  const spanningCount = createMemo(() => {
    const arr = allEvents() ?? [];
    return arr.filter((e) => {
      const s = e.startsAt ? ymdLocal(e.startsAt) : "";
      const en = e.endsAt ? ymdLocal(e.endsAt) : s;
      return s && en && s !== en;
    }).length;
  });

  const goPrev  = () => { if (month() === 0)  { setMonth(11); setYear((y) => y - 1); } else setMonth((m) => m - 1); };
  const goNext  = () => { if (month() === 11) { setMonth(0);  setYear((y) => y + 1); } else setMonth((m) => m + 1); };
  const goToday = () => { const d = new Date(); setYear(d.getFullYear()); setMonth(d.getMonth()); };

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Calendar <em>— the month at a glance</em></h2>
        <p class="lead">
          Organizers can suggest a color for each event; the calendar may honor it or fall
          back to the theme palette.
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

      <section class="cal reveal d1" data-honor={honor() ? "true" : "false"}>
        <div class="cal-hd">
          <div class="cal-title">
            <h3 class="cal-month">
              {MONTH_NAMES[month()]} <em>{year()}</em>
            </h3>
            <span class="cal-sub">
              {(allEvents() ?? []).length} events · {spanningCount()} spanning
            </span>
          </div>
          <div class="cal-controls">
            <div class="cal-nav">
              <button class="icon-btn" onClick={goPrev} aria-label="Previous month"><ChevronLeft /></button>
              <button class="icon-btn today" onClick={goToday} aria-label="Today">Today</button>
              <button class="icon-btn" onClick={goNext} aria-label="Next month"><ChevronRight /></button>
            </div>
            <button class="btn-quiet" onClick={() => setComposer({ mode: "create" })}>
              <Plus /> New event
            </button>
            <label class="toggle">
              <input
                type="checkbox"
                checked={honor()}
                onChange={(e) => setHonor(e.currentTarget.checked)}
              />
              <span class="switch" />
              Honor organizer colors
              <span class="hint">suggestion, not a rule</span>
            </label>
          </div>
        </div>

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
                  // Empty-cell click → open composer pre-seeded with that date.
                  // Don't fire when the click came from inside an event chip
                  // (those handle their own onClick to open edit mode).
                  if ((e.target as HTMLElement).closest(".evt, .evt-span")) return;
                  setComposer({ mode: "create", defaultDate: c.ymd });
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
                            onClick={(ev) => { ev.stopPropagation(); setComposer({ mode: "edit", event: e }); }}
                          >
                            <time>{timeLabel(e)}</time>
                            <span class="title">{e.title}</span>
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
                onClick={(ev) => { ev.stopPropagation(); setComposer({ mode: "edit", event: s.evt }); }}
              >
                <span class="title">{s.evt.title}</span>
              </div>
            )}
          </For>
        </div>

        <Show when={(allEvents() ?? []).length === 0 && !allEvents.loading}>
          <p style="padding:24px;text-align:center;color:var(--ink-mute)">
            No events on the calendar yet. Click a day or "New event" to add one.
          </p>
        </Show>
      </section>
    </AuthGate>
  );
};

// ─── Composer ─────────────────────────────────────────────────────────

const EventComposer = (props: {
  houseId: string;
  state: { mode: "create"; defaultDate?: string } | { mode: "edit"; event: ApiEvent };
  onClose: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const editing = () => props.state.mode === "edit";
  const initial = () => (props.state.mode === "edit" ? props.state.event : null);

  // For "create" mode, pre-fill starts_at to 9am on the cell's date if one
  // was passed (cell-click flow); otherwise tomorrow 9am. Edit mode reads
  // the event's actual values.
  const seedDate = () => {
    if (props.state.mode === "edit") return ymdLocal(props.state.event.startsAt ?? new Date().toISOString());
    if (props.state.defaultDate) return props.state.defaultDate;
    const d = new Date(); d.setDate(d.getDate() + 1);
    return ymdLocal(d.toISOString());
  };

  const fmtLocal = (iso?: string) => {
    if (!iso) return "";
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return "";
    // `datetime-local` wants YYYY-MM-DDTHH:MM (no TZ).
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  const [title, setTitle]         = createSignal(initial()?.title ?? "");
  const [description, setDesc]    = createSignal(initial()?.description ?? "");
  const [location, setLocation]   = createSignal(initial()?.location ?? "");
  const [allDay, setAllDay]       = createSignal(Boolean(initial()?.allDay));
  const [startsAt, setStartsAt]   = createSignal(
    initial()?.startsAt ? fmtLocal(initial()!.startsAt) : `${seedDate()}T09:00`,
  );
  const [endsAt, setEndsAt]       = createSignal(
    initial()?.endsAt ? fmtLocal(initial()!.endsAt) : `${seedDate()}T10:00`,
  );
  const [busy, setBusy]   = createSignal(false);
  const [err, setErr]     = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      const body = {
        houseId: props.houseId,
        ownerMemberId: editing() ? (props.state as { event: ApiEvent }).event.ownerMemberId : "",
        title: title().trim(),
        description: description().trim() || undefined,
        location: location().trim() || undefined,
        startsAt: startsAt() ? new Date(startsAt()).toISOString() : undefined,
        endsAt:   endsAt()   ? new Date(endsAt()).toISOString()   : undefined,
        allDay: allDay(),
      };
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

  const remove = async () => {
    if (props.state.mode !== "edit" || busy()) return;
    if (!confirm(`Delete "${props.state.event.title}"?`)) return;
    setBusy(true);
    setErr(null);
    try {
      await eventClient.deleteEvent(props.state.event.eventId);
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
            type="text"
            value={title()}
            onInput={(e) => setTitle(e.currentTarget.value)}
            required
            autofocus
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:12px;color:var(--ink-mute)">Description (optional)</span>
          <textarea
            value={description()}
            onInput={(e) => setDesc(e.currentTarget.value)}
            rows="2"
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px;grid-column:1/-1">
          <span style="font-size:12px;color:var(--ink-mute)">Location (optional)</span>
          <input
            type="text"
            value={location()}
            onInput={(e) => setLocation(e.currentTarget.value)}
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:12px;color:var(--ink-mute)">Starts</span>
          <input
            type="datetime-local"
            value={startsAt()}
            onInput={(e) => setStartsAt(e.currentTarget.value)}
            disabled={allDay()}
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;flex-direction:column;gap:4px">
          <span style="font-size:12px;color:var(--ink-mute)">Ends</span>
          <input
            type="datetime-local"
            value={endsAt()}
            onInput={(e) => setEndsAt(e.currentTarget.value)}
            disabled={allDay()}
            style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
          />
        </label>
        <label style="display:flex;gap:8px;align-items:center;grid-column:1/-1">
          <input
            type="checkbox"
            checked={allDay()}
            onChange={(e) => setAllDay(e.currentTarget.checked)}
          />
          <span style="font-size:14px">All day</span>
        </label>
      </div>

      <Show when={err()}>
        {(m) => <p style="color:var(--rust);font-size:13px;margin:10px 0 0">{m()}</p>}
      </Show>

      <div style="display:flex;justify-content:space-between;gap:10px;margin-top:14px">
        <Show when={editing()}>
          <button type="button" class="btn btn-ghost" onClick={remove} disabled={busy()} style="color:var(--rust)">
            Delete
          </button>
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
