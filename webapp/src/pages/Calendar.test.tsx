import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, waitFor } from "@solidjs/testing-library";
import { createMemoryHistory, MemoryRouter, Route } from "@solidjs/router";
import type { Event as ApiEvent, EventListRequest } from "@longhouse/client";
import { CalendarPage } from "./Calendar";

const { listEvents } = vi.hoisted(() => ({ listEvents: vi.fn<(req: EventListRequest) => Promise<ApiEvent[]>>() }));

vi.mock("~/data/clients", () => ({
  eventClient: {
    listEvents,
    getCalendarView: async () => ({ subscriptions: [] }),
  },
  memberClient: { listMembers: async () => [] },
}));
vi.mock("~/stores/auth", () => ({
  currentMemberId: () => "member-1",
  useCurrentHouseId: () => () => "house-1",
}));
vi.mock("~/components/AuthGate", () => ({ AuthGate: (props: { children: unknown }) => props.children }));

beforeEach(() => { listEvents.mockReset().mockResolvedValue([]); });

describe("calendar event loading", () => {
  it("reloads the range when the user changes dates or views", async () => {
    const history = createMemoryHistory();
    history.set({ value: "/calendar?date=2026-11-01&view=day" });
    const screen = render(() => (
      <MemoryRouter history={history}>
        <Route path="/calendar" component={CalendarPage} />
      </MemoryRouter>
    ));
    const expectRange = async (start: Date, end: Date) => {
      await waitFor(() => expect(listEvents).toHaveBeenLastCalledWith({
        houseId: "house-1", rangeStart: start.toISOString(), rangeEnd: end.toISOString(), limit: 500, offset: 0,
      }));
    };
    await expectRange(new Date(2026, 10, 1), new Date(2026, 10, 2));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await expectRange(new Date(2026, 10, 2), new Date(2026, 10, 3));
    fireEvent.click(screen.getByRole("button", { name: "Week" }));
    await expectRange(new Date(2026, 10, 2), new Date(2026, 10, 9));
    fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    await expectRange(new Date(2026, 9, 26), new Date(2026, 10, 2));
  });

  it("opens a linked event after all pages of its date range load", async () => {
    const rows: ApiEvent[] = Array.from({ length: 501 }, (_, i) => ({
      eventId: `event-${i}`, houseId: "house-1", ownerMemberId: "member-1",
      title: `Event ${i}`, createdAt: "", updatedAt: "",
      startsAt: new Date(2026, 10, 1, 10).toISOString(),
      endsAt: new Date(2026, 10, 1, 11).toISOString(),
    }));
    listEvents.mockImplementation(async (req) => req.rangeStart === new Date(2026, 10, 1).toISOString()
      ? rows.slice(Number(req.offset), Number(req.offset) + Number(req.limit))
      : []);
    const history = createMemoryHistory();
    history.set({ value: "/calendar?date=2026-11-01&view=day&event=event-500" });
    const screen = render(() => (
      <MemoryRouter history={history}>
        <Route path="/calendar" component={CalendarPage} />
      </MemoryRouter>
    ));
    await waitFor(() => expect(screen.getByDisplayValue("Event 500")).toBeInTheDocument());
    await waitFor(() => expect(history.get()).not.toContain("event="));
    expect(listEvents.mock.calls.some(([req]) => req.offset === 500)).toBe(true);
  });
});
