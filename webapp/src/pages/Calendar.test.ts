
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, waitFor } from "~/ui/testing";
import { createTestHistory } from "~/ui/testing";
import type { Event as ApiEvent, EventListRequest } from "@longhouse/client";
import { fromEventCbor, toEventCbor } from "@longhouse/client";
import { CalendarPage } from "./Calendar";
const { listEvents, listLocalEvents, acceptEvent, recoverEdit, updateEvent, getEvent, createEvent } = vi.hoisted(() => ({
    listEvents: vi.fn<(req: EventListRequest) => Promise<ApiEvent[]>>(),
    listLocalEvents: vi.fn<() => Promise<ApiEvent[]>>(),
    acceptEvent: vi.fn(),
    recoverEdit: vi.fn(),
    updateEvent: vi.fn(),
    getEvent: vi.fn(),
    createEvent: vi.fn(),
}));
vi.mock("~/data/clients", () => ({
    eventClient: {
        listEvents,
        listLocalEvents,
        acceptEvent,
        recoverEdit,
        updateEvent,
        getEvent,
        createEvent,
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
vi.mock("~/components/AuthGate", () => ({ AuthGate: (props: {
        children: unknown;
    }) => props.children }));
beforeEach(() => {
    listEvents.mockReset().mockResolvedValue([]);
    listLocalEvents.mockReset().mockResolvedValue([]);
    acceptEvent.mockReset().mockResolvedValue({});
    recoverEdit.mockReset().mockResolvedValue({});
    updateEvent.mockReset().mockResolvedValue({});
    getEvent.mockReset().mockRejectedValue(new Error("Event not found"));
    createEvent.mockReset().mockResolvedValue({});
});
describe("recurrence on the wire", () => {
    it("creates an event without sending an empty frequency enum", async () => {
        const screen = render(() => CalendarPage());
        fireEvent.click(await screen.findByRole("button", {name: "New event"}));
        fireEvent.input(screen.getByRole("textbox", {name: "Title"}), {target: {value: "One meeting"}});
        fireEvent.click(screen.getByRole("button", {name: "Create event"}));
        await waitFor(() => expect(createEvent).toHaveBeenCalledOnce());
        const body = createEvent.mock.calls[0][0];
        const wire = fromEventCbor(toEventCbor({...body, eventId: "", createdAt: "", updatedAt: ""}));
        expect(wire.recurrenceFreq).toBeUndefined();
        expect(wire.clearRecurrence).toBe(true);
    });
});
describe("calendar event loading", () => {
    it("reloads the range when the user changes dates or views", async () => {
        const history = createTestHistory();
        history.set({ value: "/calendar?date=2026-11-01&view=day" });
        const screen = render(() => (CalendarPage()));
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
        const history = createTestHistory();
        history.set({ value: "/calendar?date=2026-11-01&view=day&event=event-500" });
        const screen = render(() => (CalendarPage()));
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
    const mount = () => render(() => CalendarPage());
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
    const mount = () => render(() => CalendarPage());
    it("sends this-and-future edits with the reviewed revision", async () => {
        listLocalEvents.mockResolvedValue([occurrence]);
        const screen = mount();
        fireEvent.click(await screen.findByRole("button", { name: "Weekly meeting" }));
        fireEvent.change(screen.getByRole("combobox", { name: "Apply changes to" }), { target: { value: "this_and_future" } });
        fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
        await waitFor(() => expect(updateEvent).toHaveBeenCalledWith(expect.objectContaining({
            eventId: "occurrence-1", editScope: "this_and_future", revision: 4,
            recurrenceRule: "FREQ=WEEKLY", clearRecurrence: false,
        })));
    });
    it("clears a remote rule only when the owner removes it", async () => {
        listLocalEvents.mockResolvedValue([occurrence]);
        const screen = mount();
        fireEvent.click(await screen.findByRole("button", {name: "Weekly meeting"}));
        fireEvent.input(screen.getByDisplayValue("FREQ=WEEKLY"), {target: {value: ""}});
        fireEvent.click(screen.getByRole("button", {name: "Save changes"}));
        await waitFor(() => expect(updateEvent).toHaveBeenCalledWith(expect.objectContaining({
            recurrenceRule: "", clearRecurrence: true,
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
