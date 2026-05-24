/**
 * Domain types. These are the client's view of Longhouse data —
 * they don't have to match the wire format 1:1; the transport layer
 * is responsible for translating between CBOR-tagged payloads and these.
 */

export type ID = string;
export type ISODateTime = string; // RFC3339
export type HexColor = `#${string}`;

export type ThemeTone = "ocean" | "grass" | "sky" | "heather" | "moss";

export interface Member {
  id: ID;
  name: string;
  initials: string;
  /** which gradient swatch the avatar should use */
  swatch: "a1" | "a2" | "a3" | "a4";
  /** active status — drives the corner dot */
  status: "active" | "away" | "offline";
  doing?: string;        // "editing 'Garden bed rotation'"
  lastSeenLabel?: string; // "12m", "just now"
}

export interface Task {
  id: ID;
  title: string;
  done: boolean;
  due?: string;          // "by 11am", "Thu morning" — display string for the comp
  tag?: "field" | "house" | "calls" | "barn";
  meta?: string;         // free-form right-side meta ("tools in the side shed")
  assignees: ID[];       // member ids
  estimateMinutes?: number;
  groupLabel: "today" | "week" | "later";
}

export interface Event {
  id: ID;
  title: string;
  /** display string the chip uses for time, e.g. "09:00" or "—" for all-day */
  timeLabel: string;
  /** start date in YYYY-MM-DD */
  startDate: string;
  /** end date (inclusive) — same as startDate for single-day events */
  endDate: string;
  /** which theme palette tone the event should use when colors aren't honored */
  tone: ThemeTone;
  /** organizer's suggested color — a suggestion the calendar may or may not honor */
  organizerColor: HexColor;
  organizer: ID;
  location?: string;
  attendees?: ID[];
}

export interface ProjectMilestone {
  label: string;
  when: string;
  state: "done" | "current" | "future";
}

export interface Project {
  id: ID;
  slug: string;
  category: string;   // "Operations", "Events", "Facilities", "Finance"
  title: string;
  blurb: string;
  swatch: "r1" | "r2" | "r3" | "r4";
  progress: number;   // 0–100
  members: ID[];
  owners: ID[];
  startedLabel: string;
  dueLabel: string;
  sharedWith?: string;
  description?: string;
  milestones?: ProjectMilestone[];
}
