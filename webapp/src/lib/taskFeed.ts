import { batch, createSignal, untrack, type Accessor } from "solid-js";
import type { Task, TaskList } from "@longhouse/client";

/**
 * A sparse, server-ordered task list. The server defines the order for each
 * filter (see TaskListRequest in longhouse.csil). Every matching row has a
 * fixed slot: its server offset. Pages load by slot, so a list can fetch the
 * page under the scroll position directly, with nothing above it loaded.
 *
 * Slots count rows the caller cannot read. A page of 50 slots that returns
 * 47 tasks and hidden_count 3 fills its first 47 slots with the tasks (in
 * order) and its last 3 with "hidden" markers, which a list draws at zero
 * height. The server only says HOW MANY rows of a page are hidden, not
 * which, so this keeps the readable order and moves hidden rows to the end
 * of their page.
 *
 * Loaded pages stay in memory; a virtual list decides which rows are in the
 * DOM.
 */

/** Fetch `limit` slots from `offset`. `withCounts` asks for the extras that
 *  only the first page needs (due-date group counts). */
export type PageFetcher = (offset: number, limit: number, withCounts: boolean) => Promise<TaskList>;

export const DEFAULT_PAGE_SIZE = 50;
/** The server caps a page at this many rows. */
export const MAX_PAGE_SIZE = 500;

interface Page {
  tasks: Task[];
  hidden: number;
}

/** What a slot holds: a task, HIDDEN (not readable), or undefined (not loaded). */
export const HIDDEN = null;
export type Slot = Task | typeof HIDDEN | undefined;

export interface TaskFeed {
  /** True once the first page told us the total. */
  ready: Accessor<boolean>;
  /** Slots in the list: matching rows before visibility. */
  total: Accessor<number>;
  /** due_counts from the first page, when the fetcher asked for them. */
  dueCounts: Accessor<number[] | null>;
  /** Readable rows, best guess: total less the hidden rows seen so far. */
  estimatedTotal: Accessor<number>;
  /** True when every page is loaded. */
  allLoaded: Accessor<boolean>;
  /** Loaded readable rows, in slot order. */
  loadedRows: Accessor<Task[]>;
  slot: (i: number) => Slot;
  error: Accessor<unknown>;
  /** Load every page that covers slots [start, end). */
  ensure: (start: number, end: number) => void;
  /** Re-read the first page and every loaded page, after a change. The
   *  pages swap in together, so rows never mix two snapshots. */
  refresh: () => Promise<void>;
  /** Start again with a new fetcher (null clears the feed). */
  reset: (fetcher: PageFetcher | null) => void;
}

