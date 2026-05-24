import type { Event, Member, Project, Task } from "./types";

export const members: Member[] = [
  { id: "tod",    name: "Tod Hansmann",    initials: "T", swatch: "a1", status: "active", doing: "reviewing the dashboard",          lastSeenLabel: "now" },
  { id: "lorna",  name: "Lorna Hansmann",  initials: "L", swatch: "a2", status: "active", doing: "editing 'Garden bed rotation'",    lastSeenLabel: "12m" },
  { id: "marcus", name: "Marcus Wei",      initials: "M", swatch: "a4", status: "active", doing: "commented on 'Barn rebuild'",       lastSeenLabel: "just now" },
  { id: "anya",   name: "Anya Berenson",   initials: "A", swatch: "a3", status: "active", doing: "updated retreat agenda",            lastSeenLabel: "5m" },
  { id: "sam",    name: "Sam Patel",       initials: "S", swatch: "a1", status: "away",   doing: "last seen working on tasks",        lastSeenLabel: "38m" },
];

export const tasks: Task[] = [
  { id: "t1", title: "Sweep the porch and shake the rugs",                              done: true,  tag: "house", due: "15m",         meta: "Lorna finished at 7:42", assignees: ["lorna"],          groupLabel: "today", estimateMinutes: 15 },
  { id: "t2", title: "Re-hang the rear gate — second hinge is loose",                   done: false, tag: "barn",  due: "by 11am",      meta: "tools in the side shed",  assignees: ["sam","marcus"],   groupLabel: "today", estimateMinutes: 45 },
  { id: "t3", title: "Sign for the lumber drop, stack it under the south overhang",     done: false, tag: "field", due: "1pm window",   meta: "Maple Yard, invoice #4118", assignees: ["tod"],          groupLabel: "today", estimateMinutes: 30 },
  { id: "t4", title: "Call Sue back about the well-pump quote",                         done: false, tag: "calls", due: "15m",          meta: "she's off after 4pm",     assignees: ["tod"],            groupLabel: "today", estimateMinutes: 15 },
  { id: "t5", title: "Confirm potluck dishes — Sam said pie, no one's claimed bread",   done: false, tag: "house", due: "before 5pm",   meta: "3 of 6 confirmed",        assignees: ["anya"],           groupLabel: "today", estimateMinutes: 10 },
  { id: "t6", title: "Walk the back fence with Sam — flag what needs replacing",        done: false, tag: "field", due: "Thu morning",  meta: "~1h",                     assignees: ["sam","tod"],      groupLabel: "week",  estimateMinutes: 60 },
  { id: "t7", title: "Quarterly check-in with the retreat committee",                   done: false, tag: "calls", due: "Fri 2pm",      meta: "send agenda Wed",         assignees: ["anya","marcus","tod"], groupLabel: "week", estimateMinutes: 60 },
];

const ORG_COLORS = {
  tod:    "#4f7ea0" as const, // admin / blue
  anya:   "#b88a5a" as const, // retreat / tan
  lorna:  "#6e9a58" as const, // outdoor / green
  marcus: "#a06448" as const, // facilities / rust
  sam:    "#6f7d88" as const, // ops / slate
};

