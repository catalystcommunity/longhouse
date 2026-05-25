/**
 * Pure derivation helpers — UI flavor extracted from API primitives. No
 * fetches, no signals; just deterministic functions the pages can call to
 * compute display strings, palette swatches, group labels, and similar.
 *
 * Keeping these out of the data layer means we can audit them (and unit-
 * test them) independently of the network path.
 */

import type { Member, Task, Event as ApiEvent } from "~/api/types.gen";

// ---- People ------------------------------------------------------------

/** Best human-readable label for a member, in priority order. */
export const displayName = (m: { displayName?: string; linkkeysUserId?: string; memberId?: string }): string =>
  m.displayName?.trim() || m.linkkeysUserId || m.memberId || "?";

/** Single-letter initial for an avatar bubble. */
export const initial = (m: { displayName?: string; linkkeysUserId?: string; memberId?: string }): string => {
  const name = displayName(m);
  const match = name.match(/[A-Za-z0-9]/);
  return (match?.[0] ?? "?").toUpperCase();
};

export type Swatch = "a1" | "a2" | "a3" | "a4";

/** Deterministic swatch derived from member id. Same input → same color
 *  across pages and sessions. */
export const memberSwatch = (id: string): Swatch => {
  const variants: Swatch[] = ["a1", "a2", "a3", "a4"];
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) | 0;
  return variants[Math.abs(h) % variants.length];
};

/** Was the member active recently? lastSeenAt within 15min = active, within
 *  the day = away, else offline. lastSeenAt missing → offline. */
export type MemberStatus = "active" | "away" | "offline";

export const memberStatus = (m: { lastSeenAt?: string }, now = Date.now()): MemberStatus => {
  if (!m.lastSeenAt) return "offline";
  const t = Date.parse(m.lastSeenAt);
  if (Number.isNaN(t)) return "offline";
  const delta = now - t;
  if (delta < 15 * 60 * 1000) return "active";
  if (delta < 24 * 60 * 60 * 1000) return "away";
  return "offline";
};

/** "12m" / "2h" / "3d" / "just now" style ago strings. */
export const lastSeenLabel = (m: { lastSeenAt?: string }, now = Date.now()): string | undefined => {
  if (!m.lastSeenAt) return undefined;
  const t = Date.parse(m.lastSeenAt);
  if (Number.isNaN(t)) return undefined;
  const sec = Math.max(0, Math.floor((now - t) / 1000));
  if (sec < 60) return "just now";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h`;
  return `${Math.floor(hr / 24)}d`;
};

// ---- Tasks ------------------------------------------------------------

export type TaskGroup = "today" | "week" | "later" | "overdue" | "noDate";

/** Bucket a task into today/this-week/later based on its due_at vs now.
 *  Tasks with no due_at land in "noDate". */
export const taskGroup = (t: Task, now = new Date()): TaskGroup => {
  if (!t.dueAt) return "noDate";
  const due = new Date(t.dueAt);
  if (Number.isNaN(due.getTime())) return "noDate";
  const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const endToday = startToday + 24 * 3600 * 1000;
  const endWeek = startToday + 7 * 24 * 3600 * 1000;
  const ts = due.getTime();
  if (ts < startToday) return "overdue";
  if (ts < endToday) return "today";
  if (ts < endWeek) return "week";
  return "later";
};

/** True when the task's status is done or cancelled. */
export const isTaskClosed = (t: Task): boolean => t.status === "done" || t.status === "cancelled";

/** Display string for a due timestamp ("by 11am", "Thu morning"-style is the
 *  comp's flavour; for real data we use compact wall-clock formats). */
export const dueLabel = (t: Task, now = new Date()): string | undefined => {
  if (!t.dueAt) return undefined;
  const due = new Date(t.dueAt);
  if (Number.isNaN(due.getTime())) return undefined;
  const group = taskGroup(t, now);
  if (group === "today") {
    return due.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
  }
  if (group === "week") {
    return due.toLocaleDateString(undefined, { weekday: "short" });
  }
  return due.toLocaleDateString(undefined, { month: "short", day: "numeric" });
};

// ---- Events -----------------------------------------------------------

export type Tone = "ocean" | "grass" | "sky" | "heather" | "moss";

/** Deterministic palette tone from organizer member id. Drives the
 *  per-event chip color without needing an organizer-specified value. */
export const eventTone = (organizerId: string): Tone => {
  const tones: Tone[] = ["ocean", "grass", "sky", "heather", "moss"];
  let h = 0;
  for (let i = 0; i < organizerId.length; i++) h = (h * 31 + organizerId.charCodeAt(i)) | 0;
  return tones[Math.abs(h) % tones.length];
};

/** YYYY-MM-DD slice of an ISO datetime, in the viewer's local TZ. The
 *  calendar grid is local-time; UTC slicing would put events in the
 *  wrong day at the edges. */
export const ymdLocal = (iso: string): string => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso.slice(0, 10);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
};

/** Compact wall-clock label for an event ("09:00", "—" for all-day). */
export const timeLabel = (e: ApiEvent): string => {
  if (e.allDay) return "—";
  if (!e.startsAt) return "—";
  const d = new Date(e.startsAt);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", hour12: false });
};

// ---- Generic ----------------------------------------------------------

/** "Good morning/afternoon/evening" depending on local hour. */
export const partOfDayGreeting = (now = new Date()): string => {
  const h = now.getHours();
  if (h < 12) return "Good morning";
  if (h < 17) return "Good afternoon";
  return "Good evening";
};

/** "MONDAY · 25 MAY · DASHBOARD"-style banner the dashboard hero uses. */
export const dashboardKicker = (now = new Date()): string => {
  const day = now.toLocaleDateString(undefined, { weekday: "long" }).toUpperCase();
  const date = `${now.getDate()} ${now.toLocaleDateString(undefined, { month: "short" }).toUpperCase()}`;
  return `${day} · ${date} · DASHBOARD`;
};

/** Membersake initials/swatch into the shape the existing Avatar component
 *  expects. Keeps the comp's typing centralised so a future Avatar rewrite
 *  has one upstream place to edit. */
export interface AvatarBits {
  initials: string;
  swatch: Swatch;
  name?: string;
}

export const toAvatar = (m: Member): AvatarBits => ({
  initials: initial(m),
  swatch: memberSwatch(m.memberId),
  name: displayName(m),
});