export function createTaskFeed(pageSize = DEFAULT_PAGE_SIZE): TaskFeed {
  const [pages, setPages] = createSignal<Map<number, Page>>(new Map());
  const [total, setTotal] = createSignal<number | null>(null);
  const [dueCounts, setDueCounts] = createSignal<number[] | null>(null);
  const [error, setError] = createSignal<unknown>(undefined);

  let fetcher: PageFetcher | null = null;
  // Bumped on reset and refresh; a response for an older generation is
  // dropped, and the page is fetched again on the next ensure.
  let generation = 0;
  const inFlight = new Set<number>();
  let lastRange: [number, number] = [0, pageSize];

  const toPage = (list: TaskList): Page => ({ tasks: list.tasks, hidden: Number(list.hiddenCount) });
  const pagesFor = (start: number, end: number) => {
    const t = total();
    const last = t === null ? 0 : Math.max(0, Math.ceil(t / pageSize) - 1);
    const from = Math.max(0, Math.floor(start / pageSize));
    const to = Math.min(last, Math.floor(Math.max(start, end - 1) / pageSize));
    const out: number[] = [];
    for (let p = from; p <= to; p++) out.push(p);
    return out;
  };

  const fetchPage = (p: number) => {
    if (!fetcher || inFlight.has(p) || pages().has(p)) return;
    const gen = generation;
    inFlight.add(p);
    fetcher(p * pageSize, pageSize, p === 0)
      .then((list) => {
        if (gen !== generation) return;
        batch(() => {
          setError(undefined);
          if (list.total !== undefined) setTotal(Number(list.total));
          if (p === 0 && list.dueCounts) setDueCounts(list.dueCounts.map(Number));
          const next = new Map(pages());
          next.set(p, toPage(list));
          setPages(next);
        });
      })
      .catch((e) => {
        if (gen === generation) setError(e);
      })
      .finally(() => {
        if (gen === generation) inFlight.delete(p);
      });
  };

  const ensure = (start: number, end: number) => {
    lastRange = [start, end];
    if (total() === null) {
      fetchPage(0);
      return;
    }
    for (const p of pagesFor(start, end)) fetchPage(p);
  };

  const refresh = async () => {
    if (!fetcher) return;
    const f = fetcher;
    const gen = ++generation;
    inFlight.clear();
    const want = new Set<number>([0, ...pages().keys()]);
    try {
      const results = await Promise.all(
        [...want].map(async (p) => [p, await f(p * pageSize, pageSize, p === 0)] as const),
      );
      if (gen !== generation) return;
      const first = results.find(([p]) => p === 0)![1];
      const t = first.total !== undefined ? Number(first.total) : null;
      const lastPage = t === null ? Infinity : Math.ceil(t / pageSize) - 1;
      const next = new Map<number, Page>();
      for (const [p, list] of results) if (p <= lastPage) next.set(p, toPage(list));
      batch(() => {
        setError(undefined);
        setTotal(t);
        setDueCounts(first.dueCounts ? first.dueCounts.map(Number) : null);
        setPages(next);
      });
      // The total can change; fill whatever is on screen now.
      ensure(...lastRange);
    } catch (e) {
      if (gen === generation) setError(e);
    }
  };

  const reset = (next: PageFetcher | null) => {
    generation++;
    inFlight.clear();
    fetcher = next;
    lastRange = [0, pageSize];
    batch(() => {
      setPages(new Map());
      setTotal(null);
      setDueCounts(null);
      setError(undefined);
    });
    if (next) fetchPage(0);
  };

  const slot = (i: number): Slot => {
    const page = pages().get(Math.floor(i / pageSize));
    if (!page) return undefined;
    const at = i % pageSize;
    return at < page.tasks.length ? page.tasks[at] : HIDDEN;
  };

  const hiddenSeen = () => {
    let n = 0;
    for (const p of pages().values()) n += p.hidden;
    return n;
  };
  const loadedRows = () => {
    const keys = [...pages().keys()].sort((a, b) => a - b);
    return keys.flatMap((k) => pages().get(k)!.tasks);
  };
  const allLoaded = () => {
    const t = total();
    return t !== null && pages().size >= Math.ceil(t / pageSize);
  };

  // The actions read the feed's own signals. Callers start them from
  // effects, so untrack them: an effect must not re-run each time a page
  // lands.
  return {
    ready: () => total() !== null,
    total: () => total() ?? 0,
    dueCounts,
    estimatedTotal: () => Math.max(0, (total() ?? 0) - hiddenSeen()),
    allLoaded,
    loadedRows,
    slot,
    error,
    ensure: (start, end) => untrack(() => ensure(start, end)),
    refresh: () => untrack(refresh),
    reset: (next) => untrack(() => reset(next)),
  };
}

/** Every task a fetcher returns, page by page. For pickers that need the
 *  whole set (a parent task, a dependency); lists use a feed. */
export async function listAllTasks(fetch: (offset: number, limit: number) => Promise<TaskList>): Promise<Task[]> {
  const out: Task[] = [];
  let offset = 0;
  for (;;) {
    const page = await fetch(offset, MAX_PAGE_SIZE);
    out.push(...page.tasks);
    const used = page.tasks.length + Number(page.hiddenCount);
    offset += used;
    if (used < MAX_PAGE_SIZE) return out;
    if (page.total !== undefined && offset >= Number(page.total)) return out;
  }
}

