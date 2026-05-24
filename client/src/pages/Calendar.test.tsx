import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@solidjs/testing-library";
import { CalendarPage } from "./Calendar";
import { RepoProvider } from "~/data/RepoContext";
import type { Repo } from "~/data/repo";
import type { Event } from "~/data/types";

const events: Event[] = [
  {
    id: "standup",
    title: "Weekly standup",
    timeLabel: "09:00",
    startDate: "2026-05-18",
    endDate: "2026-05-18",
    tone: "ocean",
    organizerColor: "#4f7ea0",
    organizer: "tod",
  },
  {
    id: "openhouse",
    title: "Spring open house",
    timeLabel: "—",
    startDate: "2026-05-09",
    endDate: "2026-05-10",
    tone: "heather",
    organizerColor: "#b88a5a",
    organizer: "anya",
  },
];

const fakeRepo = (): Repo => ({
  listTasks: async () => [],
  listEvents: async () => events,
  listMembers: async () => [],
  getMember: async () => undefined,
  listProjects: async () => [],
  getProject: async () => undefined,
});

const renderCalendar = () =>
  render(() => (
    <RepoProvider repo={fakeRepo()}>
      <CalendarPage />
    </RepoProvider>
  ));

describe("CalendarPage", () => {
  it("renders single-day and multi-day events from the repo", async () => {
    const { container } = renderCalendar();
    // wait for the resource to resolve and chips to render
    await screen.findByText("Weekly standup");
    expect(screen.getByText("Spring open house")).toBeInTheDocument();

    // the multi-day event is a span bar, the single-day is a chip
    expect(container.querySelector(".evt-span")).toBeTruthy();
    expect(container.querySelector(".evt")).toBeTruthy();
  });

  it("defaults to NOT honoring organizer colors", async () => {
    const { container } = renderCalendar();
    await screen.findByText("Weekly standup");
    expect(container.querySelector(".cal")!.getAttribute("data-honor")).toBe("false");
  });

  it("flips data-honor when the toggle is clicked", async () => {
    const { container } = renderCalendar();
    await screen.findByText("Weekly standup");

    const checkbox = container.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
    fireEvent.click(checkbox);

    expect(container.querySelector(".cal")!.getAttribute("data-honor")).toBe("true");
  });

  it("carries the organizer's suggested color on each event for the honor path", async () => {
    const { container } = renderCalendar();
    await screen.findByText("Weekly standup");

    const standup = [...container.querySelectorAll<HTMLElement>(".evt")].find(
      (e) => e.textContent?.includes("Weekly standup"),
    )!;
    expect(standup.getAttribute("data-tone")).toBe("ocean");
    // the suggested color is exposed as a CSS var the honor-mode CSS reads
    expect(standup.getAttribute("style")).toContain("#4f7ea0");
  });
});
