import { createMemo, type JSX } from "solid-js";
import { VirtualList } from "~/components/VirtualList";
import {
  createEntryBuilder,
  estimateTaskEntry,
  slotRange,
  taskEntryKey,
  type GroupStart,
  type TaskFeed,
} from "~/lib/taskFeed";
import type { Task } from "@longhouse/client";

/**
 * A task feed drawn as a virtual list. Rows that are not loaded show as
 * skeleton rows at the estimated height; the pages under the viewport load
 * directly, so a jump into the middle of a long list fills in place. Rows
 * the caller cannot read take no space.
 */
export const TaskFeedList = (props: {
  feed: TaskFeed;
  /** Group headers by slot (for a due-ordered open list). */
  groups?: GroupStart[];
  renderTask: (task: Task) => JSX.Element;
}) => {
  const build = createEntryBuilder();
  const entries = createMemo(() => build(props.feed, props.groups ?? []));
  return (
    <VirtualList
      entries={entries()}
      keyOf={taskEntryKey}
      estimate={estimateTaskEntry}
      render={(e) => {
        switch (e.kind) {
          case "header":
            return <div class="group-lbl">{e.label}</div>;
          case "task":
            return props.renderTask(e.task);
          case "pending":
            return (
              <div class="task task-skel" aria-hidden="true">
                <span class="skel-bar" />
              </div>
            );
          case "hidden":
            return null;
        }
      }}
      onRange={(start, end) => {
        const r = slotRange(entries(), start, end);
        if (r) props.feed.ensure(r[0], r[1]);
      }}
    />
  );
};