// ─── Grouped rows for a virtual list ───────────────────────────────────

/** One row of a task list: a group header, a task, a slot still loading,
 *  or a slot the caller cannot read. */
export type TaskEntry =
  | { kind: "header"; key: string; label: string }
  | { kind: "task"; key: string; task: Task; slot: number }
  | { kind: "pending"; key: string; slot: number }
  | { kind: "hidden"; key: string; slot: number };

/** A group: the label and the first slot. A group ends where the next one
 *  starts (the last one at the total). */
export interface GroupStart {
  label: string;
  start: number;
}

/**
 * Builds the rows for a feed, reusing row objects between builds. A row the
 * virtual list keeps on screen must be the same object after a page lands,
 * or Solid remounts it (and an open editor in it closes).
 */
export function createEntryBuilder() {
  let cache = new Map<string, TaskEntry>();
  return (feed: TaskFeed, groups: GroupStart[] = []): TaskEntry[] => {
    const next = new Map<string, TaskEntry>();
    const out: TaskEntry[] = [];
    const reuse = <E extends TaskEntry>(fresh: E, same: (old: E) => boolean): E => {
      const old = cache.get(fresh.key) as E | undefined;
      const e = old && old.kind === fresh.kind && same(old) ? old : fresh;
      next.set(e.key, e);
      return e;
    };
    const total = feed.total();
    const starts = groups.filter((g, i) => {
      const end = i + 1 < groups.length ? groups[i + 1].start : total;
      return end > g.start;
    });
    let g = 0;
    for (let i = 0; i < total; i++) {
      while (g < starts.length && starts[g].start === i) {
        const label = starts[g].label;
        out.push(reuse({ kind: "header", key: `h:${label}`, label }, (o) => o.label === label));
        g++;
      }
      const s = feed.slot(i);
      if (s === undefined) out.push(reuse({ kind: "pending", key: `p:${i}`, slot: i }, () => true));
      else if (s === HIDDEN) out.push(reuse({ kind: "hidden", key: `x:${i}`, slot: i }, () => true));
      else out.push(reuse({ kind: "task", key: s.taskId, task: s, slot: i }, (o) => o.task === s && o.slot === i));
    }
    cache = next;
    return out;
  };
}

/** Group starts for an open, due-ordered list from its due_counts, which
 *  were requested with dueBoundaries(). */
export function dueGroupStarts(counts: number[] | null, labels: string[]): GroupStart[] {
  if (!counts || counts.length + 1 !== labels.length) return [];
  return labels.map((label, i) => ({ label, start: i === 0 ? 0 : counts[i - 1] }));
}

/** Boundaries that split a due-ordered list into overdue, today, this week,
 *  later, and no date, the same cut points as derive.taskGroup. */
export function dueBoundaries(now = new Date()): string[] {
  const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const day = 24 * 3600 * 1000;
  return [
    new Date(startToday).toISOString(),
    new Date(startToday + day).toISOString(),
    new Date(startToday + 7 * day).toISOString(),
    // Rows with any due date; the rest have none.
    "9999-01-01T00:00:00Z",
  ];
}

/** Row heights for rows not measured yet: they size the scrollbar until a
 *  row renders once. A hidden slot takes no space. */
export const estimateTaskEntry = (e: TaskEntry | undefined) =>
  e?.kind === "header" ? 34 : e?.kind === "hidden" ? 0 : 58;
export const taskEntryKey = (e: TaskEntry) => e.key;

/** The slot range a run of rows covers, for TaskFeed.ensure. */
export function slotRange(entries: TaskEntry[], start: number, end: number): [number, number] | null {
  let lo = Infinity;
  let hi = -Infinity;
  for (let i = start; i < end; i++) {
    const e = entries[i];
    if (e && e.kind !== "header") {
      lo = Math.min(lo, e.slot);
      hi = Math.max(hi, e.slot);
    }
  }
  return lo === Infinity ? null : [lo, hi + 1];
}
