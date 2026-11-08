import { defineView } from "~/ui/dom";
import { computed } from "~/ui/state";
import { type Content, live, el } from "~/ui/dom";
import { VirtualList } from "~/components/VirtualList";
import { createEntryBuilder, estimateTaskEntry, slotRange, taskEntryKey, type GroupStart, type TaskFeed, } from "~/lib/taskFeed";
import type { Task } from "@longhouse/client";
/**
 * A task feed drawn as a virtual list. Rows that are not loaded show as
 * skeleton rows at the estimated height; the pages under the viewport load
 * directly, so a jump into the middle of a long list fills in place. Rows
 * the caller cannot read take no space.
 */
const renderTaskFeedList = (props: {
    feed: TaskFeed;
    /** Group headers by slot (for a due-ordered open list). */
    groups?: GroupStart[];
    renderTask: (task: Task) => Content;
}) => {
    const build = createEntryBuilder();
    const entries = computed(() => build(props.feed, props.groups ?? []));
    return (VirtualList({
        get "entries"() {
            return entries();
        },
        get "keyOf"() {
            return taskEntryKey;
        },
        get "estimate"() {
            return estimateTaskEntry;
        },
        "render": (e) => {
            switch (e.kind) {
                case "header":
                    return el("div", {
                        "class": "group-lbl",
                        get children() {
                            return live(() => e.label);
                        }
                    });
                case "task":
                    return props.renderTask(e.task);
                case "pending":
                    return (el("div", {
                        "class": "task task-skel",
                        "aria-hidden": "true",
                        get children() {
                            return el("span", {
                                "class": "skel-bar"
                            });
                        }
                    }));
                case "hidden":
                    return null;
            }
        },
        "onRange": (start, end) => {
            const r = slotRange(entries(), start, end);
            if (r)
                props.feed.ensure(r[0], r[1]);
        }
    }));
};

export const TaskFeedList = defineView("longhouse-task-feed", renderTaskFeedList);
