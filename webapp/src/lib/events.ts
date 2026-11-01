import type { Event as ApiEvent, EventListRequest } from "@longhouse/client";
import { startOfMonFirstWeek } from "./month";

export type CalendarViewMode = "month" | "week" | "3day" | "day";
export const VIEW_DAYS = { week: 7, "3day": 3, day: 1 } as const;

const EVENT_PAGE_SIZE = 500;

// Keep pagination in one place so the calendar and dashboard show all
// matching events, including ranges with more than one full page.
export async function loadEvents(
  client: { listEvents(req: EventListRequest): Promise<ApiEvent[]> },
  request: Omit<EventListRequest, "limit" | "offset">,
): Promise<ApiEvent[]> {
  const events: ApiEvent[] = [];
  for (let offset = 0; ; offset += EVENT_PAGE_SIZE) {
    const page = await client.listEvents({ ...request, limit: EVENT_PAGE_SIZE, offset });
    events.push(...page);
    if (page.length < EVENT_PAGE_SIZE) return events;
  }
}

export function startOfRange(anchor: Date, view: Exclude<CalendarViewMode, "month">): Date {
  const start = new Date(anchor);
  start.setHours(0, 0, 0, 0);
  return view === "week" ? startOfMonFirstWeek(start) : start;
}

// Use local midnights and calendar days to include daylight saving changes.
// The month view includes all 42 cells, including days in adjacent months.
export function viewRange(view: CalendarViewMode, anchor: Date): { start: Date; end: Date } {
  const start = view === "month"
    ? startOfMonFirstWeek(new Date(anchor.getFullYear(), anchor.getMonth(), 1))
    : startOfRange(anchor, view);
  const end = new Date(start);
  end.setDate(start.getDate() + (view === "month" ? 42 : VIEW_DAYS[view]));
  return { start, end };
}
