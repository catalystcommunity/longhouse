import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, waitFor } from "@solidjs/testing-library";
import { createMemoryHistory, MemoryRouter, Route } from "@solidjs/router";
import type { Event as ApiEvent, EventListRequest } from "@longhouse/client";
import { CalendarPage } from "./Calendar";

const { listEvents, listLocalEvents, acceptEvent, recoverEdit, updateEvent, getEvent } = vi.hoisted(() => ({
  listEvents: vi.fn<(req: EventListRequest) => Promise<ApiEvent[]>>(),
  listLocalEvents: vi.fn<() => Promise<ApiEvent[]>>(),
  acceptEvent: vi.fn(),
  recoverEdit: vi.fn(),
  updateEvent: vi.fn(),
  getEvent: vi.fn(),
}));

vi.mock("~/data/clients", () => ({
  eventClient: {
    listEvents,
    listLocalEvents,
    acceptEvent,
    recoverEdit,
    updateEvent,
    getEvent,
    getCalendarView: async () => ({ subscriptions: [] }),
  },
  memberClient: { listMembers: async () => [] },
  calendarClient: { getStatus: async () => ({ available: false, enabled: false, ready: false, connected: false }) },
}));
vi.mock("~/stores/auth", () => ({
  currentMemberId: () => "member-1",
  hasRole: () => false,
  useCurrentHouseId: () => () => "house-1",
}));
vi.mock("~/components/AuthGate", () => ({ AuthGate: (props: { children: unknown }) => props.children }));

beforeEach(() => {
  listEvents.mockReset().mockResolvedValue([]);
  listLocalEvents.mockReset().mockResolvedValue([]);
  acceptEvent.mockReset().mockResolvedValue({});
  recoverEdit.mockReset().mockResolvedValue({});
  updateEvent.mockReset().mockResolvedValue({});
  getEvent.mockReset().mockRejectedValue(new Error("Event not found"));
});

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

describe("owner acceptance", () => {
  const proposal: ApiEvent = {
    eventId: "proposal-1", houseId: "house-1", ownerMemberId: "member-1",
    title: "Assigned meeting", description: "Review these saved details", createdAt: "", updatedAt: "",
    startsAt: "2026-11-06T15:00:00Z", endsAt: "2026-11-06T16:00:00Z",
    calendarState: "pending", revision: 7,
  };
  const mount = () => render(() => <MemoryRouter><Route path="/" component={CalendarPage} /></MemoryRouter>);

  it("accepts only the saved revision after owner review", async () => {
    listLocalEvents.mockResolvedValue([proposal]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Assigned meeting" }));
    expect(screen.getByDisplayValue("Review these saved details")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Accept event" }));
    await waitFor(() => expect(acceptEvent).toHaveBeenCalledExactlyOnceWith({ eventId: "proposal-1", revision: 7 }));
  });

  it("requires saving changes before acceptance", async () => {
    listLocalEvents.mockResolvedValue([proposal]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Assigned meeting" }));
    fireEvent.input(screen.getByDisplayValue("Assigned meeting"), { target: { value: "Changed proposal" } });
    expect(screen.getByRole("button", { name: "Accept event" })).toBeDisabled();
    expect(acceptEvent).not.toHaveBeenCalled();
  });

  it("does not offer acceptance to another member", async () => {
    listLocalEvents.mockResolvedValue([{ ...proposal, ownerMemberId: "member-2" }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Assigned meeting" }));
    expect(screen.queryByRole("button", { name: "Accept event" })).not.toBeInTheDocument();
  });

  it("locks an uncertain publication against edit and cancellation", async () => {
    listLocalEvents.mockResolvedValue([{ ...proposal, calendarState: "publishing", publicationError: "Publication did not finish." }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Assigned meeting" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Accept event" })).not.toBeInTheDocument();
    fireEvent.input(screen.getByDisplayValue("Assigned meeting"), { target: { value: "Unsaved edit" } });
    fireEvent.click(screen.getByRole("button", { name: "Retry publication" }));
    await waitFor(() => expect(acceptEvent).toHaveBeenCalledExactlyOnceWith({ eventId: "proposal-1", revision: 7 }));
  });

  it("does not let another member retry publication", async () => {
    listLocalEvents.mockResolvedValue([{ ...proposal, ownerMemberId: "member-2", calendarState: "publishing" }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Assigned meeting" }));
    expect(screen.queryByRole("button", { name: "Retry publication" })).not.toBeInTheDocument();
  });
});

describe("remote series edits", () => {
  const occurrence: ApiEvent = {
    eventId: "occurrence-1", houseId: "house-1", ownerMemberId: "member-1",
    title: "Weekly meeting", createdAt: "", updatedAt: "",
    startsAt: "2026-11-17T15:00:00Z", endsAt: "2026-11-17T16:00:00Z",
    calendarState: "remote", revision: 4, recurrenceRootEventId: "root-1",
    recurrenceRule: "FREQ=WEEKLY", timezone: "America/Denver",
  };
  const mount = () => render(() => <MemoryRouter><Route path="/" component={CalendarPage} /></MemoryRouter>);

  it("sends this-and-future edits with the reviewed revision", async () => {
    listLocalEvents.mockResolvedValue([occurrence]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
    fireEvent.change(screen.getByRole("combobox", { name: "Apply changes to" }), { target: { value: "this_and_future" } });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(updateEvent).toHaveBeenCalledWith(expect.objectContaining({
      eventId: "occurrence-1", editScope: "this_and_future", revision: 4,
    })));
  });

  it("shows recovery when a split save has an uncertain result", async () => {
    listLocalEvents.mockResolvedValue([occurrence]);
    updateEvent.mockRejectedValueOnce(new Error("Calendar edit is not confirmed"));
    getEvent.mockResolvedValue({ ...occurrence, editPendingMemberId: "member-1", publicationError: "Retry the saved edit." });
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
    fireEvent.change(screen.getByRole("combobox", { name: "Apply changes to" }), { target: { value: "this_and_future" } });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByRole("button", { name: "Retry saved edit" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("retries the saved edit without sending changed form content", async () => {
    listLocalEvents.mockResolvedValue([{ ...occurrence, editPendingMemberId: "member-1", publicationError: "Retry the saved edit." }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    fireEvent.input(screen.getByDisplayValue("Weekly meeting"), { target: { value: "Unsaved change" } });
    fireEvent.click(screen.getByRole("button", { name: "Retry saved edit" }));
    await waitFor(() => expect(recoverEdit).toHaveBeenCalledExactlyOnceWith("occurrence-1"));
    expect(updateEvent).not.toHaveBeenCalled();
  });

  it("keeps another member's pending edit locked", async () => {
    listLocalEvents.mockResolvedValue([{ ...occurrence, editPendingMemberId: "member-2" }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
    expect(screen.queryByRole("button", { name: "Retry saved edit" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("keeps an unmatched occurrence readable without allowing edits", async () => {
    listLocalEvents.mockResolvedValue([{ ...occurrence, calendarUnbound: true, publicationError: "This occurrence has no match after the series edit." }]);
    const screen = mount();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
    expect(screen.getByRole("alert")).toHaveTextContent("no match");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Retry saved edit" })).not.toBeInTheDocument();
  });
});
