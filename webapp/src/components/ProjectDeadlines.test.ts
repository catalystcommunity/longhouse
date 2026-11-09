import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "~/ui/testing";
import { ProjectDeadlines } from "./ProjectDeadlines";
import type { ProjectDeadlineList } from "@longhouse/client";

const { list } = vi.hoisted(() => ({ list: vi.fn() }));
vi.mock("~/data/clients", () => ({ calendarClient: { listProjectDeadlines: list } }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });
const ready: ProjectDeadlineList = { enabled: true, pending: false, category: "Garden", deadlines: [] };

describe("Project deadline calendar", () => {
    it("shows remote deadlines, changes months, and releases polling", async () => {
        const spy = vi.spyOn(window, "clearInterval");
        list.mockResolvedValue({ ...ready, deadlines: [{ taskId: "task", title: "Plant trees", dueAt: new Date().toISOString() }] });
        const view = render(() => ProjectDeadlines({ projectId: "project", revision: 0 }));
        await waitFor(() => expect(view.getByText("Plant trees")).toBeInTheDocument());
        expect(view.getByText("No reminders. Times use your local time zone.")).toBeInTheDocument();
        const first = list.mock.calls.at(-1)![0];
        fireEvent.click(view.getByRole("button", { name: "Next month" }));
        await waitFor(() => expect(list.mock.calls.at(-1)![0].rangeStart).toBe(first.rangeEnd));
        view.unmount();
        expect(spy).toHaveBeenCalled();
        spy.mockRestore();
    });

    it("shows pending updates without claiming the month is empty", async () => {
        list.mockResolvedValue({ ...ready, pending: true });
        const view = render(() => ProjectDeadlines({ projectId: "project", revision: 0 }));
        await waitFor(() => expect(view.getByText(/Updating deadlines/)).toBeInTheDocument());
        expect(view.queryByText("No task deadlines this month.")).not.toBeInTheDocument();
    });

    it("shows a remote error and permits a retry", async () => {
        list.mockRejectedValueOnce(new Error("offline")).mockResolvedValue(ready);
        const view = render(() => ProjectDeadlines({ projectId: "project", revision: 0 }));
        await waitFor(() => expect(view.getByText(/could not be loaded/)).toBeInTheDocument());
        fireEvent.click(view.getByRole("button", { name: "Refresh" }));
        await waitFor(() => expect(view.getByText("No task deadlines this month.")).toBeInTheDocument());
    });
});
