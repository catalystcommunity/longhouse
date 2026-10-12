import { describe, expect, it, vi } from "vitest";
import { createEffect, createRoot } from "solid-js";
import type { Task, TaskList } from "@longhouse/client";
import {
  HIDDEN,
  MAX_PAGE_SIZE,
  createEntryBuilder,
  createTaskFeed,
  dueGroupStarts,
  listAllTasks,
  slotRange,
} from "./taskFeed";

const task = (n: number): Task =>
  ({ taskId: `t${n}`, houseId: "h", ownerMemberId: "m", title: `Task ${n}`, assignees: [], createdAt: "", updatedAt: "" }) as Task;

/** A fake server: `count` rows in order; ids in `hiddenIds` are not
 *  readable. Reports the total before visibility, like the api. */
const fakeServer = (count: number, hiddenIds: Set<number> = new Set(), dueCounts?: number[]) => {
  const calls: { offset: number; limit: number; withCounts: boolean }[] = [];
  const fn = vi.fn(async (offset: number, limit: number, withCounts: boolean): Promise<TaskList> => {
    calls.push({ offset, limit, withCounts });
    const slice = Array.from({ length: Math.max(0, Math.min(limit, count - offset)) }, (_, i) => offset + i);
    return {
      tasks: slice.filter((n) => !hiddenIds.has(n)).map(task),
      hiddenCount: slice.filter((n) => hiddenIds.has(n)).length,
      total: count,
      ...(withCounts && dueCounts ? { dueCounts } : {}),
    };
  });
  return { fn, calls };
};

const settle = () => new Promise((r) => setTimeout(r, 0));

describe("task feed", () => {
  it("loads the first page on reset, with counts", async () => {
    const { fn, calls } = fakeServer(120, new Set(), [3, 10]);
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    expect(feed.ready()).toBe(true);
    expect(feed.total()).toBe(120);
    expect(feed.dueCounts()).toEqual([3, 10]);
    expect(calls).toEqual([{ offset: 0, limit: 50, withCounts: true }]);
    expect(feed.slot(0)?.taskId).toBe("t0");
    expect(feed.slot(60)).toBeUndefined();
  });

  it("loads a page in the middle directly", async () => {
    const { fn, calls } = fakeServer(1000);
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    feed.ensure(520, 540);
    await settle();
    expect(calls.map((c) => c.offset)).toEqual([0, 500]);
    expect(feed.slot(520)?.taskId).toBe("t520");
    expect(feed.slot(450)).toBeUndefined();
    expect(feed.allLoaded()).toBe(false);
  });

  it("puts hidden rows at the end of their page and drops them from the estimate", async () => {
    const { fn } = fakeServer(100, new Set([3, 4, 60]));
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    expect(feed.slot(3)?.taskId).toBe("t5");
    expect(feed.slot(48)).toBe(HIDDEN);
    expect(feed.slot(49)).toBe(HIDDEN);
    expect(feed.estimatedTotal()).toBe(98);
    feed.ensure(50, 100);
    await settle();
    expect(feed.estimatedTotal()).toBe(97);
    expect(feed.allLoaded()).toBe(true);
    expect(feed.loadedRows().length).toBe(97);
  });

  it("does not fetch a page twice while it is loading", async () => {
    const { fn } = fakeServer(200);
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    feed.ensure(60, 70);
    feed.ensure(55, 99);
    await settle();
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("refresh re-reads loaded pages and swaps them in together", async () => {
    let count = 200;
    const fn = vi.fn(async (offset: number, limit: number): Promise<TaskList> => {
      const slice = Array.from({ length: Math.max(0, Math.min(limit, count - offset)) }, (_, i) => offset + i);
      return { tasks: slice.map(task), hiddenCount: 0, total: count };
    });
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    feed.ensure(150, 160);
    await settle();
    count = 120; // rows were removed on the server
    await feed.refresh();
    expect(feed.total()).toBe(120);
    // Page 3 (slots 150+) no longer exists; page 0 was re-read.
    expect(feed.slot(150)).toBeUndefined();
    expect(fn.mock.calls.filter(([o]) => o === 0)).toHaveLength(2);
  });

  it("drops a response that arrives after a reset", async () => {
    let release!: (v: TaskList) => void;
    const slow = vi.fn(() => new Promise<TaskList>((r) => (release = r)));
    const feed = createTaskFeed(50);
    feed.reset(slow);
    feed.reset(null);
    release({ tasks: [task(1)], hiddenCount: 0, total: 1 });
    await settle();
    expect(feed.ready()).toBe(false);
  });

  it("does not re-run the effect that resets it", async () => {
    const { fn } = fakeServer(120);
    const feed = createTaskFeed(50);
    let runs = 0;
    const dispose = createRoot((d) => {
      createEffect(() => {
        runs++;
        feed.reset(fn);
        feed.ensure(0, 120);
      });
      return d;
    });
    await settle();
    await settle();
    dispose();
    expect(runs).toBe(1);
  });
});

describe("entry builder", () => {
  it("places group headers by slot before rows load", async () => {
    const { fn } = fakeServer(200, new Set(), [0, 120, 180]);
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    const groups = dueGroupStarts(feed.dueCounts(), ["Overdue", "Today", "Later", "No date"]);
    const entries = createEntryBuilder()(feed, groups);
    // Overdue is empty, so no header for it.
    const headers = entries.flatMap((e, i) => (e.kind === "header" ? [[e.label, i] as const] : []));
    expect(headers).toEqual([
      ["Today", 0],
      ["Later", 121],
      ["No date", 182],
    ]);
    expect(entries[121 + 1]).toMatchObject({ kind: "pending", slot: 120 });
    expect(slotRange(entries, 120, 125)).toEqual([119, 123]);
  });

  it("reuses row objects when a page lands", async () => {
    const { fn } = fakeServer(120);
    const feed = createTaskFeed(50);
    feed.reset(fn);
    await settle();
    const build = createEntryBuilder();
    const before = build(feed);
    feed.ensure(60, 70);
    await settle();
    const after = build(feed);
    expect(after[3]).toBe(before[3]);
    expect(after[60]).not.toBe(before[60]);
    expect(after[60].kind).toBe("task");
  });
});

describe("listAllTasks", () => {
  it("walks pages until the end", async () => {
    const { fn, calls } = fakeServer(MAX_PAGE_SIZE + 20, new Set([1]));
    const all = await listAllTasks((offset, limit) => fn(offset, limit, false));
    expect(all.length).toBe(MAX_PAGE_SIZE + 19);
    expect(calls.map((c) => c.offset)).toEqual([0, MAX_PAGE_SIZE]);
  });
});