export const events: Event[] = [
  { id: "e1",  title: "All-hands kickoff",      timeLabel: "09:00", startDate: "2026-05-01", endDate: "2026-05-01", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e2",  title: "Garden open day",        timeLabel: "—",     startDate: "2026-05-03", endDate: "2026-05-03", tone: "grass",   organizerColor: ORG_COLORS.lorna,  organizer: "lorna" },
  { id: "e3",  title: "Weekly standup",         timeLabel: "09:00", startDate: "2026-05-04", endDate: "2026-05-04", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e4",  title: "Retreat planning",       timeLabel: "15:00", startDate: "2026-05-05", endDate: "2026-05-05", tone: "heather", organizerColor: ORG_COLORS.anya,   organizer: "anya" },
  { id: "e5",  title: "Quarterly board review", timeLabel: "14:00", startDate: "2026-05-06", endDate: "2026-05-06", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e6",  title: "Vendor follow-up",       timeLabel: "17:00", startDate: "2026-05-06", endDate: "2026-05-06", tone: "moss",    organizerColor: ORG_COLORS.sam,    organizer: "sam" },
  { id: "e7",  title: "Lunch with neighbors",   timeLabel: "12:00", startDate: "2026-05-08", endDate: "2026-05-08", tone: "grass",   organizerColor: ORG_COLORS.lorna,  organizer: "lorna" },
  { id: "e8",  title: "Spring open house",      timeLabel: "—",     startDate: "2026-05-09", endDate: "2026-05-10", tone: "heather", organizerColor: ORG_COLORS.anya,   organizer: "anya" },
  { id: "e9",  title: "Weekly standup",         timeLabel: "09:00", startDate: "2026-05-11", endDate: "2026-05-11", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e10", title: "Vendor walk-through",    timeLabel: "11:00", startDate: "2026-05-11", endDate: "2026-05-11", tone: "moss",    organizerColor: ORG_COLORS.sam,    organizer: "sam" },
  { id: "e11", title: "Retreat committee lunch",timeLabel: "13:00", startDate: "2026-05-13", endDate: "2026-05-13", tone: "heather", organizerColor: ORG_COLORS.anya,   organizer: "anya" },
  { id: "e12", title: "Maintenance window",     timeLabel: "08:00", startDate: "2026-05-14", endDate: "2026-05-14", tone: "grass",   organizerColor: ORG_COLORS.marcus, organizer: "marcus" },
  { id: "e13", title: "Membership review",      timeLabel: "15:30", startDate: "2026-05-14", endDate: "2026-05-14", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e14", title: "Book club",              timeLabel: "19:00", startDate: "2026-05-17", endDate: "2026-05-17", tone: "sky",     organizerColor: "#7a9aa8",         organizer: "anya" },
  { id: "e15", title: "Weekly standup",         timeLabel: "09:00", startDate: "2026-05-18", endDate: "2026-05-18", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e16", title: "Standing dinner",        timeLabel: "18:30", startDate: "2026-05-18", endDate: "2026-05-18", tone: "heather", organizerColor: ORG_COLORS.anya,   organizer: "anya" },
  { id: "e17", title: "Backup window",          timeLabel: "23:00", startDate: "2026-05-18", endDate: "2026-05-18", tone: "moss",    organizerColor: ORG_COLORS.sam,    organizer: "sam" },
  { id: "e18", title: "Retreat dry-run",        timeLabel: "16:00", startDate: "2026-05-19", endDate: "2026-05-19", tone: "heather", organizerColor: ORG_COLORS.anya,   organizer: "anya" },
  { id: "e19", title: "Maple yard inventory",   timeLabel: "—",     startDate: "2026-05-20", endDate: "2026-05-22", tone: "moss",    organizerColor: ORG_COLORS.sam,    organizer: "sam" },
  { id: "e20", title: "Trail maintenance",      timeLabel: "10:00", startDate: "2026-05-23", endDate: "2026-05-23", tone: "grass",   organizerColor: ORG_COLORS.lorna,  organizer: "lorna" },
  { id: "e21", title: "Weekly standup",         timeLabel: "09:00", startDate: "2026-05-25", endDate: "2026-05-25", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e22", title: "Quarterly review",       timeLabel: "10:00", startDate: "2026-05-27", endDate: "2026-05-27", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
  { id: "e23", title: "Barn punch-list",        timeLabel: "14:00", startDate: "2026-05-27", endDate: "2026-05-27", tone: "moss",    organizerColor: ORG_COLORS.marcus, organizer: "marcus" },
  { id: "e24", title: "Board sync",             timeLabel: "15:00", startDate: "2026-05-28", endDate: "2026-05-28", tone: "ocean",   organizerColor: ORG_COLORS.tod,    organizer: "tod" },
];

export const projects: Project[] = [
  {
    id: "p1", slug: "spring-planting", category: "Operations",
    title: "Spring planting", swatch: "r1", progress: 62,
    blurb: "Beds prepped, seeds in by Friday if weather holds.",
    members: ["lorna","tod","sam","marcus"], owners: ["lorna"],
    startedLabel: "April 2", dueLabel: "June 1",
  },
  {
    id: "p2", slug: "retreat-weekend", category: "Events",
    title: "Retreat weekend", swatch: "r2", progress: 34,
    blurb: "Eight guests in June. Menu, rooms, transport.",
    members: ["anya","tod","lorna","marcus","sam","anya"], owners: ["anya"],
    startedLabel: "March 14", dueLabel: "June 14",
  },
  {
    id: "p3", slug: "barn-rebuild", category: "Facilities",
    title: "Barn rebuild", swatch: "r3", progress: 78,
    blurb: "Frame is up. Cladding ordered, arrives next week.",
    members: ["marcus","sam","tod"], owners: ["marcus","sam"],
    startedLabel: "March 4", dueLabel: "June 21",
    sharedWith: "Retreat committee (read)",
    description:
      "Frame is up, cladding ordered, doors hung. Two weekends of finish work remain, weather permitting. The committee has approved a two-week extension to the original open-house date.",
    milestones: [
      { label: "Old frame removed, footings poured", when: "Mar",         state: "done"    },
      { label: "Skeleton up, roof on",                when: "Apr",         state: "done"    },
      { label: "Cladding and doors",                  when: "May · current", state: "current" },
      { label: "Interior and lighting",               when: "Jun",         state: "future"  },
      { label: "Open house",                          when: "Jun 21",      state: "future"  },
    ],
  },
  {
    id: "p4", slug: "q2-numbers", category: "Finance",
    title: "Q2 numbers", swatch: "r4", progress: 46,
    blurb: "Quiet quarter, mostly. Two invoices still out.",
    members: ["tod","anya"], owners: ["tod"],
    startedLabel: "April 1", dueLabel: "June 30",
  },
];

/** Members by id, for quick lookup */
export const memberById = new Map(members.map((m) => [m.id, m]));
export const projectBySlug = new Map(projects.map((p) => [p.slug, p]));
