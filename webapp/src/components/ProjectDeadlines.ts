import { cell, cleanup, query } from "~/ui/state";
import { el, live, Repeat, When } from "~/ui/dom";
import { calendarClient } from "~/data/clients";

export function ProjectDeadlines(props: { projectId: string; revision: number; category?: string }) {
    const now = new Date();
    const [month, setMonth] = cell(new Date(now.getFullYear(), now.getMonth(), 1));
    const [result, { refetch }] = query(() => ({
        projectId: props.projectId,
        rangeStart: month().toISOString(),
        rangeEnd: new Date(month().getFullYear(), month().getMonth() + 1, 1).toISOString(),
        revision: props.revision,
        category: props.category,
    }), ({ projectId, rangeStart, rangeEnd }) => calendarClient.listProjectDeadlines({ projectId, rangeStart, rangeEnd }));
    const timer = setInterval(() => {
        if (!document.hidden && !result.loading && result()?.enabled) void refetch();
    }, 15000);
    cleanup(() => clearInterval(timer));
    const move = (delta: number) => setMonth(new Date(month().getFullYear(), month().getMonth() + delta, 1));
    return el("section", {
        "aria-label": "Project deadlines",
        style: "padding:24px;border-top:1px solid var(--line)",
        children: [
            el("h3", { children: "Deadline calendar" }),
            el("p", { children: live(() => `${result()?.category ?? props.category ?? "Uncategorized"} · This project's task deadlines`) }),
            el("div", { style: "display:flex;align-items:center;gap:12px;flex-wrap:wrap", children: [
                el("button", { type: "button", "aria-label": "Previous month", onClick: () => move(-1), children: "←" }),
                el("strong", { children: live(() => month().toLocaleDateString(undefined, { month: "long", year: "numeric" })) }),
                el("button", { type: "button", "aria-label": "Next month", onClick: () => move(1), children: "→" }),
                el("button", { type: "button", onClick: () => refetch(), children: "Refresh" }),
            ] }),
            el("p", { role: "status", children: live(() => {
                if (result.error) return "The deadline calendar could not be loaded. Select Refresh to try again.";
                if (result.loading && !result()) return "Loading deadlines…";
                if (!result()?.enabled) return "Enable the house calendar in Calendar settings to show deadlines here.";
                if (result()?.lastError) return result()!.lastError!;
                if (result()?.pending) return "Updating deadlines. Recent changes will appear after the update.";
                return "No reminders. Times use your local time zone.";
            }) }),
            When({ get when() { return result()?.enabled && !result.error && !result.loading; }, get children() {
                return [
                    Repeat({ get each() { return result()?.deadlines ?? []; }, children: deadline => el("div", {
                        style: "display:flex;gap:16px;flex-wrap:wrap;padding:10px 0;border-bottom:1px solid var(--line)",
                        children: [
                            el("time", { datetime: deadline.dueAt, children: new Date(deadline.dueAt).toLocaleString(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }) }),
                            el("span", { children: deadline.title }),
                        ],
                    }) }),
                    When({ get when() { return !result.loading && !result()?.pending && !result()?.lastError && result()?.deadlines.length === 0; }, children: el("p", { children: "No task deadlines this month." }) }),
                ];
            } }),
        ],
    });
}
