import { For, createEffect, createMemo, createSignal, onCleanup, onMount, type JSX } from "solid-js";

/**
 * A window-scrolled virtual list with rows of any height. Only the rows near
 * the viewport are in the DOM; spacers above and below hold the rest of the
 * height, so the page scrollbar matches the whole list, not the DOM.
 *
 * Heights are measured with a ResizeObserver and kept by row key, so a row
 * that grows (an opened editor) moves the rows under it, and a row measured
 * once keeps its height after it leaves the DOM. A row not yet measured uses
 * `estimate`. The list holds every row, loaded or not (a not-loaded row is a
 * placeholder entry), and `onRange` reports which rows are near the
 * viewport, so the caller can load exactly those.
 */

export interface VirtualListProps<T> {
  entries: T[];
  keyOf: (entry: T) => string;
  render: (entry: T) => JSX.Element;
  /** Height in px for a row that has not been measured. */
  estimate: (entry: T | undefined) => number;
  /** Called with the [start, end) entry indexes rendered now. */
  onRange?: (start: number, end: number) => void;
  /** Extra px rendered above and below the viewport. */
  overscan?: number;
}

export function VirtualList<T>(props: VirtualListProps<T>) {
  let root!: HTMLDivElement;
  const heights = new Map<string, number>();
  const [version, setVersion] = createSignal(0);
  const [view, setView] = createSignal({ top: 0, bottom: typeof window !== "undefined" ? window.innerHeight : 800 });

  // offsets[i] is the top of row i; offsets[n] is the end of loaded rows.
  const offsets = createMemo(() => {
    version();
    const out = new Array<number>(props.entries.length + 1);
    out[0] = 0;
    props.entries.forEach((e, i) => {
      out[i + 1] = out[i] + (heights.get(props.keyOf(e)) ?? props.estimate(e));
    });
    return out;
  });
  const totalHeight = () => offsets()[props.entries.length];

  const overscan = () => props.overscan ?? 600;

  // First row whose bottom is below `y`.
  const indexAt = (y: number) => {
    const o = offsets();
    let lo = 0;
    let hi = props.entries.length;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (o[mid + 1] <= y) lo = mid + 1;
      else hi = mid;
    }
    return lo;
  };
  const range = createMemo(() => {
    const v = view();
    const start = indexAt(v.top - overscan());
    const end = Math.min(props.entries.length, indexAt(v.bottom + overscan()) + 1);
    return { start, end };
  });
  const slice = createMemo(() => props.entries.slice(range().start, range().end));

  let frame = 0;
  const measureView = () => {
    frame = 0;
    if (!root) return;
    const rect = root.getBoundingClientRect();
    setView({ top: -rect.top, bottom: -rect.top + window.innerHeight });
  };
  const schedule = () => {
    if (!frame) frame = requestAnimationFrame(measureView);
  };

  const observer =
    typeof ResizeObserver !== "undefined"
      ? new ResizeObserver((records) => {
          let changed = false;
          for (const r of records) {
            const key = (r.target as HTMLElement).dataset.vkey;
            if (!key) continue;
            const h = (r.target as HTMLElement).offsetHeight;
            if (heights.get(key) !== h) {
              heights.set(key, h);
              changed = true;
            }
          }
          if (changed) setVersion((v) => v + 1);
        })
      : null;

  onMount(() => {
    measureView();
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
  });
  onCleanup(() => {
    window.removeEventListener("scroll", schedule);
    window.removeEventListener("resize", schedule);
    if (frame) cancelAnimationFrame(frame);
    observer?.disconnect();
  });

  createEffect(() => {
    const r = range();
    props.onRange?.(r.start, r.end);
  });

  return (
    <div ref={root} style={{ position: "relative", height: `${totalHeight()}px` }}>
      <div style={{ position: "absolute", left: 0, right: 0, top: `${offsets()[range().start] ?? 0}px` }}>
        <For each={slice()}>
          {(entry) => (
            <div
              class="vrow"
              data-vkey={props.keyOf(entry)}
              ref={(el) => {
                observer?.observe(el);
                onCleanup(() => observer?.unobserve(el));
              }}
            >
              {props.render(entry)}
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
