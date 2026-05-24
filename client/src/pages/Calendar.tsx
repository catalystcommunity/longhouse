import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { ChevronLeft, ChevronRight } from "~/components/Icons";
import { useRepo } from "~/data/RepoContext";
import { buildCells, groupSingleByDate, placeSpans } from "~/lib/month";

// for now this page is fixed to May 2026 (matches the comp) — when we
// wire to real data this becomes a signal with prev/next month nav.
const YEAR = 2026;
const MONTH = 4; // 0-indexed: May
const TODAY_YMD = "2026-05-18";

const WEEKDAYS_MON_FIRST = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const MONTH_NAMES = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

export const CalendarPage = () => {
  const repo = useRepo();
  const [allEvents] = createResource(() => repo.listEvents());
  const [honor, setHonor] = createSignal(false);

  const cells = createMemo(() => buildCells(YEAR, MONTH, TODAY_YMD));
  const single = createMemo(() => groupSingleByDate(allEvents() ?? []));
  const spans = createMemo(() => placeSpans(allEvents() ?? [], cells()));

  const spanningCount = createMemo(() =>
    (allEvents() ?? []).filter((e) => e.startDate !== e.endDate).length,
  );

  return (
    <>
      <div class="section-hd reveal">
        <h2>
          Calendar <em>— the month at a glance</em>
        </h2>
        <p class="lead">
          Organizers can suggest a color for each event; the calendar may honor it or fall
          back to the theme palette.
        </p>
      </div>

      <section class="cal reveal d1" data-honor={honor() ? "true" : "false"}>
        <div class="cal-hd">
          <div class="cal-title">
            <h3 class="cal-month">
              {MONTH_NAMES[MONTH]} <em>{YEAR}</em>
            </h3>
            <span class="cal-sub">
              {(allEvents() ?? []).length} events · {spanningCount()} spanning
            </span>
          </div>
          <div class="cal-controls">
            <div class="cal-nav">
              <button class="icon-btn" aria-label="Previous month"><ChevronLeft /></button>
              <button class="icon-btn today" aria-label="Today">Today</button>
              <button class="icon-btn" aria-label="Next month"><ChevronRight /></button>
            </div>
            <Segmented options={["Month", "Week", "List"]} />
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
              >
                <span class="day">{c.date.getDate()}</span>
                <Show when={single().get(c.ymd)}>
                  {(evts) => (
                    <div class="cal-events">
                      <For each={evts().slice(0, 3)}>
                        {(e) => (
                          <div
                            class="evt"
                            data-tone={e.tone}
                            style={{ "--evt-c-org": e.organizerColor } as any}
                            title={e.title}
                          >
                            <time>{e.timeLabel}</time>
                            <span class="title">{e.title}</span>
                            <span class="sugg-dot" />
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

          {/* multi-day bars overlaid on the grid via explicit grid placement */}
          <For each={spans()}>
            {(s) => (
              <div
                class="evt-span"
                data-tone={s.evt.tone}
                style={{
                  "grid-row": s.row,
                  "grid-column": `${s.col} / span ${s.span}`,
                  "--evt-c-org": s.evt.organizerColor,
                } as any}
                title={s.evt.title}
              >
                <span class="title">{s.evt.title}</span>
              </div>
            )}
          </For>
        </div>

        <div class="cal-legend">
          <span style="font-weight:600;color:var(--ink-soft)">Theme palette</span>
          <span class="swatch-pair"><span class="sw" style="--c:var(--ocean-1)" /> admin</span>
          <span class="swatch-pair"><span class="sw" style="--c:var(--grass-3)" /> outdoor</span>
          <span class="swatch-pair"><span class="sw" style="--c:var(--sky-3)" /> social</span>
          <span class="swatch-pair"><span class="sw" style="--c:var(--heather)" /> retreat</span>
          <span class="swatch-pair"><span class="sw" style="--c:var(--moss)" /> operations</span>
          <span class="sep" />
          <span style="font-family: var(--display); font-style: italic; font-variation-settings: 'opsz' 14, 'SOFT' 100, 'wght' 380;">
            toggle{" "}
            <b style="color:var(--ink-soft);font-weight:600;font-style:normal;font-family:var(--sans)">
              Honor organizer colors
            </b>{" "}
            to see each event use its organizer's suggested hue instead.
          </span>
        </div>
      </section>
    </>
  );
};

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
