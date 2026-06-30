import { For, Show, createEffect, createResource, createSignal, onCleanup, onMount } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Bell } from "./Icons";
import { notificationClient } from "~/data/clients";
import { useCurrentHouseId } from "~/stores/auth";
import { lastSeenLabel } from "~/lib/derive";
import type { Notification as Notif } from "@longhouse/client";

/**
 * Header notification bell: an unread badge that polls in the background, and
 * a dropdown feed of the caller's notifications. Clicking a notification marks
 * it read (and deep-links to the target when one still exists); "Mark all
 * read" clears the badge. The feed is each member's own — the API scopes every
 * call to the caller, so there's nothing house-admin-special here.
 *
 * Notifications are self-contained snapshots, so we render actor/target/body
 * straight from the row; if the underlying task/project was deleted the text
 * still reads fine and the deep-link is simply skipped.
 */

const POLL_MS = 45_000;

export const NotificationBell = () => {
  const houseId = useCurrentHouseId();
  const navigate = useNavigate();
  const [open, setOpen] = createSignal(false);
  const [unread, setUnread] = createSignal(0);
  let wrap: HTMLDivElement | undefined;

  const [feed, { refetch, mutate }] = createResource(
    () => (open() ? houseId() : null),
    async (h) => (h ? await notificationClient.listNotifications({ houseId: h, limit: 30 }) : []),
  );

  const refreshUnread = async () => {
    const h = houseId();
    if (!h) {
      setUnread(0);
      return;
    }
    try {
      const r = await notificationClient.unreadCount(h);
      setUnread(Number(r.count ?? 0));
    } catch {
      /* transient — keep the last good count */
    }
  };

  const onDoc = (e: MouseEvent) => {
    if (open() && wrap && !wrap.contains(e.target as Node)) setOpen(false);
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") setOpen(false);
  };

  let timer: ReturnType<typeof setInterval> | undefined;
  onMount(() => {
    timer = setInterval(() => void refreshUnread(), POLL_MS);
    document.addEventListener("click", onDoc);
    document.addEventListener("keydown", onKey);
  });
  onCleanup(() => {
    if (timer) clearInterval(timer);
    document.removeEventListener("click", onDoc);
    document.removeEventListener("keydown", onKey);
  });

  // Initial fetch + refresh whenever the selected house changes.
  createEffect(() => {
    houseId();
    void refreshUnread();
  });

  const toggle = () => {
    const next = !open();
    setOpen(next);
    if (next) void refetch();
  };

  const markRead = async (n: Notif) => {
    if (n.read) return;
    mutate((prev) => (prev ?? []).map((x) => (x.notificationId === n.notificationId ? { ...x, read: true } : x)));
    setUnread((c) => Math.max(0, c - 1));
    try {
      await notificationClient.markRead(n.notificationId);
    } catch {
      void refetch();
    }
    void refreshUnread();
  };

  const activate = (n: Notif) => {
    void markRead(n);
    if (n.targetType === "project" && n.targetId) {
      setOpen(false);
      navigate(`/projects/${n.targetId}`);
    } else if (n.targetType === "task") {
      setOpen(false);
      navigate("/tasks");
    }
  };

  const markAll = async () => {
    const h = houseId();
    if (!h) return;
    mutate((prev) => (prev ?? []).map((x) => ({ ...x, read: true })));
    setUnread(0);
    try {
      await notificationClient.markAllRead(h);
    } catch {
      void refetch();
    }
  };

  const badge = () => (unread() > 9 ? "9+" : String(unread()));

  return (
    <div ref={wrap} style="position:relative">
      <button
        class="icon-btn"
        onClick={toggle}
        aria-label="Notifications"
        aria-haspopup="menu"
        aria-expanded={open() ? "true" : "false"}
        style="position:relative"
      >
        <Bell />
        <Show when={unread() > 0}>
          <span
            aria-label={`${unread()} unread`}
            style="position:absolute;top:-3px;right:-3px;min-width:16px;height:16px;padding:0 4px;display:flex;align-items:center;justify-content:center;background:var(--rust);color:#fff;border-radius:999px;font-size:10px;font-weight:700;line-height:1"
          >
            {badge()}
          </span>
        </Show>
      </button>

      <Show when={open()}>
        <div
          role="menu"
          aria-label="Notifications"
          style="position:absolute;top:calc(100% + 6px);right:0;width:340px;max-height:60vh;overflow:auto;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);z-index:60;display:flex;flex-direction:column"
        >
          <div style="display:flex;align-items:center;justify-content:space-between;padding:10px 14px;border-bottom:1px solid var(--line);position:sticky;top:0;background:var(--paper)">
            <span style="font-size:13px;font-weight:600;color:var(--ink)">Notifications</span>
            <Show when={(feed() ?? []).some((n) => !n.read)}>
              <button
                type="button"
                onClick={markAll}
                style="background:transparent;border:0;color:var(--ocean-2);font-size:12px;cursor:pointer;padding:0"
              >
                Mark all read
              </button>
            </Show>
          </div>

          <Show
            when={!feed.loading}
            fallback={<p style="padding:18px 14px;font-size:13px;color:var(--ink-faint)">Loading…</p>}
          >
            <Show
              when={(feed() ?? []).length > 0}
              fallback={<p style="padding:18px 14px;font-size:13px;color:var(--ink-faint)">You're all caught up.</p>}
            >
              <For each={feed()}>
                {(n) => (
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => activate(n)}
                    style={`display:flex;gap:10px;align-items:flex-start;padding:10px 14px;border:0;border-bottom:1px solid var(--line);text-align:left;cursor:pointer;background:${n.read ? "transparent" : "color-mix(in oklab, var(--grass-1) 22%, transparent)"}`}
                  >
                    <span
                      aria-hidden="true"
                      style={`flex:none;width:7px;height:7px;margin-top:6px;border-radius:999px;background:${n.read ? "transparent" : "var(--grass-4)"}`}
                    />
                    <span style="flex:1;min-width:0">
                      <span style="font-size:13px;color:var(--ink);line-height:1.35">
                        <strong>{n.actorName || "Someone"}</strong>
                        {" commented on "}
                        {n.targetType ?? "a thread"}
                        <Show when={n.targetTitle}> “{n.targetTitle}”</Show>
                      </span>
                      <Show when={n.body}>
                        <span style="display:block;font-size:12.5px;color:var(--ink-mute);margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
                          {n.body}
                        </span>
                      </Show>
                      <span style="display:block;font-size:11px;color:var(--ink-faint);margin-top:3px">
                        {lastSeenLabel({ lastSeenAt: n.createdAt }) ?? ""}
                      </span>
                    </span>
                  </button>
                )}
              </For>
            </Show>
          </Show>
        </div>
      </Show>
    </div>
  );
};
